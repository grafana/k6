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

// k6BrowserNative allows accessing native browser objects
// even if the page under test has overridden them.
const k6BrowserNative = (() => {
  const iframe = document.createElement('iframe');
  // hide it offscreen with zero size
  iframe.style.position = 'absolute';
  iframe.style.width = '0';
  iframe.style.height = '0';
  iframe.style.border = '0';
  iframe.style.top = '-9999px';
  iframe.style.left = '-9999px';
  iframe.style.display = 'none';

  // grab the native browser object
  document.documentElement.appendChild(iframe);
  const win = iframe.contentWindow;
  document.documentElement.removeChild(iframe);

  return {
    Set: win.Set,
    Map: win.Map,
    // Add other native browser objects as needed.
  }
})();

// packages/playwright-core/src/utils/isomorphic/stringUtils.ts
var normalizedWhitespaceCache;
function normalizeWhiteSpace(text) {
  normalizedWhitespaceCache = normalizedWhitespaceCache || new k6BrowserNative.Map();
  let result = normalizedWhitespaceCache == null ? void 0 : normalizedWhitespaceCache.get(text);
  if (result === void 0) {
    result = text.replace(/[\u200b\u00ad]/g, "").trim().replace(/\s+/g, " ");
    normalizedWhitespaceCache == null ? void 0 : normalizedWhitespaceCache.set(text, result);
  }
  return result;
}

// packages/injected/src/domUtils.ts
var globalOptions = {};
function getGlobalOptions() {
  return globalOptions;
}
function parentElementOrShadowHost(element) {
  if (element.parentElement)
    return element.parentElement;
  if (!element.parentNode)
    return;
  if (element.parentNode.nodeType === 11 && element.parentNode.host)
    return element.parentNode.host;
}
function enclosingShadowRootOrDocument(element) {
  let node = element;
  while (node.parentNode)
    node = node.parentNode;
  if (node.nodeType === 11 || node.nodeType === 9)
    return node;
}
function enclosingShadowHost(element) {
  while (element.parentElement)
    element = element.parentElement;
  return parentElementOrShadowHost(element);
}
function closestCrossShadow(element, css, scope) {
  while (element) {
    const closest = element.closest(css);
    if (scope && closest !== scope && (closest == null ? void 0 : closest.contains(scope)))
      return;
    if (closest)
      return closest;
    element = enclosingShadowHost(element);
  }
}
function getElementComputedStyle(element, pseudo) {
  return element.ownerDocument && element.ownerDocument.defaultView ? element.ownerDocument.defaultView.getComputedStyle(element, pseudo) : void 0;
}
function isElementStyleVisibilityVisible(element, style) {
  style = style != null ? style : getElementComputedStyle(element);
  if (!style)
    return true;
  if (Element.prototype.checkVisibility && globalOptions.browserNameForWorkarounds !== "webkit") {
    if (!element.checkVisibility())
      return false;
  } else {
    const detailsOrSummary = element.closest("details,summary");
    if (detailsOrSummary !== element && (detailsOrSummary == null ? void 0 : detailsOrSummary.nodeName) === "DETAILS" && !detailsOrSummary.open)
      return false;
  }
  if (style.visibility !== "visible")
    return false;
  return true;
}
function isVisibleTextNode(node) {
  const range = node.ownerDocument.createRange();
  range.selectNode(node);
  const rect = range.getBoundingClientRect();
  return rect.width > 0 && rect.height > 0;
}
function elementSafeTagName(element) {
  if (element instanceof HTMLFormElement)
    return "FORM";
  return element.tagName.toUpperCase();
}

