import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import type { ComponentProps } from "svelte";
import NewSessionDialog from "../../src/components/sessions/NewSessionDialog.svelte";
import type { Project } from "../../src/contracts/schemas";
import {
  enableCreationFeatures,
  resetCreationFeatures,
} from "./feature-fixture";
import { setFeatureCapabilities } from "../../src/platform/feature-host.svelte";
import { repository, worktree, session, branch, base } from "./shell-fixture";

const plain: Project = {
  id: "directory:notes",
  name: "Notes",
  path: "/projects/notes",
  repositoryId: "",
  availability: "available",
  error: "",
};
const repo = repository({
  worktrees: [
    worktree(),
    worktree({
      id: "feature",
      kind: "linked",
      path: "/repos/webmux-feature",
      branch: "feature/project",
    }),
  ],
});
const git: Project = {
  id: "repository:" + repo.id,
  name: repo.name,
  path: repo.anchorPath,
  repositoryId: repo.id,
  availability: "available",
  error: "",
};
const other = repository({
  id: "repo2",
  name: "other-project",
  anchorPath: "/repos/other",
  worktrees: [
    worktree({ id: "other", repositoryId: "repo2", path: "/repos/other" }),
  ],
});
let calls: { path: string; method: string; body: Record<string, unknown> }[];
const oncreated = vi.fn();
const onclose = vi.fn();

beforeEach(async () => {
  localStorage.clear();
  await enableCreationFeatures(["workspace"], true);
  calls = [];
  oncreated.mockReset();
  onclose.mockReset();
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const path = new URL(String(input), location.href).pathname;
      const method = init?.method ?? "GET";
      const body = init?.body
        ? (JSON.parse(String(init.body)) as Record<string, unknown>)
        : {};
      calls.push({ path, method, body });
      if (path === "/api/projects" && method === "GET")
        return Response.json([plain, git]);
      if (path === "/api/repositories/repo1/worktrees")
        return Response.json(repo);
      if (path === "/api/repositories/repo1/branches")
        return Response.json({
          branches: [
            branch("main", "a".repeat(40), [repo.anchorPath]),
            branch("feature/available", "a".repeat(40)),
          ],
          bases: [base("refs/heads/main", "a".repeat(40), "initial")],
          tags: [],
          defaultBase: "a".repeat(40),
          defaultBaseSubject: "initial",
          defaultBranch: "main",
          defaultParent: "/repos/worktrees",
        });
      if (path === "/api/repositories/repo1/branches/refresh")
        return Response.json({
          branch: "main",
          base: "a".repeat(40),
          baseSubject: "initial",
          baseSource: "local",
          upstream: "",
          upstreamHead: "",
          fetchWarning: "",
        });
      if (path === "/api/repositories/repo1/commits")
        return Response.json({ commits: [] });
      if (path === "/api/projects" && method === "POST")
        return Response.json({ ...plain, path: body.path });
      if (path === "/api/sessions" && method === "POST")
        return Response.json(
          { session: session(), configWarnings: [] },
          { status: 201 },
        );
      return Response.json({ error: "unexpected endpoint" }, { status: 404 });
    }),
  );
});
afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  vi.unstubAllGlobals();
  resetCreationFeatures();
  localStorage.clear();
});
function open(extra: Partial<ComponentProps<typeof NewSessionDialog>> = {}) {
  return render(NewSessionDialog, {
    launchConfigs: { status: "unavailable", error: "launch disabled" },
    defaultCwd: "/projects",
    oncreated,
    onclose,
    onchange: () => {},
    onworktreecreated: () => {},
    ...extra,
  });
}

