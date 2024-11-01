/**
 * Copyright (c) Microsoft Corporation.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

const autoClosingTags = new Set([
  "AREA",
  "BASE",
  "BR",
  "COL",
  "COMMAND",
  "EMBED",
  "HR",
  "IMG",
  "INPUT",
  "KEYGEN",
  "LINK",
  "MENUITEM",
  "META",
  "PARAM",
  "SOURCE",
  "TRACK",
  "WBR",
]);
const booleanAttributes = new Set([
  "checked",
  "selected",
  "disabled",
  "readonly",
  "multiple",
]);
const eventType = new Map([
  ["auxclick", "mouse"],
  ["click", "mouse"],
  ["dblclick", "mouse"],
  ["mousedown", "mouse"],
  ["mouseeenter", "mouse"],
  ["mouseleave", "mouse"],
  ["mousemove", "mouse"],
  ["mouseout", "mouse"],
  ["mouseover", "mouse"],
  ["mouseup", "mouse"],
  ["mouseleave", "mouse"],
  ["mousewheel", "mouse"],

  ["keydown", "keyboard"],
  ["keyup", "keyboard"],
  ["keypress", "keyboard"],
  ["textInput", "keyboard"],

  ["touchstart", "touch"],
  ["touchmove", "touch"],
  ["touchend", "touch"],
  ["touchcancel", "touch"],

  ["pointerover", "pointer"],
  ["pointerout", "pointer"],
  ["pointerenter", "pointer"],
  ["pointerleave", "pointer"],
  ["pointerdown", "pointer"],
  ["pointerup", "pointer"],
  ["pointermove", "pointer"],
  ["pointercancel", "pointer"],
  ["gotpointercapture", "pointer"],
  ["lostpointercapture", "pointer"],

  ["focus", "focus"],
  ["blur", "focus"],

  ["drag", "drag"],
  ["dragstart", "drag"],
  ["dragend", "drag"],
  ["dragover", "drag"],
  ["dragenter", "drag"],
  ["dragleave", "drag"],
  ["dragexit", "drag"],
  ["drop", "drag"],
]);

const continuePolling = Symbol("continuePolling");

function isVisible(element) {
  if (!element.ownerDocument || !element.ownerDocument.defaultView) {
    return true;
  }
  const style = element.ownerDocument.defaultView.getComputedStyle(element);
  if (!style || style.visibility === "hidden") {
    return false;
  }
  const rect = element.getBoundingClientRect();
  return rect.width > 0 && rect.height > 0;
}

function oneLine(s) {
  return s.replace(/\n/g, "↵").replace(/\t/g, "⇆");
}

class CSSQueryEngine {
  queryAll(root, selector) {
    return root.querySelectorAll(selector);
  }
}

class TextQueryEngine {
  queryAll(root, selector) {
    return root.queryAll(selector);
  }
}

class XPathQueryEngine {
  queryAll(root, selector) {
    if (selector.startsWith("/")) {
      selector = "." + selector;
    }
    const result = [];

    // DocumentFragments cannot be queried with XPath and they do not implement
    // evaluate. It first needs to be converted to a Document before being able
    // to run the evaluate against it.
    //
    // This avoids the following error:
    // - Failed to execute 'evaluate' on 'Document': The node provided is
    //   '#document-fragment', which is not a valid context node type.
    if (root instanceof DocumentFragment) {
      root = convertToDocument(root);
    }

    const document = root instanceof Document ? root : root.ownerDocument;
    if (!document) {
      return result;
    }
    const it = document.evaluate(
      selector,
      root,
      null,
      XPathResult.ORDERED_NODE_ITERATOR_TYPE
    );
    for (let node = it.iterateNext(); node; node = it.iterateNext()) {
      if (node.nodeType === 1 /*Node.ELEMENT_NODE*/) {
        result.push(node);
      }
    }
    return result;
  }
}