// packages/injected/src/roleUtils.ts
function hasExplicitAccessibleName(e) {
  return e.hasAttribute("aria-label") || e.hasAttribute("aria-labelledby");
}
var kAncestorPreventingLandmark = "article:not([role]), aside:not([role]), main:not([role]), nav:not([role]), section:not([role]), [role=article], [role=complementary], [role=main], [role=navigation], [role=region]";
var kGlobalAriaAttributes = [
  ["aria-atomic", void 0],
  ["aria-busy", void 0],
  ["aria-controls", void 0],
  ["aria-current", void 0],
  ["aria-describedby", void 0],
  ["aria-details", void 0],
  // Global use deprecated in ARIA 1.2
  // ['aria-disabled', undefined],
  ["aria-dropeffect", void 0],
  // Global use deprecated in ARIA 1.2
  // ['aria-errormessage', undefined],
  ["aria-flowto", void 0],
  ["aria-grabbed", void 0],
  // Global use deprecated in ARIA 1.2
  // ['aria-haspopup', undefined],
  ["aria-hidden", void 0],
  // Global use deprecated in ARIA 1.2
  // ['aria-invalid', undefined],
  ["aria-keyshortcuts", void 0],
  ["aria-label", ["caption", "code", "deletion", "emphasis", "generic", "insertion", "paragraph", "presentation", "strong", "subscript", "superscript"]],
  ["aria-labelledby", ["caption", "code", "deletion", "emphasis", "generic", "insertion", "paragraph", "presentation", "strong", "subscript", "superscript"]],
  ["aria-live", void 0],
  ["aria-owns", void 0],
  ["aria-relevant", void 0],
  ["aria-roledescription", ["generic"]]
];
function hasGlobalAriaAttribute(element, forRole) {
  return kGlobalAriaAttributes.some(([attr, prohibited]) => {
    return !(prohibited == null ? void 0 : prohibited.includes(forRole || "")) && element.hasAttribute(attr);
  });
}
function hasTabIndex(element) {
  return !Number.isNaN(Number(String(element.getAttribute("tabindex"))));
}
function isFocusable(element) {
  return !isNativelyDisabled(element) && (isNativelyFocusable(element) || hasTabIndex(element));
}
function isNativelyFocusable(element) {
  const tagName = elementSafeTagName(element);
  if (["BUTTON", "DETAILS", "SELECT", "TEXTAREA"].includes(tagName))
    return true;
  if (tagName === "A" || tagName === "AREA")
    return element.hasAttribute("href");
  if (tagName === "INPUT")
    return !element.hidden;
  return false;
}
var kImplicitRoleByTagName = {
  "A": (e) => {
    return e.hasAttribute("href") ? "link" : null;
  },
  "AREA": (e) => {
    return e.hasAttribute("href") ? "link" : null;
  },
  "ARTICLE": () => "article",
  "ASIDE": () => "complementary",
  "BLOCKQUOTE": () => "blockquote",
  "BUTTON": () => "button",
  "CAPTION": () => "caption",
  "CODE": () => "code",
  "DATALIST": () => "listbox",
  "DD": () => "definition",
  "DEL": () => "deletion",
  "DETAILS": () => "group",
  "DFN": () => "term",
  "DIALOG": () => "dialog",
  "DT": () => "term",
  "EM": () => "emphasis",
  "FIELDSET": () => "group",
  "FIGURE": () => "figure",
  "FOOTER": (e) => closestCrossShadow(e, kAncestorPreventingLandmark) ? null : "contentinfo",
  "FORM": (e) => hasExplicitAccessibleName(e) ? "form" : null,
  "H1": () => "heading",
  "H2": () => "heading",
  "H3": () => "heading",
  "H4": () => "heading",
  "H5": () => "heading",
  "H6": () => "heading",
  "HEADER": (e) => closestCrossShadow(e, kAncestorPreventingLandmark) ? null : "banner",
  "HR": () => "separator",
  "HTML": () => "document",
  "IMG": (e) => e.getAttribute("alt") === "" && !e.getAttribute("title") && !hasGlobalAriaAttribute(e) && !hasTabIndex(e) ? "presentation" : "img",
  "INPUT": (e) => {
    const type = e.type.toLowerCase();
    if (type === "search")
      return e.hasAttribute("list") ? "combobox" : "searchbox";
    if (["email", "tel", "text", "url", ""].includes(type)) {
      const list = getIdRefs(e, e.getAttribute("list"))[0];
      return list && elementSafeTagName(list) === "DATALIST" ? "combobox" : "textbox";
    }
    if (type === "hidden")
      return null;
    if (type === "file" && !getGlobalOptions().inputFileRoleTextbox)
      return "button";
    return inputTypeToRole[type] || "textbox";
  },
  "INS": () => "insertion",
  "LI": () => "listitem",
  "MAIN": () => "main",
  "MARK": () => "mark",
  "MATH": () => "math",
  "MENU": () => "list",
  "METER": () => "meter",
  "NAV": () => "navigation",
  "OL": () => "list",
  "OPTGROUP": () => "group",
  "OPTION": () => "option",
  "OUTPUT": () => "status",
  "P": () => "paragraph",
  "PROGRESS": () => "progressbar",
  "SECTION": (e) => hasExplicitAccessibleName(e) ? "region" : null,
  "SELECT": (e) => e.hasAttribute("multiple") || e.size > 1 ? "listbox" : "combobox",
  "STRONG": () => "strong",
  "SUB": () => "subscript",
  "SUP": () => "superscript",
  // For <svg> we default to Chrome behavior:
  // - Chrome reports 'img'.
  // - Firefox reports 'diagram' that is not in official ARIA spec yet.
  // - Safari reports 'no role', but still computes accessible name.
  "SVG": () => "img",
  "TABLE": () => "table",
  "TBODY": () => "rowgroup",
  "TD": (e) => {
    const table = closestCrossShadow(e, "table");
    const role = table ? getExplicitAriaRole(table) : "";
    return role === "grid" || role === "treegrid" ? "gridcell" : "cell";
  },
  "TEXTAREA": () => "textbox",
  "TFOOT": () => "rowgroup",
  "TH": (e) => {
    if (e.getAttribute("scope") === "col")
      return "columnheader";
    if (e.getAttribute("scope") === "row")
      return "rowheader";
    const table = closestCrossShadow(e, "table");
    const role = table ? getExplicitAriaRole(table) : "";
    return role === "grid" || role === "treegrid" ? "gridcell" : "cell";
  },
  "THEAD": () => "rowgroup",
  "TIME": () => "time",
  "TR": () => "row",
  "UL": () => "list"
};
var kPresentationInheritanceParents = {
  "DD": ["DL", "DIV"],
  "DIV": ["DL"],
  "DT": ["DL", "DIV"],
  "LI": ["OL", "UL"],
  "TBODY": ["TABLE"],
  "TD": ["TR"],
  "TFOOT": ["TABLE"],
  "TH": ["TR"],
  "THEAD": ["TABLE"],
  "TR": ["THEAD", "TBODY", "TFOOT", "TABLE"]
};
function getImplicitAriaRole(element) {
  var _a;
  const implicitRole = ((_a = kImplicitRoleByTagName[elementSafeTagName(element)]) == null ? void 0 : _a.call(kImplicitRoleByTagName, element)) || "";
  if (!implicitRole)
    return null;
  let ancestor = element;
  while (ancestor) {
    const parent = parentElementOrShadowHost(ancestor);
    const parents = kPresentationInheritanceParents[elementSafeTagName(ancestor)];
    if (!parents || !parent || !parents.includes(elementSafeTagName(parent)))
      break;
    const parentExplicitRole = getExplicitAriaRole(parent);
    if ((parentExplicitRole === "none" || parentExplicitRole === "presentation") && !hasPresentationConflictResolution(parent, parentExplicitRole))
      return parentExplicitRole;
    ancestor = parent;
  }
  return implicitRole;
}
var validRoles = [
  "alert",
  "alertdialog",
  "application",
  "article",
  "banner",
  "blockquote",
  "button",
  "caption",
  "cell",
  "checkbox",
  "code",
  "columnheader",
  "combobox",
  "complementary",
  "contentinfo",
  "definition",
  "deletion",
  "dialog",
  "directory",
  "document",
  "emphasis",
  "feed",
  "figure",
  "form",
  "generic",
  "grid",
  "gridcell",
  "group",
  "heading",
  "img",
  "insertion",
  "link",
  "list",
  "listbox",
  "listitem",
  "log",
  "main",
  "mark",
  "marquee",
  "math",
  "meter",
  "menu",
  "menubar",
  "menuitem",
  "menuitemcheckbox",
  "menuitemradio",
  "navigation",
  "none",
  "note",
  "option",
  "paragraph",
  "presentation",
  "progressbar",
  "radio",
  "radiogroup",
  "region",
  "row",
  "rowgroup",
  "rowheader",
  "scrollbar",
  "search",
  "searchbox",
  "separator",
  "slider",
  "spinbutton",
  "status",
  "strong",
  "subscript",
  "superscript",
  "switch",
  "tab",
  "table",
  "tablist",
  "tabpanel",
  "term",
  "textbox",
  "time",
  "timer",
  "toolbar",
  "tooltip",
  "tree",
  "treegrid",
  "treeitem"
];
function getExplicitAriaRole(element) {
  const roles = (element.getAttribute("role") || "").split(" ").map((role) => role.trim());
  return roles.find((role) => validRoles.includes(role)) || null;
}
function hasPresentationConflictResolution(element, role) {
  return hasGlobalAriaAttribute(element, role) || isFocusable(element);
}
function getAriaRole(element) {
  const explicitRole = getExplicitAriaRole(element);
  if (!explicitRole)
    return getImplicitAriaRole(element);
  if (explicitRole === "none" || explicitRole === "presentation") {
    const implicitRole = getImplicitAriaRole(element);
    if (hasPresentationConflictResolution(element, implicitRole))
      return implicitRole;
  }
  return explicitRole;
}
function getAriaBoolean(attr) {
  return attr === null ? void 0 : attr.toLowerCase() === "true";
}
function isElementIgnoredForAria(element) {
  return ["STYLE", "SCRIPT", "NOSCRIPT", "TEMPLATE"].includes(elementSafeTagName(element));
}
function isElementHiddenForAria(element) {
  if (isElementIgnoredForAria(element))
    return true;
  const style = getElementComputedStyle(element);
  const isSlot = element.nodeName === "SLOT";
  if ((style == null ? void 0 : style.display) === "contents" && !isSlot) {
    for (let child = element.firstChild; child; child = child.nextSibling) {
      if (child.nodeType === 1 && !isElementHiddenForAria(child))
        return false;
      if (child.nodeType === 3 && isVisibleTextNode(child))
        return false;
    }
    return true;
  }
  const isOptionInsideSelect = element.nodeName === "OPTION" && !!element.closest("select");
  if (!isOptionInsideSelect && !isSlot && !isElementStyleVisibilityVisible(element, style))
    return true;
  return belongsToDisplayNoneOrAriaHiddenOrNonSlotted(element);
}
function belongsToDisplayNoneOrAriaHiddenOrNonSlotted(element) {
  let hidden = cacheIsHidden == null ? void 0 : cacheIsHidden.get(element);
  if (hidden === void 0) {
    hidden = false;
    if (element.parentElement && element.parentElement.shadowRoot && !element.assignedSlot)
      hidden = true;
    if (!hidden) {
      const style = getElementComputedStyle(element);
      hidden = !style || style.display === "none" || getAriaBoolean(element.getAttribute("aria-hidden")) === true;
    }
    if (!hidden) {
      const parent = parentElementOrShadowHost(element);
      if (parent)
        hidden = belongsToDisplayNoneOrAriaHiddenOrNonSlotted(parent);
    }
    cacheIsHidden == null ? void 0 : cacheIsHidden.set(element, hidden);
  }
  return hidden;
}
function getIdRefs(element, ref) {
  if (!ref)
    return [];
  const root = enclosingShadowRootOrDocument(element);
  if (!root)
    return [];
  try {
    const ids = ref.split(" ").filter((id) => !!id);
    const result = [];
    for (const id of ids) {
      const firstElement = root.querySelector("#" + CSS.escape(id));
      if (firstElement && !result.includes(firstElement))
        result.push(firstElement);
    }
    return result;
  } catch (e) {
    return [];
  }
}
function trimFlatString(s) {
  return s.trim();
}
function asFlatString(s) {
  return s.split("\xA0").map((chunk) => chunk.replace(/\r\n/g, "\n").replace(/[\u200b\u00ad]/g, "").replace(/\s\s*/g, " ")).join("\xA0").trim();
}
function queryInAriaOwned(element, selector) {
  const result = [...element.querySelectorAll(selector)];
  for (const owned of getIdRefs(element, element.getAttribute("aria-owns"))) {
    if (owned.matches(selector))
      result.push(owned);
    result.push(...owned.querySelectorAll(selector));
  }
  return result;
}
function getPseudoContent(element, pseudo) {
  const cache = pseudo === "::before" ? cachePseudoContentBefore : cachePseudoContentAfter;
  if (cache == null ? void 0 : cache.has(element))
    return (cache == null ? void 0 : cache.get(element)) || "";
  const pseudoStyle = getElementComputedStyle(element, pseudo);
  const content = getPseudoContentImpl(element, pseudoStyle);
  if (cache)
    cache.set(element, content);
  return content;
}
function getPseudoContentImpl(element, pseudoStyle) {
  if (!pseudoStyle || pseudoStyle.display === "none" || pseudoStyle.visibility === "hidden")
    return "";
  const content = pseudoStyle.content;
  let resolvedContent;
  if (content[0] === "'" && content[content.length - 1] === "'" || content[0] === '"' && content[content.length - 1] === '"') {
    resolvedContent = content.substring(1, content.length - 1);
  } else if (content.startsWith("attr(") && content.endsWith(")")) {
    const attrName = content.substring("attr(".length, content.length - 1).trim();
    resolvedContent = element.getAttribute(attrName) || "";
  }
  if (resolvedContent !== void 0) {
    const display = pseudoStyle.display || "inline";
    if (display !== "inline")
      return " " + resolvedContent + " ";
    return resolvedContent;
  }
  return "";
}
function getAriaLabelledByElements(element) {
  const ref = element.getAttribute("aria-labelledby");
  if (ref === null)
    return null;
  const refs = getIdRefs(element, ref);
  return refs.length ? refs : null;
}
function allowsNameFromContent(role, targetDescendant) {
  const alwaysAllowsNameFromContent = ["button", "cell", "checkbox", "columnheader", "gridcell", "heading", "link", "menuitem", "menuitemcheckbox", "menuitemradio", "option", "radio", "row", "rowheader", "switch", "tab", "tooltip", "treeitem"].includes(role);
  const descendantAllowsNameFromContent = targetDescendant && ["", "caption", "code", "contentinfo", "definition", "deletion", "emphasis", "insertion", "list", "listitem", "mark", "none", "paragraph", "presentation", "region", "row", "rowgroup", "section", "strong", "subscript", "superscript", "table", "term", "time"].includes(role);
  return alwaysAllowsNameFromContent || descendantAllowsNameFromContent;
}
function getElementAccessibleName(element, includeHidden) {
  const cache = includeHidden ? cacheAccessibleNameHidden : cacheAccessibleName;
  let accessibleName = cache == null ? void 0 : cache.get(element);
  if (accessibleName === void 0) {
    accessibleName = "";
    const elementProhibitsNaming = ["caption", "code", "definition", "deletion", "emphasis", "generic", "insertion", "mark", "paragraph", "presentation", "strong", "subscript", "suggestion", "superscript", "term", "time"].includes(getAriaRole(element) || "");
    if (!elementProhibitsNaming) {
      accessibleName = asFlatString(getTextAlternativeInternal(element, {
        includeHidden,
        visitedElements: new k6BrowserNative.Set(),
        embeddedInTargetElement: "self"
      }));
    }
    cache == null ? void 0 : cache.set(element, accessibleName);
  }
  return accessibleName;
}
function getTextAlternativeInternal(element, options) {
  var _a, _b, _c, _d;
  if (options.visitedElements.has(element))
    return "";
  const childOptions = {
    ...options,
    embeddedInTargetElement: options.embeddedInTargetElement === "self" ? "descendant" : options.embeddedInTargetElement
  };
  if (!options.includeHidden) {
    const isEmbeddedInHiddenReferenceTraversal = !!((_a = options.embeddedInLabelledBy) == null ? void 0 : _a.hidden) || !!((_b = options.embeddedInDescribedBy) == null ? void 0 : _b.hidden) || !!((_c = options.embeddedInNativeTextAlternative) == null ? void 0 : _c.hidden) || !!((_d = options.embeddedInLabel) == null ? void 0 : _d.hidden);
    if (isElementIgnoredForAria(element) || !isEmbeddedInHiddenReferenceTraversal && isElementHiddenForAria(element)) {
      options.visitedElements.add(element);
      return "";
    }
  }
  const labelledBy = getAriaLabelledByElements(element);
  if (!options.embeddedInLabelledBy) {
    const accessibleName = (labelledBy || []).map((ref) => getTextAlternativeInternal(ref, {
      ...options,
      embeddedInLabelledBy: { element: ref, hidden: isElementHiddenForAria(ref) },
      embeddedInDescribedBy: void 0,
      embeddedInTargetElement: void 0,
      embeddedInLabel: void 0,
      embeddedInNativeTextAlternative: void 0
    })).join(" ");
    if (accessibleName)
      return accessibleName;
  }
  const role = getAriaRole(element) || "";
  const tagName = elementSafeTagName(element);
  if (!!options.embeddedInLabel || !!options.embeddedInLabelledBy || options.embeddedInTargetElement === "descendant") {
    const isOwnLabel = [...element.labels || []].includes(element);
    const isOwnLabelledBy = (labelledBy || []).includes(element);
    if (!isOwnLabel && !isOwnLabelledBy) {
      if (role === "textbox") {
        options.visitedElements.add(element);
        if (tagName === "INPUT" || tagName === "TEXTAREA")
          return element.value;
        return element.textContent || "";
      }
      if (["combobox", "listbox"].includes(role)) {
        options.visitedElements.add(element);
        let selectedOptions;
        if (tagName === "SELECT") {
          selectedOptions = [...element.selectedOptions];
          if (!selectedOptions.length && element.options.length)
            selectedOptions.push(element.options[0]);
        } else {
          const listbox = role === "combobox" ? queryInAriaOwned(element, "*").find((e) => getAriaRole(e) === "listbox") : element;
          selectedOptions = listbox ? queryInAriaOwned(listbox, '[aria-selected="true"]').filter((e) => getAriaRole(e) === "option") : [];
        }
        if (!selectedOptions.length && tagName === "INPUT") {
          return element.value;
        }
        return selectedOptions.map((option) => getTextAlternativeInternal(option, childOptions)).join(" ");
      }
      if (["progressbar", "scrollbar", "slider", "spinbutton", "meter"].includes(role)) {
        options.visitedElements.add(element);
        if (element.hasAttribute("aria-valuetext"))
          return element.getAttribute("aria-valuetext") || "";
        if (element.hasAttribute("aria-valuenow"))
          return element.getAttribute("aria-valuenow") || "";
        return element.getAttribute("value") || "";
      }
      if (["menu"].includes(role)) {
        options.visitedElements.add(element);
        return "";
      }
    }
  }
  const ariaLabel = element.getAttribute("aria-label") || "";
  if (trimFlatString(ariaLabel)) {
    options.visitedElements.add(element);
    return ariaLabel;
  }
  if (!["presentation", "none"].includes(role)) {
    if (tagName === "INPUT" && ["button", "submit", "reset"].includes(element.type)) {
      options.visitedElements.add(element);
      const value = element.value || "";
      if (trimFlatString(value))
        return value;
      if (element.type === "submit")
        return "Submit";
      if (element.type === "reset")
        return "Reset";
      const title = element.getAttribute("title") || "";
      return title;
    }
    if (!getGlobalOptions().inputFileRoleTextbox && tagName === "INPUT" && element.type === "file") {
      options.visitedElements.add(element);
      const labels = element.labels || [];
      if (labels.length && !options.embeddedInLabelledBy)
        return getAccessibleNameFromAssociatedLabels(labels, options);
      return "Choose File";
    }
    if (tagName === "INPUT" && element.type === "image") {
      options.visitedElements.add(element);
      const labels = element.labels || [];
      if (labels.length && !options.embeddedInLabelledBy)
        return getAccessibleNameFromAssociatedLabels(labels, options);
      const alt = element.getAttribute("alt") || "";
      if (trimFlatString(alt))
        return alt;
      const title = element.getAttribute("title") || "";
      if (trimFlatString(title))
        return title;
      return "Submit";
    }
    if (!labelledBy && tagName === "BUTTON") {
      options.visitedElements.add(element);
      const labels = element.labels || [];
      if (labels.length)
        return getAccessibleNameFromAssociatedLabels(labels, options);
    }
    if (!labelledBy && tagName === "OUTPUT") {
      options.visitedElements.add(element);
      const labels = element.labels || [];
      if (labels.length)
        return getAccessibleNameFromAssociatedLabels(labels, options);
      return element.getAttribute("title") || "";
    }
    if (!labelledBy && (tagName === "TEXTAREA" || tagName === "SELECT" || tagName === "INPUT")) {
      options.visitedElements.add(element);
      const labels = element.labels || [];
      if (labels.length)
        return getAccessibleNameFromAssociatedLabels(labels, options);
      const usePlaceholder = tagName === "INPUT" && ["text", "password", "search", "tel", "email", "url"].includes(element.type) || tagName === "TEXTAREA";
      const placeholder = element.getAttribute("placeholder") || "";
      const title = element.getAttribute("title") || "";
      if (!usePlaceholder || title)
        return title;
      return placeholder;
    }
    if (!labelledBy && tagName === "FIELDSET") {
      options.visitedElements.add(element);
      for (let child = element.firstElementChild; child; child = child.nextElementSibling) {
        if (elementSafeTagName(child) === "LEGEND") {
          return getTextAlternativeInternal(child, {
            ...childOptions,
            embeddedInNativeTextAlternative: { element: child, hidden: isElementHiddenForAria(child) }
          });
        }
      }
      const title = element.getAttribute("title") || "";
      return title;
    }
    if (!labelledBy && tagName === "FIGURE") {
      options.visitedElements.add(element);
      for (let child = element.firstElementChild; child; child = child.nextElementSibling) {
        if (elementSafeTagName(child) === "FIGCAPTION") {
          return getTextAlternativeInternal(child, {
            ...childOptions,
            embeddedInNativeTextAlternative: { element: child, hidden: isElementHiddenForAria(child) }
          });
        }
      }
      const title = element.getAttribute("title") || "";
      return title;
    }
    if (tagName === "IMG") {
      options.visitedElements.add(element);
      const alt = element.getAttribute("alt") || "";
      if (trimFlatString(alt))
        return alt;
      const title = element.getAttribute("title") || "";
      return title;
    }
    if (tagName === "TABLE") {
      options.visitedElements.add(element);
      for (let child = element.firstElementChild; child; child = child.nextElementSibling) {
        if (elementSafeTagName(child) === "CAPTION") {
          return getTextAlternativeInternal(child, {
            ...childOptions,
            embeddedInNativeTextAlternative: { element: child, hidden: isElementHiddenForAria(child) }
          });
        }
      }
      const summary = element.getAttribute("summary") || "";
      if (summary)
        return summary;
    }
    if (tagName === "AREA") {
      options.visitedElements.add(element);
      const alt = element.getAttribute("alt") || "";
      if (trimFlatString(alt))
        return alt;
      const title = element.getAttribute("title") || "";
      return title;
    }
    if (tagName === "SVG" || element.ownerSVGElement) {
      options.visitedElements.add(element);
      for (let child = element.firstElementChild; child; child = child.nextElementSibling) {
        if (elementSafeTagName(child) === "TITLE" && child.ownerSVGElement) {
          return getTextAlternativeInternal(child, {
            ...childOptions,
            embeddedInLabelledBy: { element: child, hidden: isElementHiddenForAria(child) }
          });
        }
      }
    }
    if (element.ownerSVGElement && tagName === "A") {
      const title = element.getAttribute("xlink:title") || "";
      if (trimFlatString(title)) {
        options.visitedElements.add(element);
        return title;
      }
    }
  }
  const shouldNameFromContentForSummary = tagName === "SUMMARY" && !["presentation", "none"].includes(role);
  if (allowsNameFromContent(role, options.embeddedInTargetElement === "descendant") || shouldNameFromContentForSummary || !!options.embeddedInLabelledBy || !!options.embeddedInDescribedBy || !!options.embeddedInLabel || !!options.embeddedInNativeTextAlternative) {
    options.visitedElements.add(element);
    const accessibleName = innerAccumulatedElementText(element, childOptions);
    const maybeTrimmedAccessibleName = options.embeddedInTargetElement === "self" ? trimFlatString(accessibleName) : accessibleName;
    if (maybeTrimmedAccessibleName)
      return accessibleName;
  }
  if (!["presentation", "none"].includes(role) || tagName === "IFRAME") {
    options.visitedElements.add(element);
    const title = element.getAttribute("title") || "";
    if (trimFlatString(title))
      return title;
  }
  options.visitedElements.add(element);
  return "";
}
function innerAccumulatedElementText(element, options) {
  const tokens = [];
  const visit = (node, skipSlotted) => {
    var _a;
    if (skipSlotted && node.assignedSlot)
      return;
    if (node.nodeType === 1) {
      const display = ((_a = getElementComputedStyle(node)) == null ? void 0 : _a.display) || "inline";
      let token = getTextAlternativeInternal(node, options);
      if (display !== "inline" || node.nodeName === "BR")
        token = " " + token + " ";
      tokens.push(token);
    } else if (node.nodeType === 3) {
      tokens.push(node.textContent || "");
    }
  };
  tokens.push(getPseudoContent(element, "::before"));
  const assignedNodes = element.nodeName === "SLOT" ? element.assignedNodes() : [];
  if (assignedNodes.length) {
    for (const child of assignedNodes)
      visit(child, false);
  } else {
    for (let child = element.firstChild; child; child = child.nextSibling)
      visit(child, true);
    if (element.shadowRoot) {
      for (let child = element.shadowRoot.firstChild; child; child = child.nextSibling)
        visit(child, true);
    }
    for (const owned of getIdRefs(element, element.getAttribute("aria-owns")))
      visit(owned, true);
  }
  tokens.push(getPseudoContent(element, "::after"));
  return tokens.join("");
}
var kAriaSelectedRoles = ["gridcell", "option", "row", "tab", "rowheader", "columnheader", "treeitem"];
function getAriaSelected(element) {
  if (elementSafeTagName(element) === "OPTION")
    return element.selected;
  if (kAriaSelectedRoles.includes(getAriaRole(element) || ""))
    return getAriaBoolean(element.getAttribute("aria-selected")) === true;
  return false;
}
var kAriaCheckedRoles = ["checkbox", "menuitemcheckbox", "option", "radio", "switch", "menuitemradio", "treeitem"];
function getAriaChecked(element) {
  const result = getChecked(element, true);
  return result === "error" ? false : result;
}
function getChecked(element, allowMixed) {
  const tagName = elementSafeTagName(element);
  if (allowMixed && tagName === "INPUT" && element.indeterminate)
    return "mixed";
  if (tagName === "INPUT" && ["checkbox", "radio"].includes(element.type))
    return element.checked;
  if (kAriaCheckedRoles.includes(getAriaRole(element) || "")) {
    const checked = element.getAttribute("aria-checked");
    if (checked === "true")
      return true;
    if (allowMixed && checked === "mixed")
      return "mixed";
    return false;
  }
  return "error";
}
var kAriaPressedRoles = ["button"];
function getAriaPressed(element) {
  if (kAriaPressedRoles.includes(getAriaRole(element) || "")) {
    const pressed = element.getAttribute("aria-pressed");
    if (pressed === "true")
      return true;
    if (pressed === "mixed")
      return "mixed";
  }
  return false;
}
var kAriaExpandedRoles = ["application", "button", "checkbox", "combobox", "gridcell", "link", "listbox", "menuitem", "row", "tab", "treeitem", "columnheader", "menuitemcheckbox", "menuitemradio", "rowheader", "switch"];
function getAriaExpanded(element) {
  if (elementSafeTagName(element) === "DETAILS")
    return element.open;
  if (kAriaExpandedRoles.includes(getAriaRole(element) || "")) {
    const expanded = element.getAttribute("aria-expanded");
    if (expanded === null)
      return void 0;
    if (expanded === "true")
      return true;
    return false;
  }
  return void 0;
}
var kAriaLevelRoles = ["heading", "listitem", "row", "treeitem"];
function getAriaLevel(element) {
  const native = { "H1": 1, "H2": 2, "H3": 3, "H4": 4, "H5": 5, "H6": 6 }[elementSafeTagName(element)];
  if (native)
    return native;
  if (kAriaLevelRoles.includes(getAriaRole(element) || "")) {
    const attr = element.getAttribute("aria-level");
    const value = attr === null ? Number.NaN : Number(attr);
    if (Number.isInteger(value) && value >= 1)
      return value;
  }
  return 0;
}
var kAriaDisabledRoles = ["application", "button", "composite", "gridcell", "group", "input", "link", "menuitem", "scrollbar", "separator", "tab", "checkbox", "columnheader", "combobox", "grid", "listbox", "menu", "menubar", "menuitemcheckbox", "menuitemradio", "option", "radio", "radiogroup", "row", "rowheader", "searchbox", "select", "slider", "spinbutton", "switch", "tablist", "textbox", "toolbar", "tree", "treegrid", "treeitem"];
function getAriaDisabled(element) {
  return isNativelyDisabled(element) || hasExplicitAriaDisabled(element);
}
function isNativelyDisabled(element) {
  const isNativeFormControl = ["BUTTON", "INPUT", "SELECT", "TEXTAREA", "OPTION", "OPTGROUP"].includes(element.tagName);
  return isNativeFormControl && (element.hasAttribute("disabled") || belongsToDisabledFieldSet(element));
}
function belongsToDisabledFieldSet(element) {
  const fieldSetElement = element == null ? void 0 : element.closest("FIELDSET[DISABLED]");
  if (!fieldSetElement)
    return false;
  const legendElement = fieldSetElement.querySelector(":scope > LEGEND");
  return !legendElement || !legendElement.contains(element);
}
function hasExplicitAriaDisabled(element, isAncestor = false) {
  if (!element)
    return false;
  if (isAncestor || kAriaDisabledRoles.includes(getAriaRole(element) || "")) {
    const attribute = (element.getAttribute("aria-disabled") || "").toLowerCase();
    if (attribute === "true")
      return true;
    if (attribute === "false")
      return false;
    return hasExplicitAriaDisabled(parentElementOrShadowHost(element), true);
  }
  return false;
}
function getAccessibleNameFromAssociatedLabels(labels, options) {
  return [...labels].map((label) => getTextAlternativeInternal(label, {
    ...options,
    embeddedInLabel: { element: label, hidden: isElementHiddenForAria(label) },
    embeddedInNativeTextAlternative: void 0,
    embeddedInLabelledBy: void 0,
    embeddedInDescribedBy: void 0,
    embeddedInTargetElement: void 0
  })).filter((accessibleName) => !!accessibleName).join(" ");
}
var cacheAccessibleName;
var cacheAccessibleNameHidden;
var cacheAccessibleDescription;
var cacheAccessibleDescriptionHidden;
var cacheAccessibleErrorMessage;
var cacheIsHidden;
var cachePseudoContentBefore;
var cachePseudoContentAfter;
var cachesCounter = 0;
function beginAriaCaches() {
  ++cachesCounter;
  cacheAccessibleName != null ? cacheAccessibleName : cacheAccessibleName = new k6BrowserNative.Map();
  cacheAccessibleNameHidden != null ? cacheAccessibleNameHidden : cacheAccessibleNameHidden = new k6BrowserNative.Map();
  cacheAccessibleDescription != null ? cacheAccessibleDescription : cacheAccessibleDescription = new k6BrowserNative.Map();
  cacheAccessibleDescriptionHidden != null ? cacheAccessibleDescriptionHidden : cacheAccessibleDescriptionHidden = new k6BrowserNative.Map();
  cacheAccessibleErrorMessage != null ? cacheAccessibleErrorMessage : cacheAccessibleErrorMessage = new k6BrowserNative.Map();
  cacheIsHidden != null ? cacheIsHidden : cacheIsHidden = new k6BrowserNative.Map();
  cachePseudoContentBefore != null ? cachePseudoContentBefore : cachePseudoContentBefore = new k6BrowserNative.Map();
  cachePseudoContentAfter != null ? cachePseudoContentAfter : cachePseudoContentAfter = new k6BrowserNative.Map();
}
function endAriaCaches() {
  if (!--cachesCounter) {
    cacheAccessibleName = void 0;
    cacheAccessibleNameHidden = void 0;
    cacheAccessibleDescription = void 0;
    cacheAccessibleDescriptionHidden = void 0;
    cacheAccessibleErrorMessage = void 0;
    cacheIsHidden = void 0;
    cachePseudoContentBefore = void 0;
    cachePseudoContentAfter = void 0;
  }
}
var inputTypeToRole = {
  "button": "button",
  "checkbox": "checkbox",
  "image": "button",
  "number": "spinbutton",
  "radio": "radio",
  "range": "slider",
  "reset": "button",
  "submit": "button"
};