test("global project-first entry starts a plain directory without launch or files", async () => {
  open();
  await expect
    .element(page.getByRole("combobox", { name: "搜索项目", exact: true }))
    .toBeVisible();
  await page.getByRole("option", { name: /Notes/ }).click();
  await expect
    .element(page.getByRole("textbox", { name: "自定义启动命令", exact: true }))
    .toHaveFocus();
  await expect
    .element(page.getByRole("textbox", { name: "工作目录", exact: true }))
    .not.toBeInTheDocument();
  await expect
    .element(page.getByRole("button", { name: "工作位置", exact: true }))
    .not.toBeInTheDocument();
  await page
    .getByRole("textbox", { name: "自定义启动命令", exact: true })
    .fill("/bin/sh -i");
  await page.getByRole("button", { name: "创建并进入 →", exact: true }).click();
  await expect.poll(() => oncreated.mock.calls.length).toBe(1);
  expect(
    calls.find((call) => call.path === "/api/sessions")?.body,
  ).toMatchObject({ cwd: plain.path, cmd: ["/bin/sh", "-i"] });
  expect(
    calls.some(
      (call) =>
        call.path.startsWith("/api/fs") || call.path.startsWith("/api/launch"),
    ),
  ).toBe(false);
});

test("newly registered Git snapshot scopes work positions before the host list refreshes", async () => {
  open({ repositories: [other] });
  await page.getByRole("option", { name: /webmux/ }).click();
  await expect
    .element(page.getByRole("button", { name: "工作位置", exact: true }))
    .toHaveFocus();
  await page.getByRole("button", { name: "工作位置", exact: true }).click();
  await expect
    .element(page.getByRole("option", { name: /other-project/ }))
    .not.toBeInTheDocument();
  await expect
    .element(page.getByRole("option", { name: /输入或浏览目录/ }))
    .not.toBeInTheDocument();
  await page.getByRole("option", { name: /feature\/project/ }).click();
  await page
    .getByRole("textbox", { name: "自定义启动命令", exact: true })
    .fill("/bin/sh");
  await page.getByRole("button", { name: "创建并进入 →", exact: true }).click();
  await expect.poll(() => oncreated.mock.calls.length).toBe(1);
  expect(
    calls.find((call) => call.path === "/api/sessions")?.body,
  ).toMatchObject({
    workspace: { repositoryId: "repo1", worktreeId: "feature" },
  });
});

test("project changes retain common draft and Escape returns to selection", async () => {
  open({ repositories: [repo] });
  await page.getByRole("option", { name: /Notes/ }).click();
  await page
    .getByRole("textbox", { name: "自定义启动命令", exact: true })
    .fill("/bin/sh -i");
  await page
    .getByRole("button", { name: "高级：会话名称", exact: true })
    .click();
  await page.getByRole("textbox", { name: /会话名称/ }).fill("draft name");
  await userEvent.keyboard("{Escape}");
  await expect
    .element(page.getByRole("combobox", { name: "搜索项目", exact: true }))
    .toBeVisible();
  expect(onclose).not.toHaveBeenCalled();
  await page.getByRole("option", { name: /webmux/ }).click();
  await expect
    .element(page.getByRole("textbox", { name: "自定义启动命令", exact: true }))
    .toHaveValue("/bin/sh -i");

  await expect
    .element(page.getByRole("textbox", { name: /会话名称/ }))
    .toHaveValue("draft name");
});

test("a short drawer restores each step's scroll and the focused draft field", async () => {
  await page.viewport(320, 568);
  const original = vi.mocked(fetch).getMockImplementation()!;
  vi.mocked(fetch).mockImplementation((input, init) => {
    if (String(input).endsWith("/api/projects"))
      return Promise.resolve(
        Response.json([
          plain,
          ...Array.from({ length: 24 }, (_, index) => ({
            ...plain,
            id: `directory:notes-${index}`,
            name: `Notes ${index}`,
            path: `/projects/notes-${index}`,
          })),
        ]),
      );
    return original(input, init);
  });
  try {
    open();
    const search = page.getByRole("combobox", { name: "搜索项目" });
    const project = page.getByRole("option", { name: /^Notes 11/ });
    await search.fill("Notes");
    await expect.element(project).toBeVisible();
    const list = document.querySelector<HTMLElement>(
      '[role="dialog"] [data-slot="command-list"]',
    )!;
    const viewport = document.querySelector<HTMLElement>(
      '[role="dialog"] [data-slot="scroll-area-viewport"]',
    )!;
    list.scrollTop = 360;
    const projectListScroll = list.scrollTop;
    expect(projectListScroll).toBeGreaterThan(0);
    (project.element() as HTMLElement).click();
    const command = page.getByRole("textbox", { name: "自定义启动命令" });
    await expect.element(command).toHaveFocus();
    await command.fill("/bin/sh -i");
    await page.getByRole("button", { name: "高级：会话名称" }).click();
    const name = page.getByRole("textbox", { name: /会话名称/ });
    await expect.element(name).toBeVisible();
    await expect
      .poll(() => viewport.getAnimations({ subtree: true }).length)
      .toBe(0);
    await name.fill("drawer draft");
    (name.element() as HTMLElement).scrollIntoView({ block: "center" });
    const settingsScroll = viewport.scrollTop;
    expect(settingsScroll).toBeGreaterThan(0);
    await userEvent.keyboard("{Escape}");
    await expect.element(search).toHaveFocus();
    await expect.element(search).toHaveValue("Notes");
    await expect.poll(() => viewport.scrollTop).toBe(0);
    expect(list.scrollTop).toBe(projectListScroll);
    (project.element() as HTMLElement).click();
    await expect.poll(() => viewport.scrollTop).toBe(settingsScroll);
    expect(document.activeElement).toBe(name.element());
    await expect.element(name).toHaveValue("drawer draft");
    await expect.element(command).toHaveValue("/bin/sh -i");
  } finally {
    await cleanup();
    await page.viewport(1024, 900);
  }
});