// convertToDocument will convert a DocumentFragment into a Document. It does
// this by creating a new Document and copying the elements from the
// DocumentFragment to the Document.
function convertToDocument(fragment) {
  var newDoc = document.implementation.createHTMLDocument("Temporary Document");

  copyNodesToDocument(fragment, newDoc.body);

  return newDoc;
}

// copyNodesToDocument manually copies nodes to a new document, excluding
// ShadowRoot nodes -- ShadowRoot are not cloneable so we need to manually
// clone them one element at a time.
function copyNodesToDocument(sourceNode, targetNode) {
  sourceNode.childNodes.forEach((child) => {
      if (child.nodeType === Node.ELEMENT_NODE) {
          // Clone the child node without its descendants
          let clonedChild = child.cloneNode(false);
          targetNode.appendChild(clonedChild);

          // If the child has a shadow root, recursively copy its children
          // instead of the shadow root itself.
          if (child.shadowRoot) {
              copyNodesToDocument(child.shadowRoot, clonedChild);
          } else {
              // Recursively copy normal child nodes
              copyNodesToDocument(child, clonedChild);
          }
      } else {
          // For non-element nodes (like text nodes), clone them directly.
          let clonedChild = child.cloneNode(true);
          targetNode.appendChild(clonedChild);
      }
  });
}

class InjectedScript {
  constructor() {
    this._replaceRafWithTimeout = false;
    this._stableRafCount = 10;
    this._queryEngines = {
      css: new CSSQueryEngine(),
      text: new TextQueryEngine(),
      xpath: new XPathQueryEngine(),
    };
  }

  _queryEngineAll(part, root) {
    return this._queryEngines[part.name].queryAll(root, part.body);
  }

  _querySelectorRecursively(roots, selector, index, queryCache) {
    if (index === selector.parts.length) {
      return roots;
    }

    const part = selector.parts[index];
    if (part.name === "nth") {
      let filtered = [];
      if (part.body === "0") {
        filtered = roots.slice(0, 1);
      } else if (part.body === "-1") {
        if (roots.length) {
          filtered = roots.slice(roots.length - 1);
        }
      } else {
        if (typeof selector.capture === "number") {
          return "error:nthnocapture";
        }
        const nth = part.body;
        const set = new Set();
        for (const root of roots) {
          set.add(root.element);
          if (nth + 1 === set.size) {
            filtered = [root];
          }
        }
      }
      return this._querySelectorRecursively(
        filtered,
        selector,
        index + 1,
        queryCache
      );
    }

    if (part.name === "visible") {
      const visible = Boolean(part.body);
      return roots.filter((match) => visible === isVisible(match.element));
    }

    const result = [];
    for (const root of roots) {
      const capture =
        index - 1 === selector.capture ? root.element : root.capture;

      // Do not query engine twice for the same element.
      let queryResults = queryCache.get(root.element);
      if (!queryResults) {
        queryResults = [];
        queryCache.set(root.element, queryResults);
      }
      let all = queryResults[index];
      if (!all) {
        all = this._queryEngineAll(selector.parts[index], root.element);
        queryResults[index] = all;
      }

      for (const element of all) {
        if (!("nodeName" in element)) {
          return `error:expectednode:${Object.prototype.toString.call(
            element
          )}`;
        }
        result.push({ element, capture });
      }

      // Explore the Shadow DOM recursively.
      const shadowResults = this._exploreShadowDOM(root.element, selector, index, queryCache, capture);
      result.push(...shadowResults);
    }

    return this._querySelectorRecursively(
      result,
      selector,
      index + 1,
      queryCache
    );
  }

  _exploreShadowDOM(root, selector, index, queryCache, capture) {
    let result = [];
    if (root.shadowRoot) {
      const shadowRootResults = this._querySelectorRecursively(
        [{ element: root.shadowRoot, capture }],
        selector,
        index,
        queryCache
      );
      result = result.concat(shadowRootResults);
    }

    if (!root.hasChildNodes()) return result;
    
    for (let i = 0; i < root.children.length; i++) {
      const childElement = root.children[i];
      result = result.concat(this._exploreShadowDOM(childElement, selector, index, queryCache, capture));
    }
    
    return result;
  }

  // Make sure we target an appropriate node in the DOM before performing an action.
  _retarget(node, behavior) {
    let element =
      node.nodeType === 1 /*Node.ELEMENT_NODE*/ ? node : node.parentElement;
    if (!element) {
      return null;
    }
    if (!element.matches("input, textarea, select")) {
      element =
        element.closest(
          "button, [role=button], [role=checkbox], [role=radio]"
        ) || element;
    }
    if (behavior === "follow-label") {
      if (
        !element.matches(
          "input, textarea, button, select, [role=button], [role=checkbox], [role=radio]"
        ) &&
        !element.isContentEditable
      ) {
        // Go up to the label that might be connected to the input/textarea.
        element = element.closest("label") || element;
      }
      if (element.nodeName === "LABEL") {
        element = element.control || element;
      }
    }
    return element;
  }

  checkElementState(node, state) {
    const element = this._retarget(
      node,
      ["stable", "visible", "hidden"].includes(state)
        ? "no-follow-label"
        : "follow-label"
    );
    if (!element || !element.isConnected) {
      if (state === "hidden") {
        return true;
      }
      return "error:notconnected";
    }

    if (state === "visible") {
      return this.isVisible(element);
    }
    if (state === "hidden") {
      return !this.isVisible(element);
    }

    const disabled =
      ["BUTTON", "INPUT", "SELECT", "TEXTAREA"].includes(element.nodeName) &&
      element.hasAttribute("disabled");
    if (state === "disabled") {
      return disabled;
    }
    if (state === "enabled") {
      return !disabled;
    }

    const editable = !(
      ["INPUT", "TEXTAREA", "SELECT"].includes(element.nodeName) &&
      element.hasAttribute("readonly")
    );
    if (state === "editable") {
      return !disabled && editable;
    }

    if (state === "checked") {
      if (element.getAttribute("role") === "checkbox") {
        return element.getAttribute("aria-checked") === "true";
      }
      if (element.nodeName !== "INPUT") {
        return "error:notcheckbox";
      }
      if (!["radio", "checkbox"].includes(element.type.toLowerCase())) {
        return "error:notcheckbox";
      }
      return element.checked;
    }
    return 'error:unexpected element state "' + state + '"';
  }

  checkHitTargetAt(node, point) {
    let element =
      node.nodeType === 1 /*Node.ELEMENT_NODE*/ ? node : node.parentElement;
    if (!element || !element.isConnected) {
      return "error:notconnected";
    }
    element = element.closest("button, [role=button]") || element;
    let hitElement = this.deepElementFromPoint(document, point.x, point.y);
    const hitParents = [];
    while (hitElement && hitElement !== element) {
      hitParents.push(hitElement);
      hitElement = this.parentElementOrShadowHost(hitElement);
    }
    if (hitElement === element) {
      return "done";
    }
    const hitTargetDescription = this.previewNode(hitParents[0]);
    // Root is the topmost element in the hitTarget's chain that is not in the
    // element's chain. For example, it might be a dialog element that overlays
    // the target.
    let rootHitTargetDescription;
    while (element) {
      const index = hitParents.indexOf(element);
      if (index !== -1) {
        if (index > 1) {
          rootHitTargetDescription = this.previewNode(hitParents[index - 1]);
        }
        break;
      }
      element = this.parentElementOrShadowHost(element);
    }
    if (rootHitTargetDescription)
      return {
        hitTargetDescription: `${hitTargetDescription} from ${rootHitTargetDescription} subtree`,
      };
    return { hitTargetDescription };
  }

  deepElementFromPoint(document, x, y) {
    let container = document;
    let element;
    while (container) {
      // elementFromPoint works incorrectly in Chromium (http://crbug.com/1188919),
      // so we use elementsFromPoint instead.
      const elements = container.elementsFromPoint(x, y);
      const innerElement = elements[0];
      if (!innerElement || element === innerElement) {
        break;
      }
      element = innerElement;
      container = element.shadowRoot;
    }
    return element;
  }

  dispatchEvent(node, type, eventInit) {
    let event;
    eventInit = {
      bubbles: true,
      cancelable: true,
      composed: true,
      ...eventInit,
    };
    switch (eventType.get(type)) {
      case "mouse":
        event = new MouseEvent(type, eventInit);
        break;
      case "keyboard":
        event = new KeyboardEvent(type, eventInit);
        break;
      case "touch":
        event = new TouchEvent(type, eventInit);
        break;
      case "pointer":
        event = new PointerEvent(type, eventInit);
        break;
      case "focus":
        event = new FocusEvent(type, eventInit);
        break;
      case "drag":
        event = new DragEvent(type, eventInit);
        break;
      default:
        event = new Event(type, eventInit);
        break;
    }
    node.dispatchEvent(event);
  }

  setInputFiles(node, payloads) {
    if (node.nodeType !== Node.ELEMENT_NODE)
      return "error:notelement";
    if (node.nodeName.toLowerCase() !== "input")
      return 'error:notinput';
    const type = (node.getAttribute('type') || '').toLowerCase();
    if (type !== 'file')
      return 'error:notfile';

    const dt = new DataTransfer();
    if (payloads) {
      const files = payloads.map(file => {
        const bytes = Uint8Array.from(atob(file.buffer), c => c.charCodeAt(0));
        return new File([bytes], file.name, { type: file.mimeType, lastModified: file.lastModifiedMs });
      });
      for (const file of files)
        dt.items.add(file);
    }
    node.files = dt.files;
    node.dispatchEvent(new Event('input', { 'bubbles': true }));
    node.dispatchEvent(new Event('change', { 'bubbles': true }));
    return "done";
  }

  getElementBorderWidth(node) {
    if (
      node.nodeType !== 1 /*Node.ELEMENT_NODE*/ ||
      !node.ownerDocument ||
      !node.ownerDocument.defaultView
    ) {
      return { left: 0, top: 0 };
    }
    const style = node.ownerDocument.defaultView.getComputedStyle(node);
    return {
      left: parseInt(style.borderLeftWidth || "", 10),
      top: parseInt(style.borderTopWidth || "", 10),
    };
  }

  fill(node, value) {
    const element = this._retarget(node, "follow-label");
    if (!element) {
      return "error:notconnected";
    }
    if (element.nodeName.toLowerCase() === "input") {
      const input = element;
      const type = input.type.toLowerCase();
      const kDateTypes = new Set([
        "date",
        "time",
        "datetime",
        "datetime-local",
        "month",
        "week",
      ]);
      const kTextInputTypes = new Set([
        "",
        "email",
        "number",
        "password",
        "search",
        "tel",
        "text",
        "url",
      ]);
      if (!kTextInputTypes.has(type) && !kDateTypes.has(type)) {
        return "error:notfillableinputtype";
      }
      value = value.trim();
      if (type === "number" && isNaN(Number(value))) {
        return "error:notfillablenumberinput";
      }
      input.focus();
      input.value = value;
      if (kDateTypes.has(type) && input.value !== value) {
        return "error:notvaliddate";
      }
      element.dispatchEvent(new Event("input", { bubbles: true }));
      element.dispatchEvent(new Event("change", { bubbles: true }));
      return "done"; // We have already changed the value, no need to input it.
    } else if (element.nodeName.toLowerCase() === "textarea") {
      // Nothing to check here.
    } else if (!element.isContentEditable) {
      return "error:notfillableelement";
    }
    this.selectText(element);
    return "needsinput"; // Still need to input the value.
  }

  focusNode(node, resetSelectionIfNotFocused) {
    if (!node.isConnected) {
      return "error:notconnected";
    }
    if (node.nodeType !== 1 /*Node.ELEMENT_NODE*/) {
      return "error:notelement";
    }
    const wasFocused =
      node.getRootNode().activeElement === node &&
      node.ownerDocument &&
      node.ownerDocument.hasFocus();
    node.focus();
    if (
      resetSelectionIfNotFocused &&
      !wasFocused &&
      node.nodeName.toLowerCase() === "input"
    ) {
      try {
        node.setSelectionRange(0, 0);
      } catch (e) {
        // Some inputs do not allow selection.
      }
    }
    return "done";
  }

  getDocumentElement(node) {
    const doc = node;
    if (doc.documentElement && doc.documentElement.ownerDocument === doc) {
      return doc.documentElement;
    }
    return node.ownerDocument ? node.ownerDocument.documentElement : null;
  }

  isVisible(element) {
    return isVisible(element);
  }

  parentElementOrShadowHost(element) {
    if (element.parentElement) {
      return element.parentElement;
    }
    if (!element.parentNode) {
      return;
    }
    if (
      element.parentNode.nodeType === 11 /*Node.DOCUMENT_FRAGMENT_NODE*/ &&
      element.parentNode.host
    ) {
      return element.parentNode.host;
    }
  }

  previewNode(node) {
    if (node.nodeType === 3 /*Node.TEXT_NODE*/) {
      return oneLine(`#text=${node.nodeValue || ""}`);
    }
    if (node.nodeType !== 1 /*Node.ELEMENT_NODE*/) {
      return oneLine(`<${node.nodeName.toLowerCase()} />`);
    }
    const element = node;

    const attrs = [];
    for (let i = 0; i < element.attributes.length; i++) {
      const { name, value } = element.attributes[i];
      if (name === "style") {
        continue;
      }
      if (!value && booleanAttributes.has(name)) {
        attrs.push(` ${name}`);
      } else {
        attrs.push(` ${name}="${value}"`);
      }
    }
    attrs.sort((a, b) => a.length - b.length);
    let attrText = attrs.join("");
    if (attrText.length > 50) {
      attrText = attrText.substring(0, 49) + "\u2026";
    }
    if (autoClosingTags.has(element.nodeName)) {
      return oneLine(`<${element.nodeName.toLowerCase()}${attrText}/>`);
    }

    const children = element.childNodes;
    let onlyText = false;
    if (children.length <= 5) {
      onlyText = true;
      for (let i = 0; i < children.length; i++) {
        onlyText = onlyText && children[i].nodeType === 3 /*Node.TEXT_NODE*/;
      }
    }
    let text = onlyText
      ? element.textContent || ""
      : children.length
      ? "\u2026"
      : "";
    if (text.length > 50) {
      text = text.substring(0, 49) + "\u2026";
    }
    return oneLine(
      `<${element.nodeName.toLowerCase()}${attrText}>${text}</${element.nodeName.toLowerCase()}>`
    );
  }

  querySelector(selector, strict, root) {
    if (!root["querySelector"]) {
      return "error:notqueryablenode";
    }
    const result = this._querySelectorRecursively(
      [{ element: root, capture: undefined }],
      selector,
      0,
      new Map()
    );
    if (strict && result.length > 1) {
      throw "error:strictmodeviolation";
    }
    if (result.length == 0) {
      return null;
    }
    return result[0].capture || result[0].element;
  }

  querySelectorAll(selector, root) {
    if (!root["querySelectorAll"]) {
      return "error:notqueryablenode";
    }
    const result = this._querySelectorRecursively(
      [{ element: root, capture: undefined }],
      selector,
      0,
      new Map()
    );
    const set = new Set();
    for (const r of result) {
      set.add(r.capture || r.element);
    }
    return [...set];
  }

  selectOptions(node, optionsToSelect) {
    const element = this._retarget(node, "follow-label");
    if (!element) {
      return "error:notconnected";
    }
    if (element.nodeName.toLowerCase() !== "select") {
      return "error:notselect";
    }
    const select = element;
    const options = Array.from(select.options);
    const selectedOptions = [];
    let remainingOptionsToSelect = optionsToSelect.slice();
    for (let index = 0; index < options.length; index++) {
      const option = options[index];
      const filter = (optionToSelect) => {
        if (optionToSelect instanceof Node) {
          return option === optionToSelect;
        }
        let matches = true;
        if (
          optionToSelect.value !== undefined &&
          optionToSelect.value !== null
        ) {
          matches = matches && optionToSelect.value === option.value;
        }
        if (
          optionToSelect.label !== undefined &&
          optionToSelect.label !== null
        ) {
          matches = matches && optionToSelect.label === option.label;
        }
        if (
          optionToSelect.index !== undefined &&
          optionToSelect.index !== null
        ) {
          matches = matches && optionToSelect.index === index;
        }
        return matches;
      };
      if (!remainingOptionsToSelect.some(filter)) {
        continue;
      }
      selectedOptions.push(option);
      if (select.multiple) {
        remainingOptionsToSelect = remainingOptionsToSelect.filter(
          (o) => !filter(o)
        );
      } else {
        remainingOptionsToSelect = [];
        break;
      }
    }
    /*if (remainingOptionsToSelect.length) {
            return continuePolling;
        }*/
    select.value = undefined;
    selectedOptions.forEach((option) => (option.selected = true));
    select.dispatchEvent(new Event("input", { bubbles: true }));
    select.dispatchEvent(new Event("change", { bubbles: true }));
    return selectedOptions.map((option) => option.value);
  }

  selectText(node) {
    const element = this._retarget(node, "follow-label");
    if (!element) {
      return "error:notconnected";
    }
    if (element.nodeName.toLowerCase() === "input") {
      const input = element;
      input.select();
      input.focus();
      return "done";
    }
    if (element.nodeName.toLowerCase() === "textarea") {
      const textarea = element;
      textarea.selectionStart = 0;
      textarea.selectionEnd = textarea.value.length;
      textarea.focus();
      return "done";
    }
    const range = element.ownerDocument.createRange();
    range.selectNodeContents(element);
    const selection = element.ownerDocument.defaultView.getSelection();
    if (selection) {
      selection.removeAllRanges();
      selection.addRange(range);
    }
    element.focus();
    return "done";
  }

  async waitForPredicateFunction(predicateFn, polling, timeout, ...args) {
    let timedOut = false;
    let timeoutPoll = null;
    const predicate = () => {
      return predicateFn(...args) || continuePolling;
    };
    if (timeout !== undefined || timeout !== null) {
      setTimeout(() => {
        timedOut = true;
        if (timeoutPoll) timeoutPoll();
      }, timeout);
    }
    if (polling === "raf") return await pollRaf();
    if (polling === "mutation") return await pollMutation();
    if (typeof polling === "number") return await pollInterval(polling);

    async function pollMutation() {
      const success = predicate();
      if (success !== continuePolling) return Promise.resolve(success);

      let resolve, reject;
      const result = new Promise((res, rej) => {
        resolve = res;
        reject = rej;
      });
      try {
        const observer = new MutationObserver(async () => {
          if (timedOut) {
            observer.disconnect();
            reject(`timed out after ${timeout}ms`);
          }
          const success = predicate();
          if (success !== continuePolling) {
            observer.disconnect();
            resolve(success);
          }
        });
        timeoutPoll = () => {
          observer.disconnect();
          reject(`timed out after ${timeout}ms`);
        };
        observer.observe(document, {
          childList: true,
          subtree: true,
          attributes: true,
        });
      } catch(error) {
        reject(error);
        return;
      }
      return result;
    }

    async function pollRaf() {
      let resolve, reject;
      const result = new Promise((res, rej) => {
        resolve = res;
        reject = rej;
      });
      await onRaf();
      return result;

      async function onRaf() {
        try {
          if (timedOut) {
            reject(`timed out after ${timeout}ms`);
            return;
          }
          const success = predicate();
          if (success !== continuePolling) {
            resolve(success);
            return
          } else {
            requestAnimationFrame(onRaf);
          }
        } catch (error) {
          reject(error);
          return;
        }
      }
    }

    async function pollInterval(pollInterval) {
      let resolve, reject;
      const result = new Promise((res, rej) => {
        resolve = res;
        reject = rej;
      });
      await onTimeout();
      return result;

      async function onTimeout() {
        try{
          if (timedOut) {
            reject(`timed out after ${timeout}ms`);
            return;
          }
          const success = predicate();
          if (success !== continuePolling) resolve(success);
          else setTimeout(onTimeout, pollInterval);
        } catch(error) {
          reject(error);
          return;
        }
      }
    }
  }