// packages/injected/src/selectorUtils.ts
function matchesAttributePart(value, attr) {
  const objValue = typeof value === "string" && !attr.caseSensitive ? value.toUpperCase() : value;
  const attrValue = typeof attr.value === "string" && !attr.caseSensitive ? attr.value.toUpperCase() : attr.value;
  if (attr.op === "<truthy>")
    return !!objValue;
  if (attr.op === "=") {
    if (attrValue instanceof RegExp)
      return typeof objValue === "string" && !!objValue.match(attrValue);
    return objValue === attrValue;
  }
  if (typeof objValue !== "string" || typeof attrValue !== "string")
    return false;
  if (attr.op === "*=")
    return objValue.includes(attrValue);
  if (attr.op === "^=")
    return objValue.startsWith(attrValue);
  if (attr.op === "$=")
    return objValue.endsWith(attrValue);
  if (attr.op === "|=")
    return objValue === attrValue || objValue.startsWith(attrValue + "-");
  if (attr.op === "~=")
    return objValue.split(" ").includes(attrValue);
  return false;
}

// packages/playwright-core/src/utils/isomorphic/cssParser.ts
var InvalidSelectorError = class extends Error {
};

// packages/playwright-core/src/utils/isomorphic/selectorParser.ts
function parseAttributeSelector(selector, allowUnquotedStrings) {
  let wp = 0;
  let EOL = selector.length === 0;
  const next = () => selector[wp] || "";
  const eat1 = () => {
    const result2 = next();
    ++wp;
    EOL = wp >= selector.length;
    return result2;
  };
  const syntaxError = (stage) => {
    if (EOL)
      throw new InvalidSelectorError(`Unexpected end of selector while parsing selector \`${selector}\``);
    throw new InvalidSelectorError(`Error while parsing selector \`${selector}\` - unexpected symbol "${next()}" at position ${wp}` + (stage ? " during " + stage : ""));
  };
  function skipSpaces() {
    while (!EOL && /\s/.test(next()))
      eat1();
  }
  function isCSSNameChar(char) {
    return char >= "\x80" || char >= "0" && char <= "9" || char >= "A" && char <= "Z" || char >= "a" && char <= "z" || char >= "0" && char <= "9" || char === "_" || char === "-";
  }
  function readIdentifier() {
    let result2 = "";
    skipSpaces();
    while (!EOL && isCSSNameChar(next()))
      result2 += eat1();
    return result2;
  }
  function readQuotedString(quote) {
    let result2 = eat1();
    if (result2 !== quote)
      syntaxError("parsing quoted string");
    while (!EOL && next() !== quote) {
      if (next() === "\\")
        eat1();
      result2 += eat1();
    }
    if (next() !== quote)
      syntaxError("parsing quoted string");
    result2 += eat1();
    return result2;
  }
  function readRegularExpression() {
    if (eat1() !== "/")
      syntaxError("parsing regular expression");
    let source = "";
    let inClass = false;
    while (!EOL) {
      if (next() === "\\") {
        source += eat1();
        if (EOL)
          syntaxError("parsing regular expression");
      } else if (inClass && next() === "]") {
        inClass = false;
      } else if (!inClass && next() === "[") {
        inClass = true;
      } else if (!inClass && next() === "/") {
        break;
      }
      source += eat1();
    }
    if (eat1() !== "/")
      syntaxError("parsing regular expression");
    let flags = "";
    while (!EOL && next().match(/[dgimsuy]/))
      flags += eat1();
    try {
      return new RegExp(source, flags);
    } catch (e) {
      throw new InvalidSelectorError(`Error while parsing selector \`${selector}\`: ${e.message}`);
    }
  }
  function readAttributeToken() {
    let token = "";
    skipSpaces();
    if (next() === `'` || next() === `"`)
      token = readQuotedString(next()).slice(1, -1);
    else
      token = readIdentifier();
    if (!token)
      syntaxError("parsing property path");
    return token;
  }
  function readOperator() {
    skipSpaces();
    let op = "";
    if (!EOL)
      op += eat1();
    if (!EOL && op !== "=")
      op += eat1();
    if (!["=", "*=", "^=", "$=", "|=", "~="].includes(op))
      syntaxError("parsing operator");
    return op;
  }
  function readAttribute() {
    eat1();
    const jsonPath = [];
    jsonPath.push(readAttributeToken());
    skipSpaces();
    while (next() === ".") {
      eat1();
      jsonPath.push(readAttributeToken());
      skipSpaces();
    }
    if (next() === "]") {
      eat1();
      return { name: jsonPath.join("."), jsonPath, op: "<truthy>", value: null, caseSensitive: false };
    }
    const operator = readOperator();
    let value = void 0;
    let caseSensitive = true;
    skipSpaces();
    if (next() === "/") {
      if (operator !== "=")
        throw new InvalidSelectorError(`Error while parsing selector \`${selector}\` - cannot use ${operator} in attribute with regular expression`);
      value = readRegularExpression();
    } else if (next() === `'` || next() === `"`) {
      value = readQuotedString(next()).slice(1, -1);
      skipSpaces();
      if (next() === "i" || next() === "I") {
        caseSensitive = false;
        eat1();
      } else if (next() === "s" || next() === "S") {
        caseSensitive = true;
        eat1();
      }
    } else {
      value = "";
      while (!EOL && (isCSSNameChar(next()) || next() === "+" || next() === "."))
        value += eat1();
      if (value === "true") {
        value = true;
      } else if (value === "false") {
        value = false;
      } else {
        if (!allowUnquotedStrings) {
          value = +value;
          if (Number.isNaN(value))
            syntaxError("parsing attribute value");
        }
      }
    }
    skipSpaces();
    if (next() !== "]")
      syntaxError("parsing attribute value");
    eat1();
    if (operator !== "=" && typeof value !== "string")
      throw new InvalidSelectorError(`Error while parsing selector \`${selector}\` - cannot use ${operator} in attribute with non-string matching value - ${value}`);
    return { name: jsonPath.join("."), jsonPath, op: operator, value, caseSensitive };
  }
  const result = {
    name: "",
    attributes: []
  };
  result.name = readIdentifier();
  skipSpaces();
  while (next() === "[") {
    result.attributes.push(readAttribute());
    skipSpaces();
  }
  if (!EOL)
    syntaxError(void 0);
  if (!result.name && !result.attributes.length)
    throw new InvalidSelectorError(`Error while parsing selector \`${selector}\` - selector cannot be empty`);
  return result;
}

// packages/injected/src/roleSelectorEngine.ts
var kSupportedAttributes = ["selected", "checked", "pressed", "expanded", "level", "disabled", "name", "include-hidden"];
kSupportedAttributes.sort();
function validateSupportedRole(attr, roles, role) {
  if (!roles.includes(role))
    throw new Error(`"${attr}" attribute is only supported for roles: ${roles.slice().sort().map((role2) => `"${role2}"`).join(", ")}`);
}
function validateSupportedValues(attr, values) {
  if (attr.op !== "<truthy>" && !values.includes(attr.value))
    throw new Error(`"${attr.name}" must be one of ${values.map((v) => JSON.stringify(v)).join(", ")}`);
}
function validateSupportedOp(attr, ops) {
  if (!ops.includes(attr.op))
    throw new Error(`"${attr.name}" does not support "${attr.op}" matcher`);
}
function validateAttributes(attrs, role) {
  const options = { role };
  for (const attr of attrs) {
    switch (attr.name) {
      case "checked": {
        validateSupportedRole(attr.name, kAriaCheckedRoles, role);
        validateSupportedValues(attr, [true, false, "mixed"]);
        validateSupportedOp(attr, ["<truthy>", "="]);
        options.checked = attr.op === "<truthy>" ? true : attr.value;
        break;
      }
      case "pressed": {
        validateSupportedRole(attr.name, kAriaPressedRoles, role);
        validateSupportedValues(attr, [true, false, "mixed"]);
        validateSupportedOp(attr, ["<truthy>", "="]);
        options.pressed = attr.op === "<truthy>" ? true : attr.value;
        break;
      }
      case "selected": {
        validateSupportedRole(attr.name, kAriaSelectedRoles, role);
        validateSupportedValues(attr, [true, false]);
        validateSupportedOp(attr, ["<truthy>", "="]);
        options.selected = attr.op === "<truthy>" ? true : attr.value;
        break;
      }
      case "expanded": {
        validateSupportedRole(attr.name, kAriaExpandedRoles, role);
        validateSupportedValues(attr, [true, false]);
        validateSupportedOp(attr, ["<truthy>", "="]);
        options.expanded = attr.op === "<truthy>" ? true : attr.value;
        break;
      }
      case "level": {
        validateSupportedRole(attr.name, kAriaLevelRoles, role);
        if (typeof attr.value === "string")
          attr.value = +attr.value;
        if (attr.op !== "=" || typeof attr.value !== "number" || Number.isNaN(attr.value))
          throw new Error(`"level" attribute must be compared to a number`);
        options.level = attr.value;
        break;
      }
      case "disabled": {
        validateSupportedValues(attr, [true, false]);
        validateSupportedOp(attr, ["<truthy>", "="]);
        options.disabled = attr.op === "<truthy>" ? true : attr.value;
        break;
      }
      case "name": {
        if (attr.op === "<truthy>")
          throw new Error(`"name" attribute must have a value`);
        if (typeof attr.value !== "string" && !(attr.value instanceof RegExp))
          throw new Error(`"name" attribute must be a string or a regular expression`);
        options.name = attr.value;
        options.nameOp = attr.op;
        options.exact = attr.caseSensitive;
        break;
      }
      case "include-hidden": {
        validateSupportedValues(attr, [true, false]);
        validateSupportedOp(attr, ["<truthy>", "="]);
        options.includeHidden = attr.op === "<truthy>" ? true : attr.value;
        break;
      }
      default: {
        throw new Error(`Unknown attribute "${attr.name}", must be one of ${kSupportedAttributes.map((a) => `"${a}"`).join(", ")}.`);
      }
    }
  }
  return options;
}
function queryRole(scope, options, internal) {
  const result = [];
  const match = (element) => {
    if (getAriaRole(element) !== options.role)
      return;
    if (options.selected !== void 0 && getAriaSelected(element) !== options.selected)
      return;
    if (options.checked !== void 0 && getAriaChecked(element) !== options.checked)
      return;
    if (options.pressed !== void 0 && getAriaPressed(element) !== options.pressed)
      return;
    if (options.expanded !== void 0 && getAriaExpanded(element) !== options.expanded)
      return;
    if (options.level !== void 0 && getAriaLevel(element) !== options.level)
      return;
    if (options.disabled !== void 0 && getAriaDisabled(element) !== options.disabled)
      return;
    if (!options.includeHidden) {
      const isHidden = isElementHiddenForAria(element);
      if (isHidden)
        return;
    }
    if (options.name !== void 0) {
      const accessibleName = normalizeWhiteSpace(getElementAccessibleName(element, !!options.includeHidden));
      if (typeof options.name === "string")
        options.name = normalizeWhiteSpace(options.name);
      if (internal && !options.exact && options.nameOp === "=")
        options.nameOp = "*=";
      if (!matchesAttributePart(accessibleName, { name: "", jsonPath: [], op: options.nameOp || "=", value: options.name, caseSensitive: !!options.exact }))
        return;
    }
    result.push(element);
  };
  const query = (root) => {
    const shadows = [];
    if (root.shadowRoot)
      shadows.push(root.shadowRoot);
    for (const element of root.querySelectorAll("*")) {
      match(element);
      if (element.shadowRoot)
        shadows.push(element.shadowRoot);
    }
    shadows.forEach(query);
  };
  query(scope);
  return result;
}

