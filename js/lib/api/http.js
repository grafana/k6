"use strict";

$http = $http || {};

$http.get = function() {
	return $http.request.apply(this, _.concat(['GET'], arguments));
}

$http.post = function() {
	return $http.request.apply(this, _.concat(['POST'], arguments));
}

$http.put = function() {
	return $http.request.apply(this, _.concat(['PUT'], arguments));
}

$http.delete = function() {
	return $http.request.apply(this, _.concat(['DELETE'], arguments));
}

$http.patch = function() {
	return $http.request.apply(this, _.concat(['PATCH'], arguments));
}

$http.options = function() {
	return $http.request.apply(this, _.concat(['OPTIONS'], arguments));
}
