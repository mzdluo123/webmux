import { SvelteMap } from "svelte/reactivity";
import { xrPresentation } from "./xr-presentation.svelte";
import {
  spatialCandidate,
  type SpatialDirection,
} from "../../platform/spatial-navigation";
import { tvMode } from "./tv-mode.svelte";
import { tvInput } from "./tv-input.svelte";

const controls =
  'button, a[href], input, textarea, select, [tabindex="0"], [contenteditable="true"]';
const editable =
  'input, textarea, select, [contenteditable="true"], .cm-editor, .xterm';
const textFields =
  'input:not([type="checkbox"]):not([type="radio"]):not([type="range"]):not([type="color"]):not([type="file"]), textarea, [contenteditable="true"]';
const roving =
  '[role="tree"], [role="menu"], [role="menubar"], [role="listbox"], [role="tablist"], [role="radiogroup"], [role="slider"], [role="combobox"], [data-tv-native-keys]';
// Inline command lists are widgets, not modal navigation boundaries.
const overlays =
  '[role="dialog"], [role="alertdialog"], [role="menu"], [role="listbox"][data-state="open"], [data-slot="popover-content"]';

function visible(element: HTMLElement) {
  return (
    element.getClientRects().length > 0 &&
    !element.closest('[inert], [hidden], [aria-hidden="true"]') &&
    getComputedStyle(element).visibility !== "hidden"
  );
}

function candidates(root: ParentNode) {
  return Array.from(root.querySelectorAll<HTMLElement>(controls)).filter(
    (element) =>
      visible(element) &&
      !element.matches(':disabled, [aria-disabled="true"]') &&
      // Drag splitters consume arrows for resizing, not TV activation.
      !element.matches('[role="separator"]') &&
      !element.closest(".xterm") &&
      element.tabIndex >= 0,
  );
}

function focus(element: HTMLElement) {
  element.focus({ preventScroll: true });
  element.scrollIntoView({ block: "nearest", inline: "nearest" });
}

