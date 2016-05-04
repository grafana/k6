var vu = require('vu'),
	test = require('test');

var i = vu.iteration();
console.log("Iteration: " + i)
if (i == 10) {
	test.abort();
}