function createRoleEngine(internal) {
  return {
    queryAll: (scope, selector) => {
      const parsed = parseAttributeSelector(selector, true);
      const role = parsed.name.toLowerCase();
      if (!role)
        throw new Error(`Role must not be empty`);
      const options = validateAttributes(parsed.attributes, role);
      beginAriaCaches();
      try {
        return queryRole(scope, options, internal);
      } finally {
        endAriaCaches();
      }
    }
  };
}

// packages/injected/src/selectorEvaluator.ts
var SelectorEvaluatorImpl = class {
  constructor() {
    this._retainCacheCounter = 0;
    this._cacheText = new k6BrowserNative.Map();
    this._cacheQueryCSS = new k6BrowserNative.Map();
    this._cacheShadowRoots = new k6BrowserNative.Map();
  }
  begin() {
    ++this._retainCacheCounter;
  }
  end() {
    --this._retainCacheCounter;
    if (!this._retainCacheCounter) {
      this._cacheQueryCSS.clear();
      this._cacheText.clear();
      this._cacheShadowRoots.clear();
    }
  }
  _cached(cache, main, rest, cb) {
    if (!cache.has(main))
      cache.set(main, []);
    const entries = cache.get(main);
    const entry = entries.find((e) => rest.every((value, index) => e.rest[index] === value));
    if (entry)
      return entry.result;
    const result = cb();
    entries.push({ rest, result });
    return result;
  }
  _queryCSS(context, css) {
    return this._cached(this._cacheQueryCSS, css, [context.scope, context.pierceShadow, context.originalScope], () => {
      let result = [];
      function query(root) {
        result = result.concat([...root.querySelectorAll(css)]);
        if (!context.pierceShadow)
          return;
        if (root.shadowRoot)
          query(root.shadowRoot);
        for (const element of root.querySelectorAll("*")) {
          if (element.shadowRoot)
            query(element.shadowRoot);
        }
      }
      query(context.scope);
      return result;
    });
  }
};

// packages/injected/src/selectorUtils.ts
function shouldSkipForTextMatching(element) {
  const document = element.ownerDocument;
  return element.nodeName === "SCRIPT" || element.nodeName === "NOSCRIPT" || element.nodeName === "STYLE" || document.head && document.head.contains(element);
}
function elementText(cache, root) {
  let value = cache.get(root);
  if (value === void 0) {
    value = { full: "", normalized: "", immediate: [] };
    if (!shouldSkipForTextMatching(root)) {
      let currentImmediate = "";
      if (root instanceof HTMLInputElement && (root.type === "submit" || root.type === "button")) {
        value = { full: root.value, normalized: normalizeWhiteSpace(root.value), immediate: [root.value] };
      } else {
        for (let child = root.firstChild; child; child = child.nextSibling) {
          if (child.nodeType === Node.TEXT_NODE) {
            value.full += child.nodeValue || "";
            currentImmediate += child.nodeValue || "";
          } else if (child.nodeType === Node.COMMENT_NODE) {
            continue;
          } else {
            if (currentImmediate)
              value.immediate.push(currentImmediate);
            currentImmediate = "";
            if (child.nodeType === Node.ELEMENT_NODE)
              value.full += elementText(cache, child).full;
          }
        }
        if (currentImmediate)
          value.immediate.push(currentImmediate);
        if (root.shadowRoot)
          value.full += elementText(cache, root.shadowRoot).full;
        if (value.full)
          value.normalized = normalizeWhiteSpace(value.full);
      }
    }
    cache.set(root, value);
  }
  return value;
}
function getElementLabels(textCache, element) {
  const labels = getAriaLabelledByElements(element);
  if (labels)
    return labels.map((label) => elementText(textCache, label));
  const ariaLabel = element.getAttribute("aria-label");
  if (ariaLabel !== null && !!ariaLabel.trim())
    return [{ full: ariaLabel, normalized: normalizeWhiteSpace(ariaLabel), immediate: [ariaLabel] }];
  const isNonHiddenInput = element.nodeName === "INPUT" && element.type !== "hidden";
  if (["BUTTON", "METER", "OUTPUT", "PROGRESS", "SELECT", "TEXTAREA"].includes(element.nodeName) || isNonHiddenInput) {
    const labels2 = element.labels;
    if (labels2)
      return [...labels2].map((label) => elementText(textCache, label));
  }
  return [];
}
function elementMatchesText(cache, element, matcher) {
  if (shouldSkipForTextMatching(element))
    return "none";
  if (!matcher(elementText(cache, element)))
    return "none";
  for (let child = element.firstChild; child; child = child.nextSibling) {
    if (child.nodeType === Node.ELEMENT_NODE && matcher(elementText(cache, child)))
      return "selfAndChildren";
  }
  if (element.shadowRoot && matcher(elementText(cache, element.shadowRoot)))
    return "selfAndChildren";
  return "self";
}

// packages/injected/src/injectedScript.ts
function cssUnquote(s) {
  s = s.substring(1, s.length - 1);
  if (!s.includes("\\"))
    return s;
  const r = [];
  let i = 0;
  while (i < s.length) {
    if (s[i] === "\\" && i + 1 < s.length)
      i++;
    r.push(s[i++]);
  }
  return r.join("");
}
function createTextMatcher(selector, internal) {
  if (selector[0] === "/" && selector.lastIndexOf("/") > 0) {
    const lastSlash = selector.lastIndexOf("/");
    const re = new RegExp(selector.substring(1, lastSlash), selector.substring(lastSlash + 1));
    return { matcher: (elementText2) => re.test(elementText2.full), kind: "regex" };
  }
  const unquote = internal ? JSON.parse.bind(JSON) : cssUnquote;
  let strict = false;
  if (selector.length > 1 && selector[0] === '"' && selector[selector.length - 1] === '"') {
    selector = unquote(selector);
    strict = true;
  } else if (internal && selector.length > 1 && selector[0] === '"' && selector[selector.length - 2] === '"' && selector[selector.length - 1] === "i") {
    selector = unquote(selector.substring(0, selector.length - 1));
    strict = false;
  } else if (internal && selector.length > 1 && selector[0] === '"' && selector[selector.length - 2] === '"' && selector[selector.length - 1] === "s") {
    selector = unquote(selector.substring(0, selector.length - 1));
    strict = true;
  } else if (selector.length > 1 && selector[0] === "'" && selector[selector.length - 1] === "'") {
    selector = unquote(selector);
    strict = true;
  }
  selector = normalizeWhiteSpace(selector);
  if (strict) {
    if (internal)
      return { kind: "strict", matcher: (elementText2) => elementText2.normalized === selector };
    const strictTextNodeMatcher = (elementText2) => {
      if (!selector && !elementText2.immediate.length)
        return true;
      return elementText2.immediate.some((s) => normalizeWhiteSpace(s) === selector);
    };
    return { matcher: strictTextNodeMatcher, kind: "strict" };
  }
  selector = selector.toLowerCase();
  return { kind: "lax", matcher: (elementText2) => elementText2.normalized.toLowerCase().includes(selector) };
}