test("explicit worktree context bypasses project fetching and keeps its scope", async () => {
  open({
    repositories: [repo, other],
    entry: { kind: "worktree", repositoryId: "repo1", worktreeId: "feature" },
    scope: { repositoryId: "repo1", worktreeId: "feature" },
  });
  await expect
    .element(page.getByRole("button", { name: "工作位置", exact: true }))
    .toBeVisible();
  expect(calls.some((call) => call.path === "/api/projects")).toBe(false);
  await expect
    .element(page.getByRole("button", { name: "更换项目", exact: true }))
    .not.toBeInTheDocument();
  await page.getByRole("button", { name: "工作位置", exact: true }).click();
  await expect
    .element(page.getByRole("option", { name: /主工作区/ }))
    .not.toBeInTheDocument();
  await userEvent.keyboard("{Escape}");
  await expect.element(page.getByRole("dialog")).toBeVisible();
  expect(onclose).not.toHaveBeenCalled();
});

test("directory split retains the pane cwd and workspace association", async () => {
  open({
    repositories: [repo],
    entry: {
      kind: "directory",
      cwd: "/repos/webmux/subdir",
      workspace: {
        repositoryId: "repo1",
        worktreeId: "main",
        worktreePath: "/repos/webmux",
      },
    },
    split: true,
  });
  await page
    .getByRole("textbox", { name: "自定义启动命令", exact: true })
    .fill("/bin/sh");
  await page
    .getByRole("button", { name: "创建并加入分屏 →", exact: true })
    .click();
  await expect.poll(() => oncreated.mock.calls.length).toBe(1);
  const body = calls.find((call) => call.path === "/api/sessions")?.body;
  expect(body).toMatchObject({
    cwd: "/repos/webmux/subdir",
    workspace: { repositoryId: "repo1", worktreeId: "main" },
  });
  expect(oncreated.mock.calls[0][1]).toBe("horizontal");
});

test("explicit project selection clears the former directory context association", async () => {
  const original = vi.mocked(fetch).getMockImplementation()!;
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>((input, init) => {
      if (
        new URL(String(input), location.href).pathname === "/api/projects" &&
        (!init?.method || init.method === "GET")
      )
        return Promise.resolve(
          Response.json([{ ...plain, path: "/repos/webmux/subdir" }]),
        );
      return original(input, init);
    }),
  );
  open({
    repositories: [repo],
    entry: {
      kind: "directory",
      cwd: "/repos/webmux/subdir",
      workspace: {
        repositoryId: "repo1",
        worktreeId: "main",
        worktreePath: "/repos/webmux",
      },
    },
  });
  await page.getByRole("button", { name: "更换项目", exact: true }).click();
  await expect
    .element(page.getByRole("combobox", { name: "搜索项目", exact: true }))
    .toHaveFocus();
  await page.getByRole("option", { name: /Notes/ }).click();
  await page
    .getByRole("textbox", { name: "自定义启动命令", exact: true })
    .fill("/bin/sh");
  await page.getByRole("button", { name: "创建并进入 →", exact: true }).click();
  await expect.poll(() => oncreated.mock.calls.length).toBe(1);
  const body = calls.find((call) => call.path === "/api/sessions")?.body;
  expect(body?.cwd).toBe("/repos/webmux/subdir");
  expect(body).not.toHaveProperty("workspace");
});

