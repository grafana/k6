/**
 * @module k6/http
 */
import { parseHTML } from "k6/html";

export class Response {
	/**
	 * Represents an HTTP response.
	 * @memberOf module:k6/http
	 */
	constructor(data) {
		Object.assign(this, data);
	}

	json() {
		if (!this._json) {
			this._json = JSON.parse(this.body);
		}
		return this._json;
	}

	html(sel) {
		if (!this._html) {
			this._html = parseHTML(this.body);
		}
		if (sel) {
			return this._html.find(sel);
		}
		return this._html;
	}
}

function parseBody(body) {
	if (body) {
		if (typeof body === "object") {
			let formstring = "";
			for (let key in body) {
				if (formstring !== "") {
					formstring += "&";
				}
				formstring += key + "=" + encodeURIComponent(body[key]);
			}
			return formstring;
		}
		return body;
	} else {
		return '';
	}
}

/**
 * Makes an HTTP request.
 * @param  {string} method      HTTP Method (eg. "GET")
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function request(method, url, body, params = {}) {
	method = method.toUpperCase();
	body = parseBody(body);
	return new Response(__jsapi__.HTTPRequest(method, url, body, JSON.stringify(params)));
};

/**
 * Makes a GET request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function get(url, params) {
	return request("GET", url, null, params);
};

/**
 * Makes a POST request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function post(url, body, params) {
	return request("POST", url, body, params);
};

/**
 * Makes a PUT request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function put(url, body, params) {
	return request("PUT", url, body, params);
};

/**
 * Makes a DELETE request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function del(url, body, params) {
	return request("DELETE", url, body, params);
};

/**
 * Makes a PATCH request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function patch(url, body, params) {
	return request("PATCH", url, body, params);
};

/**
 * Makes a CONNECT request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function connect(url, body, params) {
	return request("CONNECT", url, body, params);
};

/**
 * Makes a OPTIONS request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function options(url, body, params) {
	return request("OPTIONS", url, body, params);
};

/**
 * Makes a TRACE request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function trace(url, body, params) {
	return request("TRACE", url, body, params);
};

/**
 * Batches multiple requests together.
 * @see    module:k6/http.request
 * @param  {Array|Object} requests	An array or object of requests, in string or object form.
 * @return {Array.<module:k6/http.Response>|Object}
 */
export function batch(requests) {
	function stringToObject(str) {
		return {
			"method": "GET",
			"url": str,
			"body": null,
			"params": JSON.stringify({})
		}
	}

	function formatObject(obj) {
		obj.params = !obj.params ? {} :obj.params
		obj.body = parseBody(obj.body)
		obj.params = JSON.stringify(obj.params)
		return obj
	}

	let result
	if (requests.length > 0) {
		result = requests.map(e => {
			if (typeof e === 'string') {
				return stringToObject(e)
			} else {
				return formatObject(e)
			}
		})
	} else {
		result = {}
		Object.keys(requests).map(e => {
			let val = requests[e]
			if (typeof val === 'string') {
				result[e] = stringToObject(val)
			} else {
				result[e] = formatObject(val)
			}
		})
	}
	
	let response = __jsapi__.BatchHTTPRequest(result);
	return response
};

export default {
	Response,
	request,
	get,
	post,
	put,
	del,
	patch,
	batch,
};
