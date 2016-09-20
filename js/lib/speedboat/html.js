export class Node {
	constructor(impl) {
		this.impl = impl;
	}
}

export class Selection {
	constructor(impl) {
		this.impl = impl;
	}

	find(sel) { return new Selection(this.impl.Find(sel)); }
	text() { return this.impl.Text(); }
};

export function parseHTML(src) {
	return new Selection(__jsapi__.HTMLParse(src));
};
