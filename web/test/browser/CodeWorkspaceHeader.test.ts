import { afterEach, expect, test, vi } from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import type { CodeWorkspaceTab } from "../../src/domains/code/code-workspace";
import Harness from "./CodeWorkspaceHeaderHarness.svelte";

afterEach(cleanup);

const menuTrigger = () =>
  page.getByRole("button", { name: "代码菜单", exact: true });

function fileTab(
  path: string,
  source: "worktree" | "index" = "worktree",
  status = "unchanged",
): CodeWorkspaceTab {
  const key = `${path}:${source}`;
  return {
    key,
    contextId: "repo",
    rootKind: "worktree",
    sequence: 0,
    status,
    file: {
      snapshotId: key,
      checkedAt: "2026-10-14T00:00:00Z",
      document: {
        nodeId: path,
        path,
        absolutePath: `/repo/${path}`,
        source,
        oid: "",
        contentVersion: key,
        kind: "text",
        text: "",
        lineCount: 0,
        referenceable: true,
        reason: "",
        mediaType: "",
        size: 0,
        downloadable: true,
      },
    },
  };
}

test("file tabs show basenames, disambiguating duplicate paths and sources without losing identity", async () => {
  render(Harness, {
    tabs: [
      fileTab("docs/notes.txt"),
      fileTab("src/shared/app.ts", "worktree", "updated"),
      fileTab("test/shared/app.ts"),
      fileTab("src/config.ts"),
      fileTab("src/config.ts", "index"),
    ],
  });

  const tabList = page.getByRole("tablist", { name: "文件标签", exact: true });
  for (const [path, source, text, changed] of [
    ["docs/notes.txt", "worktree", /^notes\.txt$/],
    ["src/shared/app.ts", "worktree", /src\/shared\/app\.ts.*●/, true],
    ["test/shared/app.ts", "worktree", /test\/shared\/app\.ts/],
    ["src/config.ts", "worktree", /config\.ts.*worktree/],
    ["src/config.ts", "index", /config\.ts.*index/],
  ] as const) {
    const tab = tabList.getByRole("tab", {
      name: `${path} · ${source}${changed ? " ●" : ""}`,
      exact: true,
    });
    await expect.element(tab).toHaveAttribute("title", `/repo/${path}`);
    await expect.element(tab).toMatchTextContent(text);
  }
});

test.each([
  { width: "720px", quickVisible: true },
  { width: "300px", quickVisible: false },
])("the code menu works in a $width pane regardless of quick-action visibility", async ({ width, quickVisible }) => {
  const onFind = vi.fn();
  const onRefresh = vi.fn();
  const reason = "当前目录不支持 Git 比较";
  render(Harness, {
    width,
    onFind,
    onRefresh,
    context: {
      contextId: "root",
      rootKind: "directory",
      rootPath: "/repo",
      browseAvailable: true,
      diffAvailable: false,
      ignoreAvailable: false,
      writableAvailable: true,
      reason,
    },
  });

  const name = "文件内查找";
  const quick = page.getByRole("button", { name, exact: true });
  if (quickVisible) {
    await quick.click();
    expect(onFind).toHaveBeenCalledTimes(1);
  } else {
    await expect.element(quick).not.toBeInTheDocument();
  }
  await expect
    .element(page.getByRole("button", { name: "刷新全部", exact: true }))
    .not.toBeInTheDocument();
  await expect.element(page.getByText(reason)).not.toBeInTheDocument();

  await menuTrigger().click();
  await expect.element(page.getByRole("menu").getByText(reason)).toBeVisible();
  await page.getByRole("menuitem", { name, exact: true }).click();
  expect(onFind).toHaveBeenCalledTimes(quickVisible ? 2 : 1);

  await menuTrigger().click();
  await page.getByRole("menuitem", { name: "刷新全部", exact: true }).click();
  expect(onRefresh).toHaveBeenCalledTimes(1);
});
