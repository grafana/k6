/**
 * @module speedboat/html
 */

/**
 * Parses an HTML string into a Selection.
 *
 * @param  {string}    src HTML source.
 * @return {Selection}
 * @throws {Error}         If src is not valid HTML.
 */
export function parseHTML(src) {
	return new Selection(__jsapi__.HTMLParse(src));
};

export class Selection {
	/**
	 * Represents a set of nodes in a DOM tree.
	 *
	 * Selections have a jQuery-compatible API, but with two caveats:
	 *
	 * - CSS and screen layout are not processed, thus calls like css() and offset() are unavailable.
	 * - DOM trees are read-only, you can't set attributes or otherwise modify nodes.
	 *
	 * (Note that the read-only nature of the DOM trees is purely to avoid a maintenance burden on code
	 * with seemingly no practical use - if a compelling use case is presented, modification can
	 * easily be implemented.)
	 *
	 * @memberOf module:speedboat/html
	 */
	constructor(impl) {
		this.impl = impl;
	}

	/**
	 * Extends the selection with another set of elements.
	 *
	 * @param {string|Selection} arg Selection or selector
	 * @return {module:speedboat/html.Selection}
	 */
	add(arg) {
		if (typeof arg === "string") {
			return new Selection(this.impl.Add(arg));
		} else if (arg instanceof Selection) {
			return new Selection(__jsapi__.HTMLSelectionAddSelection(this.impl, arg.impl));
		}
		throw new TypeError("add() argument must be a string or Selection")
	}

	/**
	 * Finds children by a selector.
	 *
	 * @param  {string}    sel CSS selector.
	 * @return {module:speedboat/html.Selection}
	 */
	find(sel) {
		return new Selection(this.impl.Find(sel));
	}

	/**
	 * Returns the combined text content of all selected nodes.
	 * @return {string}
	 */
	text() { return this.impl.Text(); }
};
