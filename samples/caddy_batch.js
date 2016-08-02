$http.batch([
	// 200
	"http://localhost:2015",
	"http://localhost:2015/style.css",
	"http://localhost:2015/teddy.jpg",
	
	// 404
	"http://localhost:2015/script.js",
]);
