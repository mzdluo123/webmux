import { afterEach, expect, test } from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./ViewportHarness.svelte";
import { headsetMode } from "../../src/hooks/shared/headset-mode.svelte";

afterEach(async () => {
  await cleanup();
  headsetMode.enabled = false;
  window.scrollTo(0, 0);
});

const frames = () =>
  new Promise<void>((resolve) =>
    requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
  );

const shownHeight = () =>
  Number(
    page.getByRole("status", { name: "Viewport height" }).element().textContent,
  );

const shownKeyboard = () =>
  page.getByRole("status", { name: "Keyboard visibility" }).element()
    .textContent;

const shownDismissed = () =>
  page.getByRole("status", { name: "Keyboard dismissed" }).element()
    .textContent;

// Scroll snapping waits for composition to end, but keyboard height changes
// apply immediately: Android keyboards compose every word and can hide
// without ending the composition.
test("composition defers scroll snapping but never the viewport height", async () => {
  await page.viewport(400, 700);
  await render(Harness);
  await expect.poll(shownHeight).toBe(700);
  expect(document.documentElement.style.overflow).toBe("hidden");

  const input = page.getByRole("textbox", { name: "Composition input" });
  const field = input.element() as HTMLInputElement;
  // Keep browser caret scrolling out of the hook's composition-scroll check.
  field.focus({ preventScroll: true });
  field.dispatchEvent(
    new CompositionEvent("compositionstart", { bubbles: true, data: "" }),
  );
  window.scrollTo(0, 50);
  window.dispatchEvent(new Event("resize"));
  await frames();
  expect(window.scrollY).toBe(50);

  await page.viewport(400, 420);
  await expect.poll(shownHeight).toBe(420);
  expect(window.scrollY).toBe(50);

  field.dispatchEvent(
    new CompositionEvent("compositionend", { bubbles: true, data: "你" }),
  );
  await expect.poll(() => window.scrollY).toBe(0);

  // An abandoned composition is released by blur.
  field.dispatchEvent(
    new CompositionEvent("compositionstart", { bubbles: true, data: "" }),
  );
  window.scrollTo(0, 50);
  window.dispatchEvent(new Event("resize"));
  await frames();
  expect(window.scrollY).toBe(50);
  field.blur();
  await expect.poll(() => window.scrollY).toBe(0);
});

test("unmount restores the document overflow", async () => {
  document.documentElement.style.overflow = "clip";
  const rendered = await render(Harness);
  expect(document.documentElement.style.overflow).toBe("hidden");
  await rendered.unmount();
  expect(document.documentElement.style.overflow).toBe("clip");
  document.documentElement.style.overflow = "";
});

// Android hides the IME with Back without blurring its editable, so touch
// handlers need to distinguish that state from an open keyboard. Browser chrome
// and rotation must not masquerade as an IME-sized viewport shrink.
test("keyboard visibility follows IME-sized height changes, not chrome or rotation", async () => {
  await page.viewport(400, 700);
  await render(Harness);
  await expect.poll(shownKeyboard).toBe("hidden");
  expect(await shownDismissed()).toBe("false");

  await page.viewport(400, 420);
  await expect.poll(shownKeyboard).toBe("visible");
  expect(await shownDismissed()).toBe("false");
  await page.viewport(400, 700);
  await expect.poll(shownKeyboard).toBe("hidden");
  // Android Back hides the IME but leaves focus in the editable.
  expect(await shownDismissed()).toBe("true");

  await page.viewport(400, 500);
  await expect.poll(shownKeyboard).toBe("visible");
  expect(await shownDismissed()).toBe("false");
  await page.viewport(400, 700);
  await expect.poll(shownKeyboard).toBe("hidden");
  expect(await shownDismissed()).toBe("true");

  // A browser toolbar appearing is smaller than an IME.
  await page.viewport(400, 630);
  await expect.poll(shownKeyboard).toBe("hidden");
  await page.viewport(400, 700);

  // Rotation and split-screen resizes change the width, so they stay unknown
  // until a later resize gives a comparable height.
  await page.viewport(700, 400);
  await expect.poll(shownKeyboard).toBe("unknown");
  await page.viewport(700, 700);
  await expect.poll(shownKeyboard).toBe("hidden");

  // Rotating while the IME is open invalidates the height baseline. Do not
  // fabricate a dismissal from the first post-rotation height.
  await page.viewport(700, 500);
  await expect.poll(shownKeyboard).toBe("visible");
  await page.viewport(400, 500);
  await expect.poll(shownKeyboard).toBe("unknown");
  await page.viewport(400, 800);
  await expect.poll(shownKeyboard).toBe("hidden");
  expect(await shownDismissed()).toBe("false");
});

test("headset window height changes without text focus are not keyboard dismissals", async () => {
  headsetMode.enabled = true;
  await page.viewport(1920, 1080);
  await render(Harness);
  await expect.poll(shownHeight).toBe(1080);
  expect(shownKeyboard()).toBe("hidden");
  expect(document.activeElement?.tagName).not.toBe("INPUT");

  // A controller browser can resize vertically without changing width.
  await page.viewport(1920, 720);
  await expect.poll(shownHeight).toBe(720);
  expect(shownKeyboard()).toBe("hidden");
  expect(shownDismissed()).toBe("false");
  await page.viewport(1280, 720);
  await expect.poll(shownKeyboard).toBe("unknown");
  await page.viewport(1280, 500);
  await expect.poll(shownHeight).toBe(500);
  expect(shownKeyboard()).toBe("hidden");
  expect(shownDismissed()).toBe("false");
});

test("headset system keyboard follows explicit focus through composition and dismissal", async () => {
  headsetMode.enabled = true;
  await page.viewport(1280, 900);
  await render(Harness);
  const field = page
    .getByRole("textbox", { name: "Composition input" })
    .element() as HTMLInputElement;
  field.focus({ preventScroll: true });
  await frames();
  field.dispatchEvent(
    new CompositionEvent("compositionstart", { bubbles: true, data: "" }),
  );
  await page.viewport(1280, 550);
  await expect.poll(shownHeight).toBe(550);
  expect(shownKeyboard()).toBe("visible");
  expect(document.activeElement).toBe(field);
  expect(shownDismissed()).toBe("false");

  // Closing a menu must not focus this field; blur also must not classify the
  // keyboard as gone before its closing viewport animation completes.
  field.blur();
  await frames();
  expect(shownKeyboard()).toBe("visible");
  await page.viewport(1280, 700);
  await expect.poll(shownHeight).toBe(700);
  expect(shownKeyboard()).toBe("visible");
  await page.viewport(1280, 900);
  await expect.poll(shownKeyboard).toBe("hidden");
  expect(shownDismissed()).toBe("true");

  await page.viewport(1280, 650);
  await expect.poll(shownHeight).toBe(650);
  expect(shownKeyboard()).toBe("hidden");
});
