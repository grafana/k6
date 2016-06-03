"use strict";

__internal__.modules.http.get = function() {
	return __internal__.modules.http.do.apply(this, _.concat(['GET'], arguments));
}

__internal__.modules.http.post = function() {
	return __internal__.modules.http.do.apply(this, _.concat(['POST'], arguments));
}

__internal__.modules.http.put = function() {
	return __internal__.modules.http.do.apply(this, _.concat(['PUT'], arguments));
}

__internal__.modules.http.delete = function() {
	return __internal__.modules.http.do.apply(this, _.concat(['DELETE'], arguments));
}

__internal__.modules.http.patch = function() {
	return __internal__.modules.http.do.apply(this, _.concat(['PATCH'], arguments));
}

__internal__.modules.http.options = function() {
	return __internal__.modules.http.do.apply(this, _.concat(['OPTIONS'], arguments));
}
