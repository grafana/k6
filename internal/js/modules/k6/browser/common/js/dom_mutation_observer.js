// dom_mutation_observer.js installs a MutationObserver on the document and
// signals the k6 browser module whenever the DOM changes. The signalling
// goes through the k6browserDomMutation CDP binding registered on the page
// by the auto-screenshot lifecycle watcher.
//
// requestAnimationFrame is used as a JS-side throttle: bursty mutations
// coalesce into at most one binding call per repaint frame. A second,
// coarser debounce window is applied on the Go side.
//
// The script self-disables if the binding is not present (mode B was not
// enabled for this page) so accidental evaluation outside auto-screenshot
// contexts is harmless.
(function () {
  if (typeof window.k6browserDomMutation !== 'function') {
    return;
  }
  if (window.__k6BrowserDomMutationObserverInstalled) {
    return;
  }
  window.__k6BrowserDomMutationObserverInstalled = true;

  var scheduled = false;
  var notify = function () {
    scheduled = false;
    try {
      window.k6browserDomMutation('');
    } catch (e) {
      // binding can disappear if the page navigates while the observer is
      // alive; swallow the error so the page is not disturbed.
    }
  };

  var observer = new MutationObserver(function () {
    if (scheduled) {
      return;
    }
    scheduled = true;
    if (typeof window.requestAnimationFrame === 'function') {
      window.requestAnimationFrame(notify);
    } else {
      setTimeout(notify, 16);
    }
  });

  var start = function () {
    if (!document.documentElement) {
      return false;
    }
    observer.observe(document.documentElement, {
      childList: true,
      subtree: true,
      attributes: true,
      characterData: true,
    });
    return true;
  };

  if (!start()) {
    document.addEventListener(
      'DOMContentLoaded',
      function () {
        start();
      },
      { once: true },
    );
  }
})();
