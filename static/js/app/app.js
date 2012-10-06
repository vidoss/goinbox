(function($, ns) {

	var ActionBtnView = Backbone.View.extend({
		
		el: ".action-row",

		events: {
			"click .back-button" : "onBackBtnClick"
		},

		initialize: function() {
			this.selectMail = this.$(".select-mail");
			this.backBtn = this.$(".back-button");
			this.actionBtns = this.$(".action-buttons");
			this.unreadBtn = this.$(".unread-button");
		},

		inboxMode: function() {
			this.selectMail.removeClass("hidden");
			this.backBtn.addClass("hidden");
			this.actionBtns.addClass("hidden");
			this.unreadBtn.addClass("hidden");
		},

		mailBodyMode: function() {
			this.selectMail.addClass("hidden");
			this.backBtn.removeClass("hidden");
			this.actionBtns.removeClass("hidden");
			this.unreadBtn.removeClass("hidden");
		},

		onBackBtnClick: function() {
			ns.app_router.navigate("/",{trigger:true});
		}

	});

	var InboxView = Backbone.View.extend({

		el: ".inbox",

		events: {
			"click .inbox .row": "openEmail"
		},

		initialize: function() {
			this.actionBtns = new ActionBtnView();
		},

		openEmail: function(e) {
			var key = $(e.currentTarget).data("key");
			if (key) {
				ns.app_router.navigate(key,{trigger: true});
			}
		},

		showInbox: function() {
			this.$el.siblings(".email-body").addClass('hidden');
			this.$el.removeClass('hidden');
			this.actionBtns.inboxMode();
		},

		showBody: function(bodyEl) {
			bodyEl.removeClass("hidden");
			this.$el.addClass("hidden");
			this.actionBtns.mailBodyMode();
		}

	});

	function initRouter() {
		var AppRouter = Backbone.Router.extend({
			routes: {
				"email/:id": "openEmail",
				"*actions": "defaultRoute"
			},
			defaultRoute: function (actions) {
				if (!this.inboxView) {
					this.inboxView = new InboxView();
				}
				this.inboxView.showInbox();
			},
			openEmail: function(actions) {
				if (!this.inboxView) {
					this.inboxView = new InboxView();
				}
				this.inboxView.showBody($("#email-"+actions));
			}
		});

		ns.app_router = new AppRouter();
		Backbone.history.start({pushState: true});
	}

	$(function() {

		initRouter();
/*
		var token = $("#token").text(),
			channel = new goog.appengine.Channel(token),
			sock = channel.open()

		sock.onmessage = function(msg) {
			console.log(msg);
		};
*/
	});

})(jQuery, {})
