import DOMPurify, { type Config } from "dompurify";
import {
  isExternalLink,
  isWorkspaceReference,
  parseChatCodeLink,
  sessionLinkPath,
} from "./markdown-link";

const forbiddenTags = [
  "image",
  "svg",
  "math",
  "use",
  "foreignobject",
  "picture",
  "video",
  "audio",
  "source",
  "track",
  "iframe",
  "object",
  "embed",
  "form",
  "input",
  "button",
  "textarea",
  "select",
  "option",
  "link",
  "meta",
  "base",
  "style",
  // Image maps would keep a raw relative `href` on `<area>`.
  "map",
  "area",
];
// Authored `id` / `name` could clobber application elements.
const forbiddenAttributes = ["style", "srcset", "background", "id", "name"];

// Each variant owns its DOMPurify instance: hooks are instance-wide state,
// so the file preview's link and image handling never reaches chat messages.
let allowMessageCodeLinks = false;
const pendingCodeHrefs = new WeakMap<Element, string>();
const messagePurifier = DOMPurify(window);
messagePurifier.addHook("uponSanitizeAttribute", (node, data) => {
  if (data.attrName.startsWith("data-md-")) {
    data.keepAttr = false;
    return;
  }
  // Capture valid code-location targets before DOMPurify's built-in URI
  // filter strips non-HTTP schemes (`file:///...`) or colon line suffixes
  // (`Makefile:15`), and never leave a non-external `href` on `<a>`.
  if (node.tagName === "A" && data.attrName === "href") {
    const sessionPath = sessionLinkPath(data.attrValue);
    if (sessionPath) {
      data.attrValue = sessionPath;
      return;
    }
    if (!isExternalLink(data.attrValue)) {
      if (allowMessageCodeLinks && parseChatCodeLink(data.attrValue))
        pendingCodeHrefs.set(node, data.attrValue);
      data.keepAttr = false;
    }
  }
});
messagePurifier.addHook("afterSanitizeAttributes", (node) => {
  if (node.nodeType !== Node.ELEMENT_NODE) return;
  const element = node as Element;
  if (element.tagName !== "A") return;
  const href = element.getAttribute("href");
  if (href !== null && sessionLinkPath(href)) {
    element.removeAttribute("target");
    element.removeAttribute("rel");
    return;
  }
  if (href !== null && isExternalLink(href)) {
    element.setAttribute("target", "_blank");
    element.setAttribute("rel", "noopener noreferrer");
    return;
  }
  element.removeAttribute("href");
  const codeHref = pendingCodeHrefs.get(element);
  if (!codeHref) return;
  pendingCodeHrefs.delete(element);
  element.setAttribute("data-md-href", codeHref);
  element.setAttribute("role", "link");
  element.setAttribute("tabindex", "0");
});
const messageConfig: Config = {
  FORBID_TAGS: ["img", ...forbiddenTags],
  FORBID_ATTR: forbiddenAttributes,
  ADD_ATTR: ["target", "rel"],
};

export function sanitizeMessageHtml(html: string, codeLinks: boolean) {
  allowMessageCodeLinks = codeLinks;
  try {
    return messagePurifier.sanitize(html, messageConfig);
  } finally {
    allowMessageCodeLinks = false;
  }
}

const documentPurifier = DOMPurify(window);
const pendingDocumentHrefs = new WeakMap<Element, string>();
// `data-md-*` attributes carry the preview's own resolved references; an
// author must not be able to forge them.
documentPurifier.addHook("uponSanitizeAttribute", (node, data) => {
  if (data.attrName.startsWith("data-md-")) {
    data.keepAttr = false;
    return;
  }
  if (
    node.tagName === "A" &&
    data.attrName === "href" &&
    /^file:/i.test(data.attrValue.trim()) &&
    parseChatCodeLink(data.attrValue)
  ) {
    pendingDocumentHrefs.set(node, data.attrValue);
    data.keepAttr = false;
  }
});
// Workspace addresses leave `href` / `src` while the markup is still inert,
// so the browser never requests them as application routes.
documentPurifier.addHook("afterSanitizeAttributes", (node) => {
  if (node.nodeType !== Node.ELEMENT_NODE) return;
  const element = node as Element;
  if (element.tagName === "A") {
    const href = element.getAttribute("href");
    if (href !== null && isExternalLink(href)) {
      element.setAttribute("target", "_blank");
      element.setAttribute("rel", "noopener noreferrer");
      return;
    }
    element.removeAttribute("href");
    const fileHref = pendingDocumentHrefs.get(element);
    if (fileHref) {
      pendingDocumentHrefs.delete(element);
      element.setAttribute("data-md-href", fileHref);
      element.setAttribute("role", "link");
      element.setAttribute("tabindex", "0");
      return;
    }
    if (href === null || !isWorkspaceReference(href)) return;
    element.setAttribute("data-md-href", href);
    element.setAttribute("role", "link");
    element.setAttribute("tabindex", "0");
  } else if (element.tagName === "IMG") {
    const src = element.getAttribute("src") ?? "";
    element.removeAttribute("src");
    element.setAttribute(
      isWorkspaceReference(src) ? "data-md-src" : "data-md-external",
      src,
    );
  }
});
const documentConfig = {
  FORBID_TAGS: forbiddenTags,
  FORBID_ATTR: forbiddenAttributes,
  ADD_ATTR: ["target", "rel"],
  RETURN_DOM_FRAGMENT: true,
} as const satisfies Config;

export function sanitizeDocumentFragment(html: string) {
  return documentPurifier.sanitize(html, documentConfig);
}
