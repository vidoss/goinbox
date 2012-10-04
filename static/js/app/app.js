(function($, ns) {

	var InboxView = Backbone.View.extend({

		el: ".inbox",

		events: {
			"click .inbox .row": "openEmail"
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
		},

		showBody: function(bodyEl) {
			bodyEl.removeClass("hidden");
			this.$el.addClass("hidden");
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

		var token = $("#token").text(),
			channel = new goog.appengine.Channel(token),
			sock = channel.open()

		sock.onmessage = function(msg) {
			console.log(msg);
		};
	});

})(jQuery, {})
