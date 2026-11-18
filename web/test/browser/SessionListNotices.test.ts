import { afterEach, expect, test } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import SessionListNotices from "../../src/components/sessions/SessionListNotices.svelte";

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
});

const settled = () =>
  new Promise<void>((resolve) =>
    requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
  );

const baseProps = {
  setupError: "",
  setupBusy: false,
  onretrysetup: () => {},
  error: "",
  rebuildFailure: "",
  recordError: "",
  capabilityError: "",
  repositoryError: "",
  hasRepositories: true,
};

test("renders zero height when idle, compact height for one notice, and bounds five stacked notices to 96px", async () => {
  const view = await render(SessionListNotices, baseProps);
  await settled();

  expect(
    page.getByRole("region", { name: "会话列表提示" }).elements(),
  ).toHaveLength(0);
  expect(view.container.getBoundingClientRect().height).toBe(0);

  await view.rerender({
    ...baseProps,
    error: "会话流连接中断",
  });
  const region = page.getByRole("region", { name: "会话列表提示" });
  await expect.element(region).toBeVisible();
  await settled();

  const singleHeight = view.container.getBoundingClientRect().height;
  expect(singleHeight).toBeGreaterThan(0);
  expect(singleHeight).toBeLessThan(96);

  await view.rerender({
    ...baseProps,
    setupError: "配置读取失败",
    error: "会话流连接中断",
    rebuildFailure: "工作树重建失败",
    recordError: "历史记录读取失败",
    repositoryError: "Git 仓库刷新失败",
  });
  await expect.element(page.getByText(/Git 仓库刷新失败/)).toBeInTheDocument();
  await settled();

  const scrollArea = view.container.querySelector<HTMLElement>(
    '[data-slot="scroll-area"]',
  )!;
  const viewport = region.element() as HTMLElement;
  expect(scrollArea.getBoundingClientRect().height).toBeLessThanOrEqual(96);
  expect(viewport.getBoundingClientRect().height).toBeLessThanOrEqual(96);
  expect(viewport.scrollHeight).toBeGreaterThan(viewport.clientHeight);
});
