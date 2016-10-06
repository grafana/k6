/**
 * @module speedboat/http
 */
import { parseHTML } from "speedboat/html";

export class Response {
	/**
	 * Represents an HTTP response.
	 * @memberOf module:speedboat/http
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

/**
 * Makes an HTTP request.
 * @param  {string} method      HTTP Method (eg. "GET")
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body (query for GET/HEAD); objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:speedboat/http.Response}
 */
export function request(method, url, body, params = {}) {
	method = method.toUpperCase();
	if (body) {
		if (typeof body === "object") {
			let formstring = "";
			for (let entry of body) {
				if (formstring !== "") {
					formstring += "&";
				}
				formstring += entry[0] + "=" + encodeURIComponent(entry[1]);
			}
			body = formstring;
		}
		if (method === "GET" || method === "HEAD") {
			url += (url.includes("?") ? "&" : "?") + body;
			body = "";
		}
	}
	return new Response(__jsapi__.HTTPRequest(method, url, body, params));
};

/**
 * Makes a GET request.
 * @see    module:speedboat/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body (query for GET/HEAD); objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:speedboat/http.Response}
 */
export function get(url, body, params) {
	return request("GET", url, body, params);
};

/**
 * Makes a POST request.
 * @see    module:speedboat/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body (query for GET/HEAD); objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:speedboat/http.Response}
 */
export function post(url, body, params) {
	return request("POST", url, body, params);
};

/**
 * Makes a PUT request.
 * @see    module:speedboat/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body (query for GET/HEAD); objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:speedboat/http.Response}
 */
export function put(url, body, params) {
	return request("PUT", url, body, params);
};

/**
 * Makes a DELETE request.
 * @see    module:speedboat/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body (query for GET/HEAD); objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:speedboat/http.Response}
 */
export function del(url, body, params) {
	return request("DELETE", url, body, params);
};

/**
 * Makes a PATCH request.
 * @see    module:speedboat/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body (query for GET/HEAD); objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:speedboat/http.Response}
 */
export function patch(url, body, params) {
	return request("PATCH", url, body, params);
};

/**
 * Sets the maximum number of redirects to follow. A request that encounters more than this many
 * redirects will error. Default: 10.
 * @param {Number} n Max number of redirects.
 */
export function setMaxRedirects(n) {
	__jsapi__.HTTPSetMaxRedirects(n);
}

export default {
	request: request,
	get: get,
	post: post,
	put: put,
	del: del,
	patch: patch,
	setMaxRedirects: setMaxRedirects,
};