  waitForElementStates(node, states, timeout, ...args) {
    let lastRect = undefined;
    let counter = 0;
    let samePositionCounter = 0;
    let lastTime = 0;

    const predicate = () => {
      for (const state of states) {
        if (state !== "stable") {
          const result = this.checkElementState(node, state);
          if (typeof result !== "boolean") {
            return result;
          }
          if (!result) {
            return continuePolling;
          }
          continue;
        }

        const element = this._retarget(node, "no-follow-label");
        if (!element) {
          return "error:notconnected";
        }

        // First raf happens in the same animation frame as evaluation, so it does not produce
        // any client rect difference compared to synchronous call. We skip the synchronous call
        // and only force layout during actual rafs as a small optimisation.
        if (++counter === 1) {
          return continuePolling;
        }

        // Drop frames that are shorter than 16ms - WebKit Win bug.
        const time = performance.now();
        if (this._stableRafCount > 1 && time - lastTime < 15) {
          return continuePolling;
        }
        lastTime = time;

        const clientRect = element.getBoundingClientRect();
        const rect = {
          x: clientRect.top,
          y: clientRect.left,
          width: clientRect.width,
          height: clientRect.height,
        };
        const samePosition =
          lastRect &&
          rect.x === lastRect.x &&
          rect.y === lastRect.y &&
          rect.width === lastRect.width &&
          rect.height === lastRect.height;
        if (samePosition) {
          ++samePositionCounter;
        } else {
          samePositionCounter = 0;
        }
        const isStable = samePositionCounter >= this._stableRafCount;
        const isStableForLogs = isStable || !lastRect;
        lastRect = rect;
        if (!isStable) {
          return continuePolling;
        }
      }
      return true; // All states are good!
    };

    if (this._replaceRafWithTimeout) {
      return this.waitForPredicateFunction(predicate, 16, timeout, ...args);
    } else {
      return this.waitForPredicateFunction(predicate, "raf", timeout, ...args);
    }
  }

  waitForSelector(selector, root, strict, state, polling, timeout, ...args) {
    let lastElement;
    const predicate = () => {
      const elements = this.querySelectorAll(selector, root || document);
      const element = elements[0];
      const visible = element ? isVisible(element) : false;

      if (lastElement !== element) {
        lastElement = element;
        if (!element) {
          console.log(`  ${selector} did not match any elements`);
        } else {
          if (elements.length > 1) {
            if (strict) {
              throw "error:strictmodeviolation";
            }
          }
        }
      }

      switch (state) {
        case "attached":
          return element ? element : continuePolling;
        case "detached":
          return !element ? true : continuePolling;
        case "visible":
          return visible ? element : continuePolling;
        case "hidden":
          return !visible ? element : continuePolling;
      }
    };

    return this.waitForPredicateFunction(predicate, polling, timeout, ...args);
  }
}
