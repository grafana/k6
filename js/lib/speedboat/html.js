export class Node {
	constructor(impl) {
		this.impl = impl;
	}
}

export class Selection {
	constructor(impl) {
		this.impl = impl;
	}

	add(arg) {
		if (typeof arg === "string") {
			return new Selection(this.impl.Add(arg));
		} else if (arg instanceof Selection) {
			return new Selection(__jsapi__.HTMLSelectionAddSelection(this.impl, arg.impl));
		}
		throw new TypeError("add() argument must be a string or Selection")
	}

	find(sel) { return new Selection(this.impl.Find(sel)); }
	text() { return this.impl.Text(); }
};

export function parseHTML(src) {
	return new Selection(__jsapi__.HTMLParse(src));
};
