/**
 * Finds all elements in a given scope.
 * @param {Node} scope - The scope of searching. It can be a node.
 *                       By default, it is document.
 * @param {InjectedScript} injected - Injected script.
 * @param {string} selector - XPath or CSS selector string.
 * @returns {Set<Node>|string} - A set of nodes found or an error string.
 */
function QueryAll(scope = document, injected, selector) {
  return injected.querySelectorAll(selector, scope);
}
