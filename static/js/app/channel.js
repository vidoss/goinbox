(function($) {

	$(function() {
		var token = $("#token").text(),
			channel = new goog.appengine.Channel(token),
			sock = channel.open()

		sock.onmessage = function(msg) {
			console.log(msg);
		};
	});

})(jQuery)
