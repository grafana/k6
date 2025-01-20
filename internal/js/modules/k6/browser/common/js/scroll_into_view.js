/**
 * Scrolls an element into view.
 * @param {Node} node - The element to scroll into.
 * @param {ScrollIntoViewOptions} options.
 * @returns {void}
 */
function scrollIntoView(node, options) {
  // we can only scroll to element nodes
  if (node.nodeType !== Node.ELEMENT_NODE) {
    return;
  }
  node.scrollIntoView(options);
}