import { parseHTML } from "speedboat/html";

export class Response {
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

export function get(url, body, params) {
	return request("GET", url, body, params);
};

export function post(url, body, params) {
	return request("POST", url, body, params);
};

export function put(url, body, params) {
	return request("PUT", url, body, params);
};

export function del(url, body, params) {
	return request("DELETE", url, body, params);
};

export function patch(url, body, params) {
	return request("PATCH", url, body, params);
};

export default {
	request: request,
	get: get,
	post: post,
	put: put,
	del: del,
	patch: patch,
};
