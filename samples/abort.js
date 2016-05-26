var vu = require('vu'),
	test = require('test'),
	log = require('log');

var i = vu.iteration();
log.info("Iteration", {i: i});
if (i == 10) {
	test.abort();
}
