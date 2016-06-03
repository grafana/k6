"use strict";

__modules__.http.get = function() {
	return __modules__.http.do.apply(this, _.concat(['GET'], arguments));
}

__modules__.http.post = function() {
	return __modules__.http.do.apply(this, _.concat(['POST'], arguments));
}

__modules__.http.put = function() {
	return __modules__.http.do.apply(this, _.concat(['PUT'], arguments));
}

__modules__.http.delete = function() {
	return __modules__.http.do.apply(this, _.concat(['DELETE'], arguments));
}

__modules__.http.patch = function() {
	return __modules__.http.do.apply(this, _.concat(['PATCH'], arguments));
}

__modules__.http.options = function() {
	return __modules__.http.do.apply(this, _.concat(['OPTIONS'], arguments));
}
