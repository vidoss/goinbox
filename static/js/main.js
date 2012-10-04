require.config({
    shim: {
        '/static/js/bootstrap/bootstrap.min.js': ['jquery'],
        '/static/js/backbone/backbone-0.9.2-min.js': [
            '/static/js/underscore/underscore-1.3.3-min.js'
         ],
         '/static/js/app/app.js': [
				'/static/js/backbone/backbone-0.9.2-min.js',
				'/static/js/bootstrap/bootstrap.min.js'
         ]
    }
});

require(["jquery",
         "/static/js/jquery/json2-min.js",
         "/static/js/underscore/underscore-1.3.3-min.js",
         "/static/js/backbone/backbone-0.9.2-min.js",
         "/static/js/bootstrap/bootstrap.min.js",
         "/static/js/app/app.js"
      ], function($) {

});
