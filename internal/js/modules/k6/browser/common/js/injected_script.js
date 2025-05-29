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

// packages/playwright-core/src/utils/isomorphic/stringUtils.ts
var normalizedWhitespaceCache;
function normalizeWhiteSpace(text) {
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
  return s.split("\xA0").map((chunk) => chunk.replace(/\r\n/g, "\
").replace(/[\u200b\u00ad]/g, "").replace(/\s\s*/g, " ")).join("\xA0").trim();
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
        visitedElements: new Set(),
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
var kAriaExpandedRoles = ["application", "button", "checkbox", "combobox", "gridcell", "link", "listbox", "menuitem", "row", "rowheader", "tab", "treeitem", "columnheader", "menuitemcheckbox", "menuitemradio", "rowheader", "switch"];
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
  cacheAccessibleName != null ? cacheAccessibleName : cacheAccessibleName = new Map();
  cacheAccessibleNameHidden != null ? cacheAccessibleNameHidden : cacheAccessibleNameHidden = new Map();
  cacheAccessibleDescription != null ? cacheAccessibleDescription : cacheAccessibleDescription = new Map();
  cacheAccessibleDescriptionHidden != null ? cacheAccessibleDescriptionHidden : cacheAccessibleDescriptionHidden = new Map();
  cacheAccessibleErrorMessage != null ? cacheAccessibleErrorMessage : cacheAccessibleErrorMessage = new Map();
  cacheIsHidden != null ? cacheIsHidden : cacheIsHidden = new Map();
  cachePseudoContentBefore != null ? cachePseudoContentBefore : cachePseudoContentBefore = new Map();
  cachePseudoContentAfter != null ? cachePseudoContentAfter : cachePseudoContentAfter = new Map();
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

// packages/playwright-core/src/utils/isomorphic/cssParser.ts
var InvalidSelectorError = class extends Error {
};

// packages/playwright-core/src/utils/isomorphic/selectorParser.ts
var customCSSNames = new Set(["not", "is", "where", "has", "scope", "light", "visible", "text", "text-matches", "text-is", "has-text", "above", "below", "right-of", "left-of", "near", "nth-match"]);

// packages/injected/src/selectorEvaluator.ts
var SelectorEvaluatorImpl = class {
  constructor() {
    this._retainCacheCounter = 0;
    this._cacheText = new Map();
    this._cacheQueryCSS = new Map();
    this._cacheMatches = new Map();
    this._cacheQuery = new Map();
    this._cacheMatchesSimple = new Map();
    this._cacheMatchesParents = new Map();
    this._cacheCallMatches = new Map();
    this._cacheCallQuery = new Map();
    this._cacheQuerySimple = new Map();
    this._engines = new Map();
    this._engines.set("not", notEngine);
    this._engines.set("is", isEngine);
    this._engines.set("where", isEngine);
    this._engines.set("has", hasEngine);
    this._engines.set("scope", scopeEngine);
    this._engines.set("light", lightEngine);
    this._engines.set("visible", visibleEngine);
    this._engines.set("text", textEngine);
    this._engines.set("text-is", textIsEngine);
    this._engines.set("text-matches", textMatchesEngine);
    this._engines.set("has-text", hasTextEngine);
    this._engines.set("right-of", createLayoutEngine("right-of"));
    this._engines.set("left-of", createLayoutEngine("left-of"));
    this._engines.set("above", createLayoutEngine("above"));
    this._engines.set("below", createLayoutEngine("below"));
    this._engines.set("near", createLayoutEngine("near"));
    this._engines.set("nth-match", nthMatchEngine);
    const allNames = [...this._engines.keys()];
    allNames.sort();
    const parserNames = [...customCSSNames];
    parserNames.sort();
    if (allNames.join("|") !== parserNames.join("|"))
      throw new Error(`Please keep customCSSNames in sync with evaluator engines: ${allNames.join("|")} vs ${parserNames.join("|")}`);
  }
  begin() {
    ++this._retainCacheCounter;
  }
  end() {
    --this._retainCacheCounter;
    if (!this._retainCacheCounter) {
      this._cacheQueryCSS.clear();
      this._cacheMatches.clear();
      this._cacheQuery.clear();
      this._cacheMatchesSimple.clear();
      this._cacheMatchesParents.clear();
      this._cacheCallMatches.clear();
      this._cacheCallQuery.clear();
      this._cacheQuerySimple.clear();
      this._cacheText.clear();
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
  _checkSelector(s) {
    const wellFormed = typeof s === "object" && s && (Array.isArray(s) || "simples" in s && s.simples.length);
    if (!wellFormed)
      throw new Error(`Malformed selector "${s}"`);
    return s;
  }
  matches(element, s, context) {
    const selector = this._checkSelector(s);
    this.begin();
    try {
      return this._cached(this._cacheMatches, element, [selector, context.scope, context.pierceShadow, context.originalScope], () => {
        if (Array.isArray(selector))
          return this._matchesEngine(isEngine, element, selector, context);
        if (this._hasScopeClause(selector))
          context = this._expandContextForScopeMatching(context);
        if (!this._matchesSimple(element, selector.simples[selector.simples.length - 1].selector, context))
          return false;
        return this._matchesParents(element, selector, selector.simples.length - 2, context);
      });
    } finally {
      this.end();
    }
  }
  query(context, s) {
    const selector = this._checkSelector(s);
    this.begin();
    try {
      return this._cached(this._cacheQuery, selector, [context.scope, context.pierceShadow, context.originalScope], () => {
        if (Array.isArray(selector))
          return this._queryEngine(isEngine, context, selector);
        if (this._hasScopeClause(selector))
          context = this._expandContextForScopeMatching(context);
        const previousScoreMap = this._scoreMap;
        this._scoreMap = new Map();
        let elements = this._querySimple(context, selector.simples[selector.simples.length - 1].selector);
        elements = elements.filter((element) => this._matchesParents(element, selector, selector.simples.length - 2, context));
        if (this._scoreMap.size) {
          elements.sort((a, b) => {
            const aScore = this._scoreMap.get(a);
            const bScore = this._scoreMap.get(b);
            if (aScore === bScore)
              return 0;
            if (aScore === void 0)
              return 1;
            if (bScore === void 0)
              return -1;
            return aScore - bScore;
          });
        }
        this._scoreMap = previousScoreMap;
        return elements;
      });
    } finally {
      this.end();
    }
  }
  _markScore(element, score) {
    if (this._scoreMap)
      this._scoreMap.set(element, score);
  }
  _hasScopeClause(selector) {
    return selector.simples.some((simple) => simple.selector.functions.some((f) => f.name === "scope"));
  }
  _expandContextForScopeMatching(context) {
    if (context.scope.nodeType !== 1)
      return context;
    const scope = parentElementOrShadowHost(context.scope);
    if (!scope)
      return context;
    return { ...context, scope, originalScope: context.originalScope || context.scope };
  }
  _matchesSimple(element, simple, context) {
    return this._cached(this._cacheMatchesSimple, element, [simple, context.scope, context.pierceShadow, context.originalScope], () => {
      if (element === context.scope)
        return false;
      if (simple.css && !this._matchesCSS(element, simple.css))
        return false;
      for (const func of simple.functions) {
        if (!this._matchesEngine(this._getEngine(func.name), element, func.args, context))
          return false;
      }
      return true;
    });
  }
  _querySimple(context, simple) {
    if (!simple.functions.length)
      return this._queryCSS(context, simple.css || "*");
    return this._cached(this._cacheQuerySimple, simple, [context.scope, context.pierceShadow, context.originalScope], () => {
      let css = simple.css;
      const funcs = simple.functions;
      if (css === "*" && funcs.length)
        css = void 0;
      let elements;
      let firstIndex = -1;
      if (css !== void 0) {
        elements = this._queryCSS(context, css);
      } else {
        firstIndex = funcs.findIndex((func) => this._getEngine(func.name).query !== void 0);
        if (firstIndex === -1)
          firstIndex = 0;
        elements = this._queryEngine(this._getEngine(funcs[firstIndex].name), context, funcs[firstIndex].args);
      }
      for (let i = 0; i < funcs.length; i++) {
        if (i === firstIndex)
          continue;
        const engine = this._getEngine(funcs[i].name);
        if (engine.matches !== void 0)
          elements = elements.filter((e) => this._matchesEngine(engine, e, funcs[i].args, context));
      }
      for (let i = 0; i < funcs.length; i++) {
        if (i === firstIndex)
          continue;
        const engine = this._getEngine(funcs[i].name);
        if (engine.matches === void 0)
          elements = elements.filter((e) => this._matchesEngine(engine, e, funcs[i].args, context));
      }
      return elements;
    });
  }
  _matchesParents(element, complex, index, context) {
    if (index < 0)
      return true;
    return this._cached(this._cacheMatchesParents, element, [complex, index, context.scope, context.pierceShadow, context.originalScope], () => {
      const { selector: simple, combinator } = complex.simples[index];
      if (combinator === ">") {
        const parent = parentElementOrShadowHostInContext(element, context);
        if (!parent || !this._matchesSimple(parent, simple, context))
          return false;
        return this._matchesParents(parent, complex, index - 1, context);
      }
      if (combinator === "+") {
        const previousSibling = previousSiblingInContext(element, context);
        if (!previousSibling || !this._matchesSimple(previousSibling, simple, context))
          return false;
        return this._matchesParents(previousSibling, complex, index - 1, context);
      }
      if (combinator === "") {
        let parent = parentElementOrShadowHostInContext(element, context);
        while (parent) {
          if (this._matchesSimple(parent, simple, context)) {
            if (this._matchesParents(parent, complex, index - 1, context))
              return true;
            if (complex.simples[index - 1].combinator === "")
              break;
          }
          parent = parentElementOrShadowHostInContext(parent, context);
        }
        return false;
      }
      if (combinator === "~") {
        let previousSibling = previousSiblingInContext(element, context);
        while (previousSibling) {
          if (this._matchesSimple(previousSibling, simple, context)) {
            if (this._matchesParents(previousSibling, complex, index - 1, context))
              return true;
            if (complex.simples[index - 1].combinator === "~")
              break;
          }
          previousSibling = previousSiblingInContext(previousSibling, context);
        }
        return false;
      }
      if (combinator === ">=") {
        let parent = element;
        while (parent) {
          if (this._matchesSimple(parent, simple, context)) {
            if (this._matchesParents(parent, complex, index - 1, context))
              return true;
            if (complex.simples[index - 1].combinator === "")
              break;
          }
          parent = parentElementOrShadowHostInContext(parent, context);
        }
        return false;
      }
      throw new Error(`Unsupported combinator "${combinator}"`);
    });
  }
  _matchesEngine(engine, element, args, context) {
    if (engine.matches)
      return this._callMatches(engine, element, args, context);
    if (engine.query)
      return this._callQuery(engine, args, context).includes(element);
    throw new Error(`Selector engine should implement "matches" or "query"`);
  }
  _queryEngine(engine, context, args) {
    if (engine.query)
      return this._callQuery(engine, args, context);
    if (engine.matches)
      return this._queryCSS(context, "*").filter((element) => this._callMatches(engine, element, args, context));
    throw new Error(`Selector engine should implement "matches" or "query"`);
  }
  _callMatches(engine, element, args, context) {
    return this._cached(this._cacheCallMatches, element, [engine, context.scope, context.pierceShadow, context.originalScope, ...args], () => {
      return engine.matches(element, args, context, this);
    });
  }
  _callQuery(engine, args, context) {
    return this._cached(this._cacheCallQuery, engine, [context.scope, context.pierceShadow, context.originalScope, ...args], () => {
      return engine.query(context, args, this);
    });
  }
  _matchesCSS(element, css) {
    return element.matches(css);
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
  _getEngine(name) {
    const engine = this._engines.get(name);
    if (!engine)
      throw new Error(`Unknown selector engine "${name}"`);
    return engine;
  }
};
var isEngine = {
  matches(element, args, context, evaluator) {
    if (args.length === 0)
      throw new Error(`"is" engine expects non-empty selector list`);
    return args.some((selector) => evaluator.matches(element, selector, context));
  },
  query(context, args, evaluator) {
    if (args.length === 0)
      throw new Error(`"is" engine expects non-empty selector list`);
    let elements = [];
    for (const arg of args)
      elements = elements.concat(evaluator.query(context, arg));
    return args.length === 1 ? elements : sortInDOMOrder(elements);
  }
};
var hasEngine = {
  matches(element, args, context, evaluator) {
    if (args.length === 0)
      throw new Error(`"has" engine expects non-empty selector list`);
    return evaluator.query({ ...context, scope: element }, args).length > 0;
  }
  // TODO: we can implement efficient "query" by matching "args" and returning
  // all parents/descendants, just have to be careful with the ":scope" matching.
};
var scopeEngine = {
  matches(element, args, context, evaluator) {
    if (args.length !== 0)
      throw new Error(`"scope" engine expects no arguments`);
    const actualScope = context.originalScope || context.scope;
    if (actualScope.nodeType === 9)
      return element === actualScope.documentElement;
    return element === actualScope;
  },
  query(context, args, evaluator) {
    if (args.length !== 0)
      throw new Error(`"scope" engine expects no arguments`);
    const actualScope = context.originalScope || context.scope;
    if (actualScope.nodeType === 9) {
      const root = actualScope.documentElement;
      return root ? [root] : [];
    }
    if (actualScope.nodeType === 1)
      return [actualScope];
    return [];
  }
};
var notEngine = {
  matches(element, args, context, evaluator) {
    if (args.length === 0)
      throw new Error(`"not" engine expects non-empty selector list`);
    return !evaluator.matches(element, args, context);
  }
};
var lightEngine = {
  query(context, args, evaluator) {
    return evaluator.query({ ...context, pierceShadow: false }, args);
  },
  matches(element, args, context, evaluator) {
    return evaluator.matches(element, args, { ...context, pierceShadow: false });
  }
};
var visibleEngine = {
  matches(element, args, context, evaluator) {
    if (args.length)
      throw new Error(`"visible" engine expects no arguments`);
    return isElementVisible(element);
  }
};
var textEngine = {
  matches(element, args, context, evaluator) {
    if (args.length !== 1 || typeof args[0] !== "string")
      throw new Error(`"text" engine expects a single string`);
    const text = normalizeWhiteSpace(args[0]).toLowerCase();
    const matcher = (elementText2) => elementText2.normalized.toLowerCase().includes(text);
    return elementMatchesText(evaluator._cacheText, element, matcher) === "self";
  }
};
var textIsEngine = {
  matches(element, args, context, evaluator) {
    if (args.length !== 1 || typeof args[0] !== "string")
      throw new Error(`"text-is" engine expects a single string`);
    const text = normalizeWhiteSpace(args[0]);
    const matcher = (elementText2) => {
      if (!text && !elementText2.immediate.length)
        return true;
      return elementText2.immediate.some((s) => normalizeWhiteSpace(s) === text);
    };
    return elementMatchesText(evaluator._cacheText, element, matcher) !== "none";
  }
};
var textMatchesEngine = {
  matches(element, args, context, evaluator) {
    if (args.length === 0 || typeof args[0] !== "string" || args.length > 2 || args.length === 2 && typeof args[1] !== "string")
      throw new Error(`"text-matches" engine expects a regexp body and optional regexp flags`);
    const re = new RegExp(args[0], args.length === 2 ? args[1] : void 0);
    const matcher = (elementText2) => re.test(elementText2.full);
    return elementMatchesText(evaluator._cacheText, element, matcher) === "self";
  }
};
var hasTextEngine = {
  matches(element, args, context, evaluator) {
    if (args.length !== 1 || typeof args[0] !== "string")
      throw new Error(`"has-text" engine expects a single string`);
    if (shouldSkipForTextMatching(element))
      return false;
    const text = normalizeWhiteSpace(args[0]).toLowerCase();
    const matcher = (elementText2) => elementText2.normalized.toLowerCase().includes(text);
    return matcher(elementText(evaluator._cacheText, element));
  }
};
function createLayoutEngine(name) {
  return {
    matches(element, args, context, evaluator) {
      const maxDistance = args.length && typeof args[args.length - 1] === "number" ? args[args.length - 1] : void 0;
      const queryArgs = maxDistance === void 0 ? args : args.slice(0, args.length - 1);
      if (args.length < 1 + (maxDistance === void 0 ? 0 : 1))
        throw new Error(`"${name}" engine expects a selector list and optional maximum distance in pixels`);
      const inner = evaluator.query(context, queryArgs);
      const score = layoutSelectorScore(name, element, inner, maxDistance);
      if (score === void 0)
        return false;
      evaluator._markScore(element, score);
      return true;
    }
  };
}
var nthMatchEngine = {
  query(context, args, evaluator) {
    let index = args[args.length - 1];
    if (args.length < 2)
      throw new Error(`"nth-match" engine expects non-empty selector list and an index argument`);
    if (typeof index !== "number" || index < 1)
      throw new Error(`"nth-match" engine expects a one-based index as the last argument`);
    const elements = isEngine.query(context, args.slice(0, args.length - 1), evaluator);
    index--;
    return index < elements.length ? [elements[index]] : [];
  }
};
function parentElementOrShadowHostInContext(element, context) {
  if (element === context.scope)
    return;
  if (!context.pierceShadow)
    return element.parentElement || void 0;
  return parentElementOrShadowHost(element);
}
function previousSiblingInContext(element, context) {
  if (element === context.scope)
    return;
  return element.previousElementSibling || void 0;
}
function sortInDOMOrder(elements) {
  const elementToEntry = new Map();
  const roots = [];
  const result = [];
  function append(element) {
    let entry = elementToEntry.get(element);
    if (entry)
      return entry;
    const parent = parentElementOrShadowHost(element);
    if (parent) {
      const parentEntry = append(parent);
      parentEntry.children.push(element);
    } else {
      roots.push(element);
    }
    entry = { children: [], taken: false };
    elementToEntry.set(element, entry);
    return entry;
  }
  for (const e of elements)
    append(e).taken = true;
  function visit(element) {
    const entry = elementToEntry.get(element);
    if (entry.taken)
      result.push(element);
    if (entry.children.length > 1) {
      const set = new Set(entry.children);
      entry.children = [];
      let child = element.firstElementChild;
      while (child && entry.children.length < set.size) {
        if (set.has(child))
          entry.children.push(child);
        child = child.nextElementSibling;
      }
      child = element.shadowRoot ? element.shadowRoot.firstElementChild : null;
      while (child && entry.children.length < set.size) {
        if (set.has(child))
          entry.children.push(child);
        child = child.nextElementSibling;
      }
    }
    entry.children.forEach(visit);
  }
  roots.forEach(visit);
  return result;
}

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

class LabelEngine {
  constructor() {
    this._evaluator = new SelectorEvaluatorImpl();
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

class AttributeEngine {
  constructor() {
    this._evaluator = new SelectorEvaluatorImpl();
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

class TestIDEngine {
  constructor() {
    this._evaluator = new SelectorEvaluatorImpl();
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
  }
}

class TextEngine {
  constructor(shadow, internal) {
    this._evaluator = new SelectorEvaluatorImpl();
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
      'internal:role': createRoleEngine(true),
      'internal:attr': new AttributeEngine(),
      'internal:label': new LabelEngine(),
      'internal:testid': new TestIDEngine(),
      'internal:text': new TextEngine(true, true),
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
      new k6BrowserNative.Map()
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
      new k6BrowserNative.Map()
    );
    const set = new k6BrowserNative.Set();
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

  waitForElementStates(node, states=[], timeout, ...args) {
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

  count(selector, root) {
    const elements = this.querySelectorAll(selector, root || document);
    return elements.length;
  }
}