test("an unavailable contextual project explains its state and cannot start", async () => {
  const unavailable = {
    ...repo,
    availability: "unavailable" as const,
    error: "项目目录身份已改变",
  };
  open({
    repositories: [unavailable],
    entry: { kind: "worktree", repositoryId: "repo1", worktreeId: "main" },
    scope: { repositoryId: "repo1" },
  });
  await page
    .getByRole("textbox", { name: "自定义启动命令", exact: true })
    .fill("/bin/sh");
  await expect
    .element(page.getByText("项目目录身份已改变", { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByRole("button", { name: "创建并进入 →", exact: true }))
    .toBeDisabled();
  expect(calls.some((call) => call.path === "/api/sessions")).toBe(false);
});

test("GitHub setup remains reachable without requesting an unavailable source", async () => {
  setFeatureCapabilities({
    version: 2,
    features: [{ id: "workspace", enabled: true, status: "ready" }],
    capabilities: [
      { id: "workspace.projects", available: true },
      { id: "workspace.manage", available: true },
      { id: "workspace.clone", available: true },
      {
        id: "workspace.github",
        available: false,
        reason: "command_unavailable",
      },
    ],
  });
  open();
  await page.getByRole("button", { name: "添加项目…", exact: true }).click();
  await page.getByRole("option", { name: /GitHub 仓库/ }).click();
  await expect.element(page.getByText(/GitHub 来源暂不可用/)).toBeVisible();
  expect(calls.some((call) => call.path === "/api/projects/github")).toBe(
    false,
  );
  await expect
    .element(page.getByRole("button", { name: "克隆并继续", exact: true }))
    .toBeDisabled();
});

for (const context of [false, true])
  for (const mode of ["new", "existing"]) {
    test(`returning to the same project from ${context ? "context" : "list"} preserves the complete ${mode} worktree draft`, async () => {
      const view = await open({
        repositories: [repo],
        ...(context
          ? {
              entry: {
                kind: "directory" as const,
                cwd: repo.anchorPath,
                workspace: {
                  repositoryId: repo.id,
                  worktreeId: "main",
                  worktreePath: repo.anchorPath,
                },
              },
              split: true,
            }
          : {}),
      });
      if (!context) await page.getByRole("option", { name: /webmux/ }).click();
      await page.getByRole("button", { name: "工作位置", exact: true }).click();
      await page.getByRole("option", { name: /^新建工作树/ }).click();
      const name = page.getByRole("textbox", {
        name: "新分支名称",
        exact: true,
      });
      await expect.element(name).not.toHaveValue("");
      if (mode === "new") await name.fill("feature/kept-draft");
      else {
        await page
          .getByRole("radio", { name: "使用已有分支", exact: true })
          .click();
        await page
          .getByRole("button", { name: "已有本地分支", exact: true })
          .click();
        await page.getByRole("option", { name: /^feature\/available/ }).click();
      }
      await page
        .getByRole("button", { name: /^高级：.*存放目录$/, exact: true })
        .click();
      const path = page.getByRole("textbox", {
        name: "新工作树完整路径",
        exact: true,
      });
      await path.fill("/repos/kept-draft");
      await page.getByRole("button", { name: "更换项目", exact: true }).click();
      await page.getByRole("option", { name: /webmux/ }).click();
      await view.rerender({
        repositories: [{ ...repo, checkedAt: "2026-11-15T01:00:00Z" }],
      });
      await expect.element(path).toHaveValue("/repos/kept-draft");
      if (mode === "new")
        await expect.element(name).toHaveValue("feature/kept-draft");
      else
        await expect
          .element(
            page.getByRole("button", { name: "已有本地分支", exact: true }),
          )
          .toHaveTextContent("feature/available");
    });
  }