/** Runs once at the application root; ordinary keyboard mode is untouched. */
export function useTvNavigation(back: () => boolean) {
  $effect(() => {
    if (!tvMode.enabled) return;
    const root = document.documentElement;
    root.dataset.uiMode = "tv";
    let last: HTMLElement | null = null;
    let redirecting = false;
    let editing: HTMLElement | null = null;
    let tabbing = false;
    function textField(target: EventTarget | null) {
      if (!(target instanceof Element) || target.closest(".xterm, .cm-editor"))
        return null;
      const field = target.closest<HTMLElement>(textFields);
      return field?.closest(roving) ? null : field;
    }
    function setTextTarget(target: HTMLElement | null, edit = false) {
      if (editing !== target) editing = null;
      if (edit) editing = target;
      tvInput.message = !target
        ? ""
        : editing !== target
          ? "文本输入 · 确认开始编辑"
          : target instanceof HTMLInputElement
            ? "正在编辑 · 确认完成"
            : "正在编辑 · 确认完成，Shift+Enter 换行";
    }
    function beginText(event: Event) {
      const target = textField(event.target);
      if (target) setTextTarget(target, true);
    }
    const synthetic = new WeakSet<Event>();
    const enhanced = new SvelteMap<HTMLElement, boolean>();
    function enhanceScroll() {
      for (const element of document.querySelectorAll<HTMLElement>(
        'main, pre, [role="log"], [role="list"], [class*="overflow"], [data-slot="dialog-content"], [data-slot="drawer-content"]',
      )) {
        if (enhanced.has(element) || element.closest(".xterm, .cm-editor"))
          continue;
        if (element.scrollHeight <= element.clientHeight + 1) continue;
        if (!/auto|scroll/.test(getComputedStyle(element).overflowY)) continue;
        const addedTabIndex = !element.hasAttribute("tabindex");
        if (addedTabIndex) element.tabIndex = 0;
        element.dataset.tvScroll = "";
        enhanced.set(element, addedTabIndex);
      }
    }
    const observer = new MutationObserver(() => {
      enhanceScroll();
      restore();
    });
    observer.observe(document.body, { childList: true, subtree: true });
    enhanceScroll();
    function focusChanged(event: FocusEvent) {
      const target = event.target;
      if (target instanceof HTMLElement && visible(target)) last = target;
      setTextTarget(textField(target), tabbing);
      tabbing = false;
    }
    function layer(phase: "capture" | "panel") {
      const event = new CustomEvent("webmux:tv-back", {
        cancelable: true,
        detail: phase,
      });
      window.dispatchEvent(event);
      return event.defaultPrevented;
    }
    function keydown(event: KeyboardEvent) {
      if (xrPresentation.active) return;
      if (
        event.defaultPrevented ||
        synthetic.has(event) ||
        event.isComposing ||
        event.altKey ||
        event.ctrlKey ||
        event.metaKey ||
        event.shiftKey
      )
        return;
      const active =
        document.activeElement instanceof HTMLElement
          ? document.activeElement
          : null;
      const open = Array.from(
        document.querySelectorAll<HTMLElement>(overlays),
      ).filter(visible);
      const top = active?.closest<HTMLElement>(overlays) ?? open.at(-1);
      const isBack = ["Escape", "BrowserBack", "GoBack", "Back"].includes(
        event.key,
      );
      if (isBack) {
        // Bits UI owns dismissal/focus restoration and its nested layer stack.
        if (top && !top.matches('[data-slot="drawer-content"]')) {
          if (event.key === "Escape") return;
          event.preventDefault();
          event.stopImmediatePropagation();
          const escape = new KeyboardEvent("keydown", {
            key: "Escape",
            code: "Escape",
            bubbles: true,
            cancelable: true,
          });
          synthetic.add(escape);
          (active ?? top).dispatchEvent(escape);
          return;
        }
        let handled = layer("capture");
        if (!handled && active?.matches(editable)) {
          active.blur();
          const scope = active.closest(
            'form, [role="dialog"], main, .session-terminal',
          );
          const target =
            scope &&
            candidates(scope).find((element) =>
              element.matches("button, a[href]"),
            );
          if (target) focus(target);
          handled = true;
        }
        if (!handled) handled = layer("panel");
        if (!handled) handled = back();
        if (handled) {
          event.preventDefault();
          event.stopImmediatePropagation();
        }
        return;
      }
      const field = textField(active);
      if (active?.closest(editable) && (!field || editing === field)) return;
      const widget = active?.closest(roving);
      // Handled keys already returned above. Tabs/radios can leave on their
      // unused axis; their own roving/selection keys always run first.
      if (
        widget &&
        !field &&
        !widget.matches(
          '[role="tablist"], [role="radiogroup"], [role="combobox"][aria-expanded="false"]',
        )
      )
        return;
      if (
        event.key === "Enter" &&
        active?.matches('[role="button"]') &&
        !active.matches("button, a")
      ) {
        event.preventDefault();
        active.click();
        return;
      }
      if (
        !["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"].includes(event.key)
      )
        return;
      enhanceScroll();
      if (
        active?.hasAttribute("data-tv-scroll") &&
        (event.key === "ArrowDown" || event.key === "ArrowUp")
      ) {
        const before = active.scrollTop;
        active.scrollTop +=
          (event.key === "ArrowDown" ? 1 : -1) * active.clientHeight * 0.7;
        if (active.scrollTop !== before) {
          event.preventDefault();
          return;
        }
      }
      const targets = candidates(top ?? document.body).filter(
        (target) => target !== active,
      );
      const next =
        active && visible(active)
          ? spatialCandidate(
              active.getBoundingClientRect(),
              targets.map((target) => ({
                target,
                rect: target.getBoundingClientRect(),
              })),
              event.key as SpatialDirection,
            )
          : targets[0];
      if (!next) return;
      event.preventDefault();
      focus(next);
    }
    function restore() {
      if (
        xrPresentation.active ||
        redirecting ||
        document.activeElement !== document.body
      )
        return;
      redirecting = true;
      const target =
        last?.isConnected && visible(last)
          ? last
          : candidates(document.body)[0];
      if (target) focus(target);
      redirecting = false;
    }
    function captureBack(event: KeyboardEvent) {
      if (xrPresentation.active) return;
      if (event.defaultPrevented || event.isComposing) return;
      if (
        event.key === "Tab" &&
        !event.ctrlKey &&
        !event.metaKey &&
        !event.altKey
      ) {
        tabbing = true;
        return;
      }
      const field = textField(document.activeElement);
      if (field) {
        const modified = event.ctrlKey || event.metaKey || event.altKey;
        const cursor =
          event.key.startsWith("Arrow") ||
          event.key === "Home" ||
          event.key === "End";
        if (
          (cursor && (modified || event.shiftKey)) ||
          (!modified &&
            (event.key.length === 1 ||
              ["Home", "End", "Backspace", "Delete"].includes(event.key)))
        )
          setTextTarget(field, true);
      }
      if (
        field &&
        event.key === "Enter" &&
        !event.ctrlKey &&
        !event.metaKey &&
        !event.altKey &&
        !event.shiftKey
      ) {
        event.preventDefault();
        event.stopImmediatePropagation();
        if (!event.repeat) {
          if (editing === field) {
            editing = null;
            setTextTarget(field);
          } else {
            // A confirmed focus also opens the device's native keyboard.
            field.blur();
            field.focus();
            setTextTarget(field, true);
          }
        }
        return;
      }
      if (
        field &&
        editing !== field &&
        !event.altKey &&
        !event.ctrlKey &&
        !event.metaKey &&
        ["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"].includes(event.key)
      ) {
        keydown(event);
        event.preventDefault();
        event.stopImmediatePropagation();
        return;
      }
      if (
        !field &&
        event.key.startsWith("Arrow") &&
        !event.altKey &&
        !event.ctrlKey &&
        !event.metaKey &&
        !event.shiftKey
      ) {
        const group = document.activeElement?.closest<HTMLElement>(
          '[role="tablist"], [role="radiogroup"]',
        );
        if (group) {
          const vertical =
            group.getAttribute("aria-orientation") === "vertical";
          const crossAxis = vertical
            ? event.key === "ArrowLeft" || event.key === "ArrowRight"
            : event.key === "ArrowUp" || event.key === "ArrowDown";
          // Bits consumes all four arrows; TV navigation owns the unused axis.
          if (crossAxis) {
            keydown(event);
            event.preventDefault();
            event.stopImmediatePropagation();
            return;
          }
        }
        const control = document.activeElement;
        const native = control?.closest(roving);
        if (
          control instanceof HTMLElement &&
          control.matches('button, a[href], [role="button"]') &&
          (!native ||
            native.matches('[role="combobox"][aria-expanded="false"]'))
        ) {
          // Enter activates a closed popup; its arrows remain TV navigation.
          keydown(event);
          event.preventDefault();
          event.stopImmediatePropagation();
          return;
        }
      }
      if (["Escape", "BrowserBack", "GoBack", "Back"].includes(event.key))
        keydown(event);
      if (
        event.key !== "ArrowRight" ||
        event.altKey ||
        event.ctrlKey ||
        event.metaKey ||
        event.shiftKey
      )
        return;
      const active = document.activeElement;
      if (!(active instanceof HTMLElement)) return;
      const leaf = active.closest<HTMLElement>(
        '[role="treeitem"], [data-file-entry]',
      );
      if (!leaf || leaf.hasAttribute("aria-expanded")) return;
      // A leaf has nothing to expand. Let the tree handle its key first, then
      // cross into the neighboring region only if it did not move focus.
      queueMicrotask(() => {
        if (
          !tvMode.enabled ||
          document.activeElement !== active ||
          !active.isConnected
        )
          return;
        const tree =
          leaf.closest('[role="tree"], [data-file-tree]') ?? leaf.parentElement;
        const scope = active.closest<HTMLElement>(overlays) ?? document.body;
        const targets = candidates(scope).filter(
          (target) => !tree?.contains(target),
        );
        const next = spatialCandidate(
          active.getBoundingClientRect(),
          targets.map((target) => ({
            target,
            rect: target.getBoundingClientRect(),
          })),
          "ArrowRight",
        );
        if (next) focus(next);
      });
    }
    function navigate(event: KeyboardEvent) {
      if (!["Escape", "BrowserBack", "GoBack", "Back"].includes(event.key))
        keydown(event);
    }
    document.addEventListener("keydown", captureBack, true);
    document.addEventListener("keydown", navigate);
    document.addEventListener("focusin", focusChanged);
    document.addEventListener("pointerdown", beginText, true);
    document.addEventListener("beforeinput", beginText, true);
    document.addEventListener("input", beginText, true);
    document.addEventListener("compositionstart", beginText, true);
    const resize = () => enhanceScroll();
    window.addEventListener("resize", resize);
    const frame = requestAnimationFrame(restore);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      document.removeEventListener("keydown", captureBack, true);
      document.removeEventListener("keydown", navigate);
      document.removeEventListener("focusin", focusChanged);
      document.removeEventListener("pointerdown", beginText, true);
      document.removeEventListener("beforeinput", beginText, true);
      document.removeEventListener("input", beginText, true);
      document.removeEventListener("compositionstart", beginText, true);
      tvInput.message = "";
      window.removeEventListener("resize", resize);
      for (const [element, addedTabIndex] of enhanced) {
        if (addedTabIndex) element.removeAttribute("tabindex");
        delete element.dataset.tvScroll;
      }
      delete root.dataset.uiMode;
    };
  });
}
