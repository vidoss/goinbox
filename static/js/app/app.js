(function($, ns) {

	var EmailCollection = Backbone.Collection.extend({
		
		url: "/email",

		sync: function(method, model, options) {
			if (!_.isFunction(options.dom2Model)) {
				return Backbone.sync.apply(this,arguments);
			}
			options.success(options.dom2Model());
		}

	});

	var ActionBtnView = Backbone.View.extend({
		
		el: ".action-row",

		events: {
			"click .back-button" : "onBackBtnClick",
			"click .unread-button" : "onUnreadBtnClick",
			"click .trash-button" : "onDeleteBtnClick",
		},

		initialize: function() {
			this.selectMail = this.$(".select-mail");
			this.actionBtns = this.$(".action-buttons");
			this.moveToBtn = this.$(".moveto-button");
			this.backBtn = this.$(".back-button");
		},

		inboxMode: function() {
			this.selectMail.removeClass("hidden");
			this.actionBtns.addClass("hidden");
			this.moveToBtn.addClass("hidden");
			this.backBtn.addClass("hidden");
		},

		mailBodyMode: function() {
			this.selectMail.addClass("hidden");
			this.actionBtns.removeClass("hidden");
			this.moveToBtn.removeClass("hidden");
			this.backBtn.removeClass("hidden");
		},

		onBackBtnClick: function() {
			ns.app_router.navigate("/",{trigger:true});
		},

		onUnreadBtnClick: function() {
			_.each(this.collection.where({selected: true}),function(m){m.save()});
			ns.app_router.navigate("/",{trigger:true});
		},

		onDeleteBtnClick: function() {
			_.each(this.collection.where({selected: true}),function(m){m.destroy()});
			ns.app_router.navigate("/",{trigger:true});
		},

		setCollection: function(coll) {
			this.collection = coll;
		}

	});

	var MessagesView = Backbone.View.extend({

		el: ".email-row",

		events: {
			"click .email-list .row": "openEmail"
		},

		initialize: function(options) {
			this.actionBtns = options.actionBtns;
			this.emailList = this.$(".email-list");

			this.collection = new EmailCollection();

			this.collection.bind('add', this.addOne, this);
			this.collection.bind('reset', this.addAll, this);

			this.collection.fetch({dom2Model: _.bind(this.dom2Model,this)});
		},

		addOne: function() {
		},

		addAll: function() {
			this.collection.each(_.bind(this.addOne,this));
		},

		openEmail: function(e) {
			var key = $(e.currentTarget).data("key");
			if (key) {
				ns.app_router.navigate(key,{trigger: true});
			}
		},

		showMessages: function() {
			this.actionBtns.setCollection(this.collection);
			this.emailList.siblings(".email-body").addClass('hidden');
			this.emailList.removeClass('hidden');
			this.actionBtns.inboxMode();
		},

		showBody: function(emailId) {
			var mdl = this.collection.get(emailId);
			if (!mdl) {
				// console.error("Eikes");
				return false;
			}
			_.each(this.collection.where({selected: true}),function(m){m.set({selected: false})});
			mdl.set({selected: true});

			var bodyEl = $("#email-"+emailId),
				rendered = bodyEl.data("rendered");

			bodyEl.removeClass('hidden');
			if (!rendered) {

				var textareaEl = bodyEl.children('textarea'),
					ifr = bodyEl.children('iframe'),
					ifrDoc = ifr.contents();

				ifr.load(function() {
					var _self = this;
					_.defer(function() {
						$(_self).height(ifrDoc[0].body.offsetHeight);
						ifr.unbind('load');
					});
				});

				var ifrEl = ifrDoc[0];
				ifrEl.open('text/html','replace');
				ifrEl.write("<h4>"+mdl.get("subject")+"</h4>");
				ifrEl.write(mdl.get("body"));
				ifrEl.close();

				bodyEl.data("rendered",true);
				textareaEl.remove();
			}
			this.emailList.addClass("hidden");
			this.actionBtns.mailBodyMode();

			return true;
		},

		dom2Model: function() {

			return  _.toArray(this.emailList.children(".email-item").map(function() {
									var thisEl = $(this),
										id = thisEl.data("key").split("/")[1];

									return {
										id: id,
										selected: thisEl.find('.email-selected input[type="checkbox"]').is(":checked"),
										from: thisEl.children(".email-from").text(),
										subject: thisEl.children(".email-subject").text(),
										body: $("#email-"+id).children("textarea").val(),
										el: this
									}
								}));
		}

	});

	function initRouter() {
		var folders = {}, 
			currentFolder = "inbox",
			actionBtnView = new ActionBtnView(),
			AppRouter = Backbone.Router.extend({
				routes: {
					"email/:id" : "openEmail",
					"inbox"	   : "openInbox",
					"trash"	   : "openTrash",
					"folder/:id": "openFolder",
					"*actions": "defaultRoute"
				},
				defaultRoute: function() {
					return this.openInbox();
				},
				openInbox: function () {
					this.openFolder("inbox");
				},
				openTrash: function() {
					this.openFolder("trash");
				},
				openFolder: function (folderId) {
					if (!folders[folderId]) {
						folders[folderId] = new MessagesView({
															actionBtns: actionBtnView
														});
					}
					currentFolder = folderId;
			  	 	folders[folderId].showMessages();
				},
				openEmail: function(emailid) {
					var fldr = folders[currentFolder];
					if (!fldr) {
						fldr = new MessagesView({actionBtns: actionBtnView});
					}
					fldr.showBody(emailid);
				}
			});

		ns.app_router = new AppRouter();
		Backbone.history.start({pushState: true});
	}

	$(function() {

		initRouter();

		var token = $("#token").text(),
			channel = new goog.appengine.Channel(token),
			sock = channel.open()

		sock.onmessage = function(msg) {
			console.log(msg);
		};

	});

})(jQuery, {})
