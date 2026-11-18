import { afterEach, expect, test } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import { homeSessionView } from "../../src/domains/sessions/home-session-view.svelte";
import SessionListHarness from "./SessionListHarness.svelte";
import { record, session } from "./shell-fixture";

afterEach(async () => {
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    homeSessionView.compact = false;
    localStorage.removeItem("webmux.homeCompact");
  }
});

const settled = () =>
  new Promise<void>((resolve) =>
    requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
  );

test("renders dormant history before snapshot arrival and keeps the first row anchored", async () => {
  const dormant = record(
    { id: "histaudt", title: "history-audit" },
    { resumeOnOpen: true },
  );
  const view = await render(SessionListHarness, {
    loading: true,
    sessions: [],
    records: [dormant],
  });

  const historyLink = page.getByRole("link", {
    name: "history-audit",
    exact: true,
  });
  const loadingStatus = page.getByRole("status");
  await expect.element(historyLink).toBeVisible();
  await expect.element(loadingStatus).toHaveTextContent("正在加载会话…");
  await settled();

  const initialTop = historyLink.element().getBoundingClientRect().top;
  const historyBottom = historyLink.element().getBoundingClientRect().bottom;
  const statusTop = loadingStatus.element().getBoundingClientRect().top;
  expect(historyBottom).toBeLessThanOrEqual(statusTop + 0.5);

  await view.rerender({
    loading: false,
    sessions: [],
    records: [dormant],
  });
  await expect.element(loadingStatus).not.toBeInTheDocument();
  await settled();
  expect(
    Math.abs(historyLink.element().getBoundingClientRect().top - initialTop),
  ).toBeLessThanOrEqual(0.5);

  const live = session({ id: "live0001", title: "active-build" });
  await view.rerender({
    loading: false,
    sessions: [live],
    records: [dormant],
  });
  const firstLiveLink = page.getByRole("link", {
    name: "active-build",
    exact: true,
  });
  await expect.element(firstLiveLink).toBeVisible();
  await settled();
  expect(
    Math.abs(firstLiveLink.element().getBoundingClientRect().top - initialTop),
  ).toBeLessThanOrEqual(0.5);
});
