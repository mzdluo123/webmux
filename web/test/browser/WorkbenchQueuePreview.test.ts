import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import WorkbenchSessionQueue from "../../src/components/workspace/WorkbenchSessionQueue.svelte";
import { now, session } from "../../stories/fixtures";

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  vi.restoreAllMocks();
});

function renderQueue() {
  const running = {
    ...session,
    id: "queue-preview-1",
    title: "Review build",
    status: "running" as const,
    preview: "queue fixture\npreview output\n",
  };
  return render(WorkbenchSessionQueue, {
    sessions: [running],
    available: [running],
    repositories: [],
    records: [],
    archive: {
      now,
      filtering: false,
      resume: vi.fn(),
      remove: vi.fn(),
      onresumed: vi.fn(),
    },
    attentionOnly: false,
    filtering: false,
    now,
    onclear: () => {},
    onrename: () => {},
    onclose: () => {},
  });
}

test("docked preview keeps aside mounted while closing, clips only during motion, and restores row focus on Escape", async () => {
  await page.viewport(1440, 900);
  await renderQueue();
  expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(
    window.innerWidth,
  );

  const trigger = page.getByRole("button", { name: "预览 Review build" });
  await trigger.click();

  const close = page
    .getByRole("complementary", { name: "预览 Review build", exact: true })
    .getByRole("button", { name: "关闭预览" });
  await expect.element(close).toHaveFocus();

  const aside = () => document.querySelector<HTMLElement>("aside");
  const column = () => aside()?.parentElement?.parentElement ?? null;
  expect(aside()).not.toBeNull();
  expect(column()?.classList.contains("sticky")).toBe(false);

  if (!matchMedia("(prefers-reduced-motion: reduce)").matches) {
    expect(column()?.classList.contains("overflow-clip")).toBe(true);
  }
  await expect
    .poll(() => column()?.classList.contains("overflow-clip"))
    .toBe(false);
  expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(
    window.innerWidth,
  );

  await userEvent.keyboard("{Escape}");
  await expect.element(trigger).toHaveFocus();

  if (!matchMedia("(prefers-reduced-motion: reduce)").matches) {
    const closingAside = aside();
    expect(closingAside).not.toBeNull();
    expect(closingAside?.inert).toBe(true);
    expect(closingAside?.getBoundingClientRect().height).toBeGreaterThan(0);
    expect(column()?.classList.contains("overflow-clip")).toBe(true);
  }

  await expect.poll(() => aside()).toBeNull();
  expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(
    window.innerWidth,
  );
});

test("docked preview settles immediately under prefers-reduced-motion and restores focus on Escape", async () => {
  await page.viewport(1440, 900);
  const originalMatchMedia = window.matchMedia.bind(window);
  vi.spyOn(window, "matchMedia").mockImplementation((query: string) => {
    const list = originalMatchMedia(query);
    if (query !== "(prefers-reduced-motion: reduce)") return list;
    return {
      matches: true,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    };
  });

  await renderQueue();
  const trigger = page.getByRole("button", { name: "预览 Review build" });
  await trigger.click();

  const close = page
    .getByRole("complementary", { name: "预览 Review build", exact: true })
    .getByRole("button", { name: "关闭预览" });
  await expect.element(close).toHaveFocus();

  const openedAside = document.querySelector<HTMLElement>("aside");
  expect(openedAside).not.toBeNull();
  expect(
    openedAside?.parentElement?.parentElement?.classList.contains(
      "overflow-clip",
    ),
  ).toBe(false);

  await userEvent.keyboard("{Escape}");
  await expect.element(trigger).toHaveFocus();
  expect(document.querySelector("aside")).toBeNull();
});