const autoClosingTags = new k6BrowserNative.Set([
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
const booleanAttributes = new k6BrowserNative.Set([
  "checked",
  "selected",
  "disabled",
  "readonly",
  "multiple",
]);
const eventType = new k6BrowserNative.Map([
  ["auxclick", "mouse"],
  ["click", "mouse"],
  ["dblclick", "mouse"],
  ["mousedown", "mouse"],
  ["mouseenter", "mouse"],
  ["mouseleave", "mouse"],
  ["mousemove", "mouse"],
  ["mouseout", "mouse"],
  ["mouseover", "mouse"],
  ["mouseup", "mouse"],
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

// Engines that look inside shadow roots themselves, or that only test the root
// element, so running them again in each shadow root finds nothing new.
var kShadowPiercingEngines = ["internal:attr", "internal:has-not-text", "internal:has-text", "internal:label", "internal:role", "internal:text", "text"];

class CSSQueryEngine {
  queryAll(root, selector) {
    return root.querySelectorAll(selector);
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

class AttributeEngine {
  constructor(evaluator) {
    this._evaluator = evaluator;
  }
  queryAll(root, selector) {
    try {
      this._evaluator.begin();

      const parsed = parseAttributeSelector(selector, true);
      if (parsed.name || parsed.attributes.length !== 1)
        throw new Error("Malformed attribute selector: " + selector);
      const { name, value, caseSensitive } = parsed.attributes[0];
      const lowerCaseValue = caseSensitive ? null : value.toLowerCase();
      let matcher;
      if (value instanceof RegExp)
        matcher = (s) => !!s.match(value);
      else if (caseSensitive)
        matcher = (s) => s === value;
      else
        matcher = (s) => s.toLowerCase().includes(lowerCaseValue);
      const elements = this._evaluator._queryCSS({ scope: root, pierceShadow: true }, `[${name}]`);
      return elements.filter((e) => matcher(e.getAttribute(name)));
    } finally {
      this._evaluator.end();
    }
  };
}

class LabelEngine {
  constructor(evaluator) {
    this._evaluator = evaluator;
  }
  queryAll(root, selector) {
    try {
      this._evaluator.begin();

      const { matcher } = createTextMatcher(selector, true);
      const allElements = this._evaluator._queryCSS({ scope: root, pierceShadow: true }, "*");
      return allElements.filter((element) => {
        return getElementLabels(this._evaluator._cacheText, element).some((label) => matcher(label));
      });
    } finally {
      this._evaluator.end();
    }
  }
}

class TextEngine {
  constructor(evaluator, shadow, internal) {
    this._evaluator = evaluator;
    this._shadow = shadow;
    this._internal = internal;
  }
  queryAll(root, selector) {
    try {
      this._evaluator.begin();

      const { matcher, kind } = createTextMatcher(selector, this._internal);
      const result = [];
      let lastDidNotMatchSelf = null;
      const appendElement = (element) => {
        if (kind === "lax" && lastDidNotMatchSelf && lastDidNotMatchSelf.contains(element))
          return false;
        const matches = elementMatchesText(this._evaluator._cacheText, element, matcher);
        if (matches === "none")
          lastDidNotMatchSelf = element;
        if (matches === "self" || matches === "selfAndChildren" && kind === "strict" && !this._internal)
          result.push(element);
      };
      if (root.nodeType === Node.ELEMENT_NODE)
        appendElement(root);
      const elements = this._evaluator._queryCSS({ scope: root, pierceShadow: this._shadow }, "*");
      for (const element of elements)
        appendElement(element);
      return result;
    } finally {
      this._evaluator.end();
    }
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

// packages/isomorphic/stringUtils.ts (Playwright v1.64.0)
function quoteCSSAttributeValue(text) {
  return `"${text.replace(/["\\]/g, (char) => "\\" + char)}"`;
}
function escapeRegExp(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
// Unlike Playwright's version, this reads escapes the same way the selector
// parser in common/selectors.go does, so adjacent quotes such as ["'] work.
function escapeRegexForSelector(re) {
  const s = String(re);
  if (re.unicode || re.unicodeSets)
    return s;
  let result = "";
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (c === "\\" && i + 1 < s.length)
      result += c + s[++i];
    else if (c === '"' || c === "'" || c === "`")
      result += "\\" + c;
    else if (c === ">" && s[i + 1] === ">")
      result += "\\>\\>", i++;
    else
      result += c;
  }
  return result;
}
function escapeForTextSelector(text, exact) {
  if (typeof text !== "string")
    return escapeRegexForSelector(text);
  return `${JSON.stringify(text)}${exact ? "s" : "i"}`;
}
function escapeForAttributeSelector(value, exact) {
  if (typeof value !== "string")
    return escapeRegexForSelector(value);
  return `"${value.replace(/\\/g, "\\\\").replace(/["]/g, '\\"')}"${exact ? "s" : "i"}`;
}
function escapeWithQuotes(text) {
  const stringified = JSON.stringify(text);
  const escapedText = stringified.substring(1, stringified.length - 1).replace(/\\"/g, '"');
  return "'" + escapedText.replace(/[']/g, "\\'") + "'";
}

// packages/injected/src/domUtils.ts (Playwright v1.64.0)
function isInsideScope(scope, element) {
  while (element) {
    if (scope.contains(element))
      return true;
    element = enclosingShadowHost(element);
  }
  return false;
}

// parseSelector parses a selector the same way NewSelector in common/selectors.go
// does, so generated selectors are checked exactly as k6 will later run them.
function parseSelector(selector) {
  const result = { selector, parts: [], capture: null };
  const appendPart = (start, end) => {
    const part = selector.substring(start, end).trim();
    if (!part)
      return;
    const eq = part.indexOf("=");
    let name = "css";
    let body = part;
    if (eq !== -1 && /^[a-zA-Z_0-9-+:*]+$/.test(part.substring(0, eq).trim())) {
      name = part.substring(0, eq).trim();
      body = part.substring(eq + 1);
    } else if (part.length > 1 && (part[0] === '"' || part[0] === "'") && part[part.length - 1] === part[0]) {
      name = "text";
    } else if (/^\(*\/\//.test(part) || part.startsWith("..")) {
      name = "xpath";
    }
    if (name.startsWith("*")) {
      if (result.capture !== null)
        throw new Error("only one of the selectors can capture using * modifier");
      name = name.substring(1);
      result.capture = result.parts.length;
    }
    result.parts.push({ name, body });
  };
  if (!selector.includes(">>")) {
    appendPart(0, selector.length);
    return result;
  }
  const ignoresQuotes = (start, end) => {
    const match = selector.substring(start, end).match(/^\s*text\s*=\s*(.*)$/);
    return !!match && match[1] !== "";
  };
  let start = 0;
  let index = 0;
  let quote = "";
  while (index < selector.length) {
    const c = selector[index];
    if (c === "\\" && index + 1 < selector.length) {
      index += 2;
    } else if (quote && c === quote) {
      quote = "";
      index++;
    } else if (!quote && (c === '"' || c === "'" || c === "`") && !ignoresQuotes(start, index)) {
      quote = c;
      index++;
    } else if (!quote && c === ">" && selector[index + 1] === ">") {
      appendPart(start, index);
      index += 2;
      start = index;
    } else {
      index++;
    }
  }
  appendPart(start, index);
  return result;
}

// packages/injected/src/selectorGenerator.ts (Playwright v1.64.0), adapted to
// k6: test ids use internal:attr, the role engine has no description
// attribute, and the accessible name is a plain string.
var kTestIdAttributeName = "data-testid";
var kTextScoreRange = 10;
var kExactPenalty = kTextScoreRange / 2;
var kTestIdScore = 1;
var kOtherTestIdScore = 2;
var kIframeByAttributeScore = 10;
var kBeginPenalizedScore = 50;
var kRoleWithNameScore = 100;
var kPlaceholderScore = 120;
var kLabelScore = 140;
var kAltTextScore = 160;
var kTextScore = 180;
var kTitleScore = 200;
var kTextScoreRegex = 250;
var kPlaceholderScoreExact = kPlaceholderScore + kExactPenalty;
var kLabelScoreExact = kLabelScore + kExactPenalty;
var kRoleWithNameScoreExact = kRoleWithNameScore + kExactPenalty;
var kAltTextScoreExact = kAltTextScore + kExactPenalty;
var kTextScoreExact = kTextScore + kExactPenalty;
var kTitleScoreExact = kTitleScore + kExactPenalty;
var kEndPenalizedScore = 300;
var kCSSIdScore = 500;
var kRoleWithoutNameScore = 510;
var kCSSInputTypeNameScore = 520;
var kCSSTagNameScore = 530;
var kNthScore = 10000;
var kCSSFallbackScore = 10000000;

function generateSelector(injectedScript, targetElement, options) {
  injectedScript._evaluator.begin();
  const cache = { allowText: new k6BrowserNative.Map(), disallowText: new k6BrowserNative.Map() };
  beginAriaCaches();
  try {
    if (!targetElement.matches("input,textarea,select") && !targetElement.isContentEditable) {
      const interactiveParent = closestCrossShadow(targetElement, "button,select,input,[role=button],[role=checkbox],[role=radio],a,[role=link]", options.root);
      if (interactiveParent && isVisible(interactiveParent))
        targetElement = interactiveParent;
    }
    const targetTokens = generateSelectorFor(cache, injectedScript, targetElement, options) || cssFallback(injectedScript, targetElement, options);
    const selector = joinTokens(targetTokens);
    return {
      selector,
      elements: injectedScript.querySelectorAll(parseSelector(selector), options.root || targetElement.ownerDocument),
    };
  } finally {
    endAriaCaches();
    injectedScript._evaluator.end();
  }
}

function generateSelectorFor(cache, injectedScript, targetElement, options) {
  if (options.root && !isInsideScope(options.root, targetElement))
    throw new Error(`Target element must belong to the root's subtree`);
  if (targetElement === options.root)
    return [{ engine: "css", selector: ":scope", score: 1 }];
  if (targetElement.ownerDocument.documentElement === targetElement)
    return [{ engine: "css", selector: "html", score: 1 }];

  let result = null;
  const updateResult = (candidate) => {
    if (!result || combineScores(candidate) < combineScores(result))
      result = candidate;
  };

  const candidates = [];
  for (const candidate of buildTextCandidates(injectedScript, targetElement, !options.isRecursive, options))
    candidates.push({ candidate, isTextCandidate: true });
  for (const token of buildNoTextCandidates(injectedScript, targetElement, options))
    candidates.push({ candidate: [token], isTextCandidate: false });
  candidates.sort((a, b) => combineScores(a.candidate) - combineScores(b.candidate));

  for (const { candidate, isTextCandidate } of candidates) {
    const elements = injectedScript.querySelectorAll(parseSelector(joinTokens(candidate)), options.root || targetElement.ownerDocument);
    if (!Array.isArray(elements) || !elements.includes(targetElement))
      continue;
    if (elements.length === 1) {
      // Candidates are sorted by score, so a unique match is the best one.
      updateResult(candidate);
      break;
    }
    const index = elements.indexOf(targetElement);
    if (index > 5)
      continue;
    updateResult([...candidate, { engine: "nth", selector: String(index), score: kNthScore }]);
    if (options.isRecursive)
      continue;
    // Try (best selector for a parent) >> (this candidate), two levels at most.
    for (let parent = parentElementOrShadowHost(targetElement); parent && parent !== options.root; parent = parentElementOrShadowHost(parent)) {
      const filtered = elements.filter((e) => isInsideScope(parent, e) && e !== parent);
      const newIndex = filtered.indexOf(targetElement);
      if (filtered.length > 5 || newIndex === -1 || newIndex === index && filtered.length > 1)
        continue;
      const inParent = filtered.length === 1 ? candidate : [...candidate, { engine: "nth", selector: String(newIndex), score: kNthScore }];
      const idealSelectorForParent = { engine: "", selector: "", score: 1 };
      if (result && combineScores([idealSelectorForParent, ...inParent]) >= combineScores(result))
        continue;
      // Don't use text in the parent when the target already uses text.
      const noText = !!options.noText || isTextCandidate;
      const cacheMap = noText ? cache.disallowText : cache.allowText;
      let parentTokens = cacheMap.get(parent);
      if (parentTokens === undefined) {
        parentTokens = generateSelectorFor(cache, injectedScript, parent, { ...options, isRecursive: true, noText }) || cssFallback(injectedScript, parent, options);
        cacheMap.set(parent, parentTokens);
      }
      if (!parentTokens)
        continue;
      updateResult([...parentTokens, ...inParent]);
    }
  }
  return result;
}

function buildNoTextCandidates(injectedScript, element, options) {
  const candidates = [];
  for (const attr of ["data-test-id", "data-test"]) {
    if (element.getAttribute(attr))
      candidates.push({ engine: "css", selector: `[${attr}=${quoteCSSAttributeValue(element.getAttribute(attr))}]`, score: kOtherTestIdScore });
  }
  const idAttr = element.getAttribute("id");
  if (idAttr && !isGuidLike(idAttr))
    candidates.push({ engine: "css", selector: makeSelectorForId(idAttr), score: kCSSIdScore });
  candidates.push({ engine: "css", selector: escapeNodeName(element), score: kCSSTagNameScore });

  if (element.nodeName === "IFRAME" || element.nodeName === "FRAME") {
    // Unlike Playwright, prefer the title to the name, and skip names that look
    // generated, such as "__privateFrame5": they can change between loads.
    const title = element.getAttribute("title");
    if (title)
      candidates.push({ engine: "css", selector: `${escapeNodeName(element)}[title=${quoteCSSAttributeValue(title)}]`, score: kIframeByAttributeScore });
    const name = element.getAttribute("name");
    if (name && !/\d$/.test(name) && !isGuidLike(name))
      candidates.push({ engine: "css", selector: `${escapeNodeName(element)}[name=${quoteCSSAttributeValue(name)}]`, score: kIframeByAttributeScore + 1 });
    if (element.getAttribute(kTestIdAttributeName))
      candidates.push({ engine: "css", selector: `[${kTestIdAttributeName}=${quoteCSSAttributeValue(element.getAttribute(kTestIdAttributeName))}]`, score: kTestIdScore });
    penalizeScoreForLength([candidates]);
    return candidates;
  }

  if (element.getAttribute(kTestIdAttributeName))
    candidates.push({ engine: "internal:attr", selector: `[${kTestIdAttributeName}=${escapeForAttributeSelector(element.getAttribute(kTestIdAttributeName), true)}]`, score: kTestIdScore });

  if ((element.nodeName === "INPUT" || element.nodeName === "TEXTAREA") && element.placeholder) {
    candidates.push({ engine: "internal:attr", selector: `[placeholder=${escapeForAttributeSelector(element.placeholder, true)}]`, score: kPlaceholderScoreExact });
    for (const alternative of suitableTextAlternatives(element.placeholder))
      candidates.push({ engine: "internal:attr", selector: `[placeholder=${escapeForAttributeSelector(alternative.text, false)}]`, score: kPlaceholderScore - alternative.scoreBonus });
  }

  for (const label of getElementLabels(injectedScript._evaluator._cacheText, element)) {
    const labelText = label.normalized;
    candidates.push({ engine: "internal:label", selector: escapeForTextSelector(labelText, true), score: kLabelScoreExact });
    for (const alternative of suitableTextAlternatives(labelText))
      candidates.push({ engine: "internal:label", selector: escapeForTextSelector(alternative.text, false), score: kLabelScore - alternative.scoreBonus });
  }

  const ariaRole = getAriaRole(element);
  if (ariaRole && !["none", "presentation"].includes(ariaRole))
    candidates.push({ engine: "internal:role", selector: ariaRole, score: kRoleWithoutNameScore });

  if (element.getAttribute("name") && ["BUTTON", "FORM", "FIELDSET", "FRAME", "IFRAME", "INPUT", "KEYGEN", "OBJECT", "OUTPUT", "SELECT", "TEXTAREA", "MAP", "META", "PARAM"].includes(element.nodeName))
    candidates.push({ engine: "css", selector: `${escapeNodeName(element)}[name=${quoteCSSAttributeValue(element.getAttribute("name"))}]`, score: kCSSInputTypeNameScore });

  if (["INPUT", "TEXTAREA"].includes(element.nodeName) && element.getAttribute("type") !== "hidden" && element.getAttribute("type"))
    candidates.push({ engine: "css", selector: `${escapeNodeName(element)}[type=${quoteCSSAttributeValue(element.getAttribute("type"))}]`, score: kCSSInputTypeNameScore });

  if (["INPUT", "TEXTAREA", "SELECT"].includes(element.nodeName) && element.getAttribute("type") !== "hidden")
    candidates.push({ engine: "css", selector: escapeNodeName(element), score: kCSSInputTypeNameScore + 1 });

  penalizeScoreForLength([candidates]);
  return candidates;
}

function buildTextCandidates(injectedScript, element, isTargetNode, options) {
  if (element.nodeName === "SELECT")
    return [];
  const candidates = [];

  if (!options.noText) {
    const title = element.getAttribute("title");
    if (title) {
      candidates.push([{ engine: "internal:attr", selector: `[title=${escapeForAttributeSelector(title, true)}]`, score: kTitleScoreExact }]);
      for (const alternative of suitableTextAlternatives(title))
        candidates.push([{ engine: "internal:attr", selector: `[title=${escapeForAttributeSelector(alternative.text, false)}]`, score: kTitleScore - alternative.scoreBonus }]);
    }
    const alt = element.getAttribute("alt");
    if (alt && ["APPLET", "AREA", "IMG", "INPUT"].includes(element.nodeName)) {
      candidates.push([{ engine: "internal:attr", selector: `[alt=${escapeForAttributeSelector(alt, true)}]`, score: kAltTextScoreExact }]);
      for (const alternative of suitableTextAlternatives(alt))
        candidates.push([{ engine: "internal:attr", selector: `[alt=${escapeForAttributeSelector(alternative.text, false)}]`, score: kAltTextScore - alternative.scoreBonus }]);
    }
  }

  const text = options.noText ? "" : elementText(injectedScript._evaluator._cacheText, element).normalized;
  const textAlternatives = text ? suitableTextAlternatives(text) : [];
  if (text) {
    if (isTargetNode) {
      if (text.length <= 80)
        candidates.push([{ engine: "internal:text", selector: escapeForTextSelector(text, true), score: kTextScoreExact }]);
      for (const alternative of textAlternatives)
        candidates.push([{ engine: "internal:text", selector: escapeForTextSelector(alternative.text, false), score: kTextScore - alternative.scoreBonus }]);
    }
    const cssToken = { engine: "css", selector: escapeNodeName(element), score: kCSSTagNameScore };
    for (const alternative of textAlternatives)
      candidates.push([cssToken, { engine: "internal:has-text", selector: escapeForTextSelector(alternative.text, false), score: kTextScore - alternative.scoreBonus }]);
    // Regexes are slow, so only use them for the target.
    if (isTargetNode && text.length <= 80) {
      const re = new RegExp("^" + escapeRegExp(text) + "$");
      candidates.push([cssToken, { engine: "internal:has-text", selector: escapeForTextSelector(re, false), score: kTextScoreRegex }]);
    }
  }

  const ariaRole = getAriaRole(element);
  if (ariaRole && !["none", "presentation"].includes(ariaRole)) {
    const ariaName = getElementAccessibleName(element, false);
    // \p{Co} is the Unicode "Private Use" category, often used by icon fonts.
    if (ariaName && !ariaName.match(/^\p{Co}+$/u)) {
      candidates.push([{ engine: "internal:role", selector: `${ariaRole}[name=${escapeForAttributeSelector(ariaName, true)}]`, score: kRoleWithNameScoreExact }]);
      for (const alternative of suitableTextAlternatives(ariaName))
        candidates.push([{ engine: "internal:role", selector: `${ariaRole}[name=${escapeForAttributeSelector(alternative.text, false)}]`, score: kRoleWithNameScore - alternative.scoreBonus }]);
    } else {
      const roleToken = { engine: "internal:role", selector: `${ariaRole}`, score: kRoleWithoutNameScore };
      for (const alternative of textAlternatives)
        candidates.push([roleToken, { engine: "internal:has-text", selector: escapeForTextSelector(alternative.text, false), score: kTextScore - alternative.scoreBonus }]);
      if (!options.noText && isTargetNode && text.length <= 80) {
        const re = new RegExp("^" + escapeRegExp(text) + "$");
        candidates.push([roleToken, { engine: "internal:has-text", selector: escapeForTextSelector(re, false), score: kTextScoreRegex }]);
      }
    }
  }

  penalizeScoreForLength(candidates);
  return candidates;
}

function makeSelectorForId(id) {
  return /^[a-zA-Z][a-zA-Z0-9\-\_]+$/.test(id) ? "#" + id : `[id=${quoteCSSAttributeValue(id)}]`;
}

function cssFallback(injectedScript, targetElement, options) {
  const root = options.root || targetElement.ownerDocument;
  const tokens = [];

  function uniqueCSSSelector(prefix) {
    const path = tokens.slice();
    if (prefix)
      path.unshift(prefix);
    const selector = path.join(" > ");
    const node = injectedScript.querySelector(parseSelector(selector), false, root);
    return node === targetElement ? selector : undefined;
  }

  function makeStrict(selector) {
    const token = { engine: "css", selector, score: kCSSFallbackScore };
    const elements = injectedScript.querySelectorAll(parseSelector(selector), root);
    if (elements.length === 1)
      return [token];
    return [token, { engine: "nth", selector: String(elements.indexOf(targetElement)), score: kNthScore }];
  }

  for (let element = targetElement; element && element !== root; element = parentElementOrShadowHost(element)) {
    let bestTokenForLevel = "";
    if (element.id) {
      const token = makeSelectorForId(element.id);
      const selector = uniqueCSSSelector(token);
      if (selector)
        return makeStrict(selector);
      bestTokenForLevel = token;
    }
    const parent = element.parentNode;
    const classes = [...element.classList].map(escapeClassName);
    for (let i = 0; i < classes.length; ++i) {
      const token = "." + classes.slice(0, i + 1).join(".");
      const selector = uniqueCSSSelector(token);
      if (selector)
        return makeStrict(selector);
      if (!bestTokenForLevel && parent && parent.querySelectorAll(token).length === 1)
        bestTokenForLevel = token;
    }
    if (parent) {
      const siblings = [...parent.children];
      const sameTagSiblings = siblings.filter((sibling) => sibling.nodeName === element.nodeName);
      const token = sameTagSiblings.indexOf(element) === 0 ? escapeNodeName(element) : `${escapeNodeName(element)}:nth-child(${1 + siblings.indexOf(element)})`;
      const selector = uniqueCSSSelector(token);
      if (selector)
        return makeStrict(selector);
      if (!bestTokenForLevel)
        bestTokenForLevel = token;
    } else if (!bestTokenForLevel) {
      bestTokenForLevel = escapeNodeName(element);
    }
    tokens.unshift(bestTokenForLevel);
  }
  return makeStrict(uniqueCSSSelector());
}

function penalizeScoreForLength(groups) {
  for (const group of groups) {
    for (const token of group) {
      if (token.score > kBeginPenalizedScore && token.score < kEndPenalizedScore)
        token.score += Math.min(kTextScoreRange, token.selector.length / 10 | 0);
    }
  }
}

function joinTokens(tokens) {
  const parts = [];
  let lastEngine = "";
  for (const { engine, selector } of tokens) {
    if (parts.length && (lastEngine !== "css" || engine !== "css" || selector.startsWith(":nth-match(")))
      parts.push(">>");
    lastEngine = engine;
    if (engine === "css")
      parts.push(selector);
    else
      parts.push(`${engine}=${selector}`);
  }
  return parts.join(" ");
}

function combineScores(tokens) {
  let score = 0;
  for (let i = 0; i < tokens.length; i++)
    score += tokens[i].score * (tokens.length - i);
  return score;
}

function isGuidLike(id) {
  let lastCharacterType;
  let transitionCount = 0;
  for (let i = 0; i < id.length; ++i) {
    const c = id[i];
    let characterType;
    if (c === "-" || c === "_")
      continue;
    if (c >= "a" && c <= "z")
      characterType = "lower";
    else if (c >= "A" && c <= "Z")
      characterType = "upper";
    else if (c >= "0" && c <= "9")
      characterType = "digit";
    else
      characterType = "other";
    if (characterType === "lower" && lastCharacterType === "upper") {
      lastCharacterType = characterType;
      continue;
    }
    if (lastCharacterType && lastCharacterType !== characterType)
      ++transitionCount;
    lastCharacterType = characterType;
  }
  return transitionCount >= id.length / 4;
}

function trimWordBoundary(text, maxLength) {
  if (text.length <= maxLength)
    return text;
  text = text.substring(0, maxLength);
  const match = text.match(/^(.*)\b(.+?)$/);
  if (!match)
    return "";
  return match[1].trimEnd();
}

function suitableTextAlternatives(text) {
  let result = [];
  {
    const match = text.match(/^([\d.,]+)[^.,\w]/);
    const leadingNumberLength = match ? match[1].length : 0;
    if (leadingNumberLength) {
      const alt = trimWordBoundary(text.substring(leadingNumberLength).trimStart(), 80);
      result.push({ text: alt, scoreBonus: alt.length <= 30 ? 2 : 1 });
    }
  }
  {
    const match = text.match(/[^.,\w]([\d.,]+)$/);
    const trailingNumberLength = match ? match[1].length : 0;
    if (trailingNumberLength) {
      const alt = trimWordBoundary(text.substring(0, text.length - trailingNumberLength).trimEnd(), 80);
      result.push({ text: alt, scoreBonus: alt.length <= 30 ? 2 : 1 });
    }
  }
  if (text.length <= 30) {
    result.push({ text, scoreBonus: 0 });
  } else {
    result.push({ text: trimWordBoundary(text, 80), scoreBonus: 0 });
    result.push({ text: trimWordBoundary(text, 30), scoreBonus: 1 });
  }
  result = result.filter((r) => r.text);
  if (!result.length)
    result.push({ text: text.substring(0, 80), scoreBonus: 0 });
  return result;
}

function escapeNodeName(node) {
  return node.nodeName.toLocaleLowerCase().replace(/[:\.]/g, (char) => "\\" + char);
}

// Escapes class names for querySelectorAll, following the CSS.escape() rules.
function escapeClassName(className) {
  let result = "";
  for (let i = 0; i < className.length; i++)
    result += cssEscapeCharacter(className, i);
  return result;
}

function cssEscapeCharacter(s, i) {
  // https://drafts.csswg.org/cssom/#serialize-an-identifier
  const c = s.charCodeAt(i);
  if (c === 0)
    return "\uFFFD";
  if (c >= 1 && c <= 31 || c >= 48 && c <= 57 && (i === 0 || i === 1 && s.charCodeAt(0) === 45))
    return "\\" + c.toString(16) + " ";
  if (i === 0 && c === 45 && s.length === 1)
    return "\\" + s.charAt(i);
  if (c >= 128 || c === 45 || c === 95 || c >= 48 && c <= 57 || c >= 65 && c <= 90 || c >= 97 && c <= 122)
    return s.charAt(i);
  return "\\" + s.charAt(i);
}

// Which elements listSelectors returns: widgets for "interactive", and
// meaningful content for "content".
var kListInteractiveRoles = ["button", "checkbox", "combobox", "gridcell", "link", "listbox", "menuitem", "menuitemcheckbox", "menuitemradio", "option", "radio", "searchbox", "slider", "spinbutton", "switch", "tab", "textbox", "treeitem"];
var kListContentRoles = ["alert", "alertdialog", "cell", "dialog", "heading", "listitem", "paragraph", "status"];
var kListCheckedRoles = ["checkbox", "menuitemcheckbox", "menuitemradio", "radio", "switch"];
var kListTextLength = 200;

function listSelectorsKind(element) {
  if (!isVisible(element))
    return null;
  const role = getAriaRole(element) || "";
  const style = getElementComputedStyle(element);
  if (style && style.pointerEvents !== "none" && isListedInteractive(element, role, style))
    return "interactive";
  if (element.getAttribute(kTestIdAttributeName) || kListContentRoles.includes(role) || (role === "img" && !!getElementAccessibleName(element, false)))
    return "content";
  return null;
}

function isListedInteractive(element, role, style) {
  if (kListInteractiveRoles.includes(role) || element.matches("input,select,textarea,button,a[href],summary"))
    return true;
  // A label is listed through the control that it labels.
  if (element.nodeName === "LABEL")
    return false;
  const parent = parentElementOrShadowHost(element);
  // Only the root of an editable region, not every element inside it.
  if (element.isContentEditable)
    return !(parent && parent.isContentEditable);
  if (style.cursor !== "pointer")
    return false;
  // Children inherit the pointer cursor; only list the element that sets it.
  const parentStyle = parent && getElementComputedStyle(parent);
  return !parentStyle || parentStyle.cursor !== "pointer";
}

// selectorToLocatorCode turns a generated selector into k6 locator code, such
// as page.getByRole('button', { name: 'Sign in' }). If any part has no k6
// equivalent, the whole selector goes into page.locator().
function selectorToLocatorCode(selector) {
  const fallback = `page.locator(${escapeWithQuotes(selector)})`;
  let code = "page";
  let last;
  // After entering an iframe there's a FrameLocator, which only has getBy*,
  // locator() and frameLocator().
  let inFrame = false;
  try {
    for (const part of parseSelector(selector).parts) {
      if (part.name === "internal:control" && part.body === "enter-frame") {
        if (!last || inFrame)
          return fallback;
        if (last.part.name === "css")
          code = code.substring(0, code.length - last.code.length) + `.frameLocator(${escapeWithQuotes(last.part.body)})`;
        else
          code += ".contentFrame()";
        inFrame = true;
        continue;
      }
      if (inFrame && (part.name === "nth" || part.name === "internal:has-text"))
        return fallback;
      const next = locatorCodeForPart(part);
      if (next === undefined)
        return fallback;
      code += next;
      last = { part, code: next };
      inFrame = false;
    }
  } catch (e) {
    return fallback;
  }
  return code;
}

function locatorCodeForPart({ name, body }) {
  if (name === "css")
    return `.locator(${escapeWithQuotes(body)})`;
  if (name === "nth")
    return body === "0" ? ".first()" : body === "-1" ? ".last()" : `.nth(${body})`;
  if (name === "internal:role") {
    const { name: role, attributes } = parseAttributeSelector(body, true);
    if (!attributes.length)
      return `.getByRole(${escapeWithQuotes(role)})`;
    const [attr] = attributes;
    if (attributes.length !== 1 || attr.name !== "name" || typeof attr.value !== "string")
      return undefined;
    return `.getByRole(${escapeWithQuotes(role)}, { name: ${escapeWithQuotes(attr.value)}${attr.caseSensitive ? ", exact: true" : ""} })`;
  }
  if (name === "internal:attr") {
    const { name: tag, attributes } = parseAttributeSelector(body, true);
    const methods = { "data-testid": "getByTestId", placeholder: "getByPlaceholder", alt: "getByAltText", title: "getByTitle" };
    const method = attributes.length === 1 && methods[attributes[0].name];
    if (tag || !method || typeof attributes[0].value !== "string")
      return undefined;
    const { value, caseSensitive } = attributes[0];
    if (method === "getByTestId")
      return caseSensitive ? `.getByTestId(${escapeWithQuotes(value)})` : undefined;
    return `.${method}(${escapeWithQuotes(value)}${caseSensitive ? ", { exact: true }" : ""})`;
  }
  const text = textSelectorToCode(body);
  if (text && (name === "internal:text" || name === "internal:label"))
    return `.${name === "internal:text" ? "getByText" : "getByLabel"}(${text.arg}${text.exact ? ", { exact: true }" : ""})`;
  // k6's hasText is always a case-insensitive substring or regex match.
  if (text && !text.exact && name === "internal:has-text")
    return `.filter({ hasText: ${text.arg} })`;
  return undefined;
}

function textSelectorToCode(body) {
  if (body[0] === "/" && body.lastIndexOf("/") > 0)
    return { arg: body, exact: false };
  if (body.length > 2 && body[0] === '"' && (body.endsWith('"i') || body.endsWith('"s')))
    return { arg: escapeWithQuotes(JSON.parse(body.substring(0, body.length - 1))), exact: body.endsWith("s") };
  return undefined;
}

class InjectedScript {
  constructor() {
    this._replaceRafWithTimeout = false;
    this._stableRafCount = 10;
    this._evaluator = new SelectorEvaluatorImpl();
    this._queryEngines = {
      css: new CSSQueryEngine(),
      text: new TextEngine(this._evaluator, true, false),
      xpath: new XPathQueryEngine(),
      'internal:role': createRoleEngine(true),
      'internal:attr': new AttributeEngine(this._evaluator),
      'internal:label': new LabelEngine(this._evaluator),
      'internal:text': new TextEngine(this._evaluator, true, true),
      'internal:has-text': this._createInternalHasTextEngine(),
      'internal:has-not-text': this._createInternalHasNotTextEngine(),
    };
  }

  _queryEngineAll(part, root) {
    return this._queryEngines[part.name].queryAll(root, part.body);
  }

  _createInternalHasTextEngine() {
    return {
      queryAll: (root, selector) => {
        if (root.nodeType !== 1 /* Node.ELEMENT_NODE */)
          return [];
        const element = root;
        const text = elementText(this._evaluator._cacheText, element);
        const { matcher } = createTextMatcher(selector, true);
        return matcher(text) ? [element] : [];
      }
    };
  }

  _createInternalHasNotTextEngine() {
    return {
      queryAll: (root, selector) => {
        if (root.nodeType !== 1 /* Node.ELEMENT_NODE */)
          return [];
        const element = root;
        const text = elementText(this._evaluator._cacheText, element);
        const { matcher } = createTextMatcher(selector, true);
        return matcher(text) ? [] : [element];
      }
    };
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
        const nth = parseInt(part.body, 10);
        const set = new k6BrowserNative.Set();
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
      const visible = part.body === "true";
      const filtered = roots.filter((match) => visible === isVisible(match.element));
      return this._querySelectorRecursively(
        filtered,
        selector,
        index + 1,
        queryCache
      );
    }

    const result = [];
    for (const root of roots) {
      const capture =
        index - 1 === selector.capture ? root.element : root.capture;

      // Run this part in the root and, unless the engine looks inside shadow
      // roots itself, in each shadow root under it, so that it also matches
      // inside open shadow DOM. The next part then runs once on all the
      // matches.
      const scopes = kShadowPiercingEngines.includes(part.name) ? [root.element] : [root.element, ...this._shadowRootsUnder(root.element)];
      for (const scope of scopes) {
        // Do not query engine twice for the same element.
        let queryResults = queryCache.get(scope);
        if (!queryResults) {
          queryResults = [];
          queryCache.set(scope, queryResults);
        }
        let all = queryResults[index];
        if (!all) {
          all = this._queryEngineAll(selector.parts[index], scope);
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
      }
    }

    return this._querySelectorRecursively(
      result,
      selector,
      index + 1,
      queryCache
    );
  }

  // _shadowRootsUnder returns the open shadow roots under root, including
  // root's own and nested ones, in document order. They're cached for as
  // long as the evaluator's other caches.
  _shadowRootsUnder(root) {
    let shadowRoots = this._evaluator._cacheShadowRoots.get(root);
    if (shadowRoots)
      return shadowRoots;
    shadowRoots = [];
    this._evaluator._cacheShadowRoots.set(root, shadowRoots);
    const visit = (node) => {
      const hosts = node.shadowRoot ? [node] : [];
      for (const element of node.querySelectorAll("*")) {
        if (element.shadowRoot)
          hosts.push(element);
      }
      for (const host of hosts) {
        shadowRoots.push(host.shadowRoot);
        visit(host.shadowRoot);
      }
    };
    visit(root);
    return shadowRoots;
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
    const hitTargetDescription = this.previewNode(hitParents[0] || document.documentElement);
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

  fill(node, value="") {
    const element = this._retarget(node, "follow-label");
    if (!element) {
      return "error:notconnected";
    }
    if (element.nodeName.toLowerCase() === "input") {
      const input = element;
      const type = input.type.toLowerCase();
      const kDateTypes = new k6BrowserNative.Set([
        "date",
        "time",
        "datetime",
        "datetime-local",
        "month",
        "week",
      ]);
      const kTextInputTypes = new k6BrowserNative.Set([
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
      if (kDateTypes.has(type)) {
        input.focus();
        input.value = value;
        if (input.value !== value) {
          return "error:notvaliddate";
        }
        element.dispatchEvent(new Event("input", { bubbles: true }));
        element.dispatchEvent(new Event("change", { bubbles: true }));
        return "done"; // We have already changed the value, no need to input it.
      }
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

  // isInViewport reports whether the element intersects the viewport by at
  // least the given ratio. A ratio of 0 means any visible pixel counts. The
  // measurement is taken with an IntersectionObserver so that scrollable
  // containers, CSS transforms and clipping are accounted for, mirroring
  // Playwright's behaviour rather than naive bounding box math.
  async isInViewport(node, ratio) {
    const element = this._retarget(node, "no-follow-label");
    if (!element || !element.isConnected) {
      return "error:notconnected";
    }
    const visibleRatio = await this.viewportRatio(element);
    return visibleRatio > 0 && visibleRatio > ratio - 1e-9;
  }

  // viewportRatio resolves with the ratio of the element that intersects the
  // viewport, as reported by an IntersectionObserver.
  viewportRatio(element) {
    return new Promise((resolve) => {
      const observer = new IntersectionObserver((entries) => {
        resolve(entries[0].intersectionRatio);
        observer.disconnect();
      });
      observer.observe(element);
      // Firefox doesn't call the IntersectionObserver callback unless there
      // are rafs.
      requestAnimationFrame(() => {});
    });
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
    this._evaluator.begin();
    try {
      const result = this._querySelectorRecursively(
        [{ element: root, capture: undefined }],
        selector,
        0,
        new k6BrowserNative.Map()
      );
      if (strict && result.length > 1) {
        throw "error:strictmodeviolation";
      }
      if (result.length == 0) {
        return null;
      }
      return result[0].capture || result[0].element;
    } finally {
      this._evaluator.end();
    }
  }

  querySelectorAll(selector, root) {
    if (!root["querySelectorAll"]) {
      return "error:notqueryablenode";
    }
    this._evaluator.begin();
    try {
      const result = this._querySelectorRecursively(
        [{ element: root, capture: undefined }],
        selector,
        0,
        new k6BrowserNative.Map()
      );
      const set = new k6BrowserNative.Set();
      for (const r of result) {
        set.add(r.capture || r.element);
      }
      return [...set];
    } finally {
      this._evaluator.end();
    }
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

  waitForElementStates(node, states=[], timeout, ...args) {
    let lastRect = undefined;
    let counter = 0;
    let samePositionCounter = 0;

    const predicate = () => {
      if (states.includes("stable")) {
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
          // assume that the element is not attached.
          console.debug(`'${selector.selector}' did not match any elements with state '${state}'`);
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
          return !visible ? element || true : continuePolling;
      }
    };

    return this.waitForPredicateFunction(predicate, polling, timeout, ...args);
  }

  count(selector, root) {
    const elements = this.querySelectorAll(selector, root || document);
    return elements.length;
  }

  // frameSelector returns the selector of an iframe element, and the
  // frameLocator() code that enters the iframe.
  frameSelector(element, options = {}) {
    const { selector } = generateSelector(this, element, options);
    return { frameSelector: selector, frameLocator: `.frameLocator(${escapeWithQuotes(selector)})` };
  }

  // locatorCode returns the k6 locator code for a selector, such as
  // page.getByRole('dialog').
  locatorCode(selector) {
    return selectorToLocatorCode(selector);
  }

  // listSelectors returns an entry for each element under root of the given
  // kind ("interactive", "content" or "all"), in document order. A visible
  // iframe gets a frameSelector() marker instead, where its entries go. Under
  // an element, selectors are relative to that element.
  listSelectors(root, kind) {
    const options = root.nodeType === Node.DOCUMENT_NODE ? {} : { root };
    const elements = [];
    const collect = (node) => {
      for (const child of node.children) {
        elements.push(child);
        if (child.shadowRoot)
          collect(child.shadowRoot);
        collect(child);
      }
    };
    collect(root);

    const entries = [];
    const seen = new k6BrowserNative.Set();
    this._evaluator.begin();
    beginAriaCaches();
    try {
      for (const element of elements) {
        if ((element.nodeName === "IFRAME" || element.nodeName === "FRAME") && isVisible(element)) {
          entries.push(this.frameSelector(element, options));
          continue;
        }
        const elementKind = listSelectorsKind(element);
        if (!elementKind || kind !== "all" && kind !== elementKind)
          continue;
        // The generator may pick an ancestor, e.g. the button around a span.
        const { selector, elements: matches } = generateSelector(this, element, options);
        if (!Array.isArray(matches) || matches.length !== 1 || seen.has(matches[0]))
          continue;
        const target = matches[0];
        const targetKind = target === element ? elementKind : listSelectorsKind(target);
        if (!targetKind || kind !== "all" && kind !== targetKind)
          continue;
        seen.add(target);
        const role = getAriaRole(target) || "";
        const entry = { kind: targetKind, role, name: getElementAccessibleName(target, false) };
        // Without a name, the visible text is what tells the reader what the element is.
        if (!entry.name) {
          const text = elementText(this._evaluator._cacheText, target).normalized;
          if (text)
            entry.text = text.length <= kListTextLength ? text : (trimWordBoundary(text, kListTextLength) || text.substring(0, kListTextLength)) + "…";
        }
        if (kListCheckedRoles.includes(role))
          entry.checked = getAriaChecked(target);
        entry.locator = selectorToLocatorCode(selector);
        entry.selector = selector;
        entries.push(entry);
      }
    } finally {
      endAriaCaches();
      this._evaluator.end();
    }
    return entries;
  }
}
