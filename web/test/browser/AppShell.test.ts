import { afterEach, beforeEach, expect, test } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { render } from "vitest-browser-svelte/pure";
import { workbenchLayout } from "../../src/domains/workspace/workbench-layout.svelte";
import { resetAttentionToasts } from "../../src/domains/notifications/attention-toast.svelte";
import {
  openSession,
  shellUi,
  type Shell,
} from "../../src/hooks/shared/shell.svelte";
import { createTerminalDrafts } from "../../src/domains/terminal/terminal-draft.svelte";
import {
  commands,
  installChatFixture,
  respond,
  socketAt,
} from "./chat-fixture";
import ShellHarness from "./ShellHarness.svelte";
import {
  chat,
  launchConfig,
  record,
  repository,
  session,
  worktree,
} from "./shell-fixture";

installChatFixture();
beforeEach(() => {
  resetAttentionToasts();
  workbenchLayout.value = "workspaces";
  workbenchLayout.attentionOnly = false;
  shellUi.setCollapsed(false);
  history.replaceState(null, "", "/");
  respond(/^GET \/api\/repositories$/, () => Response.json([repository()]));
  respond(/^GET \/api\/launch-configs$/, () => Response.json([launchConfig()]));
  respond(/^GET \/api\/sessions$/, () => Response.json([record()]));
});
afterEach(() => {
  localStorage.removeItem("webmux.sidebarCollapsed");
  localStorage.removeItem("webmux.toastNotifications");
  localStorage.removeItem("webmux.workbenchTree");
  workbenchLayout.value = "queue";
  workbenchLayout.attentionOnly = false;
  localStorage.removeItem("webmux.workbenchLayout");
  resetAttentionToasts();
  history.replaceState(null, "", "/");
});

const sidebar = () => page.getByRole("complementary", { name: "侧栏" });

async function snapshot(
  sessions = [chat({ id: "chataaaa", title: "修复排序" })],
) {
  const socket = await socketAt("/ws/sessions");
  socket.frame({ type: "snapshot", sessions, forwards: [] } as never);
  return socket;
}

// Must stay first: shellUi reads the saved width once, when it loads.
test("without a saved width the sidebar starts at its default 288px", async () => {
  expect(localStorage.getItem("webmux.sidebarWidth")).toBeNull();
  expect(shellUi.width).toBe(288);
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  await expect
    .poll(() => sidebar().element().getBoundingClientRect().width)
    .toBe(288);
});

test("the grouping switch changes Workbench without opening another page", async () => {
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  const grouping = sidebar().getByRole("radio", { name: "按分支" });
  await expect.element(grouping).toBeChecked();
  await sidebar().getByRole("radio", { name: "按会话" }).click();
  expect(location.pathname).toBe("/");
  expect(localStorage.getItem("webmux.workbenchLayout")).toBe("queue");
  await expect.element(grouping).not.toBeChecked();
  await expect
    .element(sidebar().getByRole("radiogroup", { name: "会话范围" }))
    .toBeVisible();
});

test("the grouping switch leaves the active session and keyboard focus in place", async () => {
  history.replaceState(null, "", "/s/shellaaa");
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot([session({ id: "shellaaa", title: "pnpm dev" })]);
  const grouping = sidebar().getByRole("radio", { name: "按会话" });
  await grouping.click();
  expect(location.pathname).toBe("/s/shellaaa");
  await expect.element(grouping).toHaveFocus();
  await expect.element(page.getByTestId("session-page")).toBeVisible();
  await expect
    .element(sidebar().getByRole("button", { name: /^pnpm dev/ }))
    .toHaveAttribute("aria-current", "page");
});

test("the collapsed rail keeps the expand and primary actions, not the mode switch", async () => {
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  await page.getByRole("button", { name: "收起侧栏" }).click();
  expect(localStorage.getItem("webmux.sidebarCollapsed")).toBe("true");
  const rail = page.getByRole("navigation", { name: "窄栏导航" });
  await expect
    .element(rail.getByRole("button", { name: "展开侧栏" }))
    .toBeVisible();
  await expect
    .element(rail.getByRole("button", { name: "新建会话" }))
    .toBeVisible();
  expect(rail.getByRole("radio", { name: "按分支" }).elements()).toEqual([]);
  expect(
    document.querySelector(".app-shell")?.getAttribute("data-collapsed"),
  ).toBe("true");
  await rail.getByRole("button", { name: "展开侧栏" }).click();
  expect(localStorage.getItem("webmux.sidebarCollapsed")).toBe("false");
  await expect
    .element(sidebar().getByRole("radio", { name: "按分支" }))
    .toBeVisible();
});

test("the sidebar's new-session action opens the dialog in branch mode and focuses quick start in conversation mode", async () => {
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  await sidebar().getByRole("button", { name: "新建会话" }).click();
  const dialog = page.getByRole("dialog", { name: "新建会话" });
  await expect.element(dialog).toBeVisible();
  await userEvent.keyboard("{Escape}");
  await expect.element(dialog).not.toBeInTheDocument();

  await sidebar().getByRole("radio", { name: "按会话" }).click();
  await sidebar().getByRole("button", { name: "新建会话" }).click();
  expect(location.pathname).toBe("/");
  await expect
    .element(page.getByRole("textbox", { name: "任务描述" }))
    .toHaveFocus();
});

test("opening any session preserves the active workbench grouping", () => {
  for (const layout of ["workspaces", "queue"] as const) {
    workbenchLayout.value = layout;
    openSession(session({ id: "shellaaa" }));
    expect(workbenchLayout.value).toBe(layout);
    expect(location.pathname).toBe("/s/shellaaa");
    openSession(chat({ id: "chataaaa" }));
    expect(workbenchLayout.value).toBe(layout);
    expect(location.pathname).toBe("/s/chataaaa");
  }
});

test("session references use the active chat and visibly disable locked drafts", async () => {
  workbenchLayout.value = "queue";
  history.replaceState(null, "", "/s/anchorzz");
  const drafts = createTerminalDrafts();
  const anchor = drafts.get("anchorzz");
  const focused = drafts.get("focusedz");
  drafts.edit(anchor, "Leave this draft alone");
  drafts.edit(focused, "Compare replace suffix", 8, 15);
  let shell!: Shell;
  render(ShellHarness, {
    drafts,
    exposeShell: (value) => (shell = value),
  });
  const socket = await snapshot([
    chat({ id: "anchorzz", title: "Anchor" }),
    chat({ id: "focusedz", title: "Focused" }),
    chat({ id: "sourcezz", title: "Source" }),
  ]);
  shell.setActivePane("focusedz");
  const sourceMenu = sidebar().getByRole("button", {
    name: "会话操作：Source",
    exact: true,
  });
  await sourceMenu.click();
  const reference = page.getByRole("menuitem", {
    name: "引用到当前聊天",
    exact: true,
  });
  await expect.element(reference).toBeVisible();
  await expect.element(reference).not.toHaveAttribute("aria-disabled", "true");
  drafts.beginSubmission(focused, "pending");
  await expect.element(reference).toHaveAttribute("aria-disabled", "true");
  expect(focused.text).toBe("Compare replace suffix");
  drafts.releaseSubmission(focused);
  await expect.element(reference).not.toHaveAttribute("aria-disabled", "true");
  await reference.click();
  expect(focused.text).toBe("Compare [Source](/s/sourcezz) suffix");
  expect(anchor.text).toBe("Leave this draft alone");
  expect(location.pathname).toBe("/s/anchorzz");

  // No reference action for the focused session itself.
  await sidebar().getByRole("button", { name: "会话操作：Focused" }).click();
  expect(reference.elements()).toEqual([]);
  await userEvent.keyboard("{Escape}");

  // Following a pane's view change removes the chat destination.
  socket.frame({
    type: "updated",
    session: chat({ id: "focusedz", title: "Focused" }, "terminal"),
  } as never);
  await sourceMenu.click();
  await expect.element(reference).not.toBeInTheDocument();
});

test("renaming from the sidebar menu uses the shared dialog", async () => {
  workbenchLayout.value = "queue";
  respond(/^PATCH \/api\/sessions\/chataaaa$/, () =>
    Response.json(chat({ id: "chataaaa", title: "首页排序" })),
  );
  render(ShellHarness, { drafts: createTerminalDrafts() });
  const socket = await snapshot();
  const archived = page.getByRole("region", { name: /的已归档会话$/ });
  await archived.getByRole("button", { name: /已归档/ }).click();
  // Archived records resume or delete their record; they have no rename.
  await expect
    .element(archived.getByRole("button", { name: "恢复", exact: true }))
    .toBeVisible();
  await expect
    .element(archived.getByRole("button", { name: "删除 整理文档" }))
    .toBeVisible();
  expect(archived.getByRole("button", { name: "修改标题" }).elements()).toEqual(
    [],
  );

  await sidebar().getByRole("button", { name: "会话操作：修复排序" }).click();
  await page.getByRole("menuitem", { name: "修改标题" }).click();
  const dialog = page.getByRole("dialog");
  const field = dialog.getByRole("textbox");
  await expect.element(field).toHaveValue("修复排序");
  await field.fill("首页排序");
  await userEvent.keyboard("{Enter}");
  await expect.element(dialog).not.toBeInTheDocument();
  expect(commands().at(-1)).toMatchObject({
    method: "PATCH",
    path: "/api/sessions/chataaaa",
    body: { name: "首页排序" },
  });
  // The sidebar shows what the list event brings.
  socket.frame({
    type: "updated",
    session: chat({ id: "chataaaa", title: "首页排序" }),
  } as never);
  await expect
    .element(sidebar().getByRole("button", { name: /^首页排序/ }))
    .toBeVisible();
});

test("a rejected name stays in the dialog and the row keeps its title", async () => {
  workbenchLayout.value = "queue";
  respond(/^PATCH \/api\/sessions\/chataaaa$/, () =>
    Response.json({ error: "名称不能包含控制字符" }, { status: 400 }),
  );
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  await sidebar().getByRole("button", { name: "会话操作：修复排序" }).click();
  await page.getByRole("menuitem", { name: "修改标题" }).click();
  await page.getByRole("dialog").getByRole("textbox").fill("坏\u0007名");
  await page.getByRole("dialog").getByRole("button", { name: "保存" }).click();
  await expect
    .element(page.getByRole("dialog").getByText("名称不能包含控制字符"))
    .toBeVisible();
  await expect
    .element(sidebar().getByRole("button", { name: /^修复排序/ }))
    .toBeVisible();
});

test("an archived chat resumes on its chat side, or says why it cannot", async () => {
  workbenchLayout.value = "queue";
  respond(/^POST \/api\/sessions\/archivea\/resume$/, () =>
    Response.json(
      { error: "工作目录 /repos/webmux 已不存在" },
      { status: 409 },
    ),
  );
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  const archived = page.getByRole("region", { name: /的已归档会话$/ });
  await archived.getByRole("button", { name: /已归档/ }).click();
  await archived.getByRole("button", { name: "恢复", exact: true }).click();
  await expect
    .element(archived.getByRole("alert"))
    .toMatchTextContent(
      /无法恢复“整理文档”：工作目录 \/repos\/webmux 已不存在/,
    );
  expect(location.pathname).toBe("/");
  expect(commands().at(-1)).toMatchObject({
    path: "/api/sessions/archivea/resume",
    body: { view: "chat" },
  });

  respond(/^POST \/api\/sessions\/archivea\/resume$/, () =>
    Response.json(chat({ id: "archivea", title: "整理文档" })),
  );
  await archived.getByRole("button", { name: "恢复", exact: true }).click();
  await expect
    .poll(
      () =>
        commands().filter((c) => c.path === "/api/sessions/archivea/resume")
          .length,
    )
    .toBe(2);
});

test("the sidebar menu archives a live chat and the timeline deletes an archived record", async () => {
  workbenchLayout.value = "queue";
  respond(
    /^DELETE \/api\/sessions\/chataaaa$/,
    () => new Response(null, { status: 204 }),
  );
  respond(
    /^DELETE \/api\/sessions\/archivea$/,
    () => new Response(null, { status: 204 }),
  );
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  await sidebar().getByRole("button", { name: "会话操作：修复排序" }).click();
  await page.getByRole("menuitem", { name: "停止会话并归档" }).click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "停止并归档", exact: true })
    .click();
  await expect
    .poll(() =>
      commands().some(
        (c) => c.method === "DELETE" && c.path === "/api/sessions/chataaaa",
      ),
    )
    .toBe(true);

  const archived = page.getByRole("region", { name: /的已归档会话$/ });
  await archived.getByRole("button", { name: /已归档/ }).click();
  await archived.getByRole("button", { name: "删除 整理文档" }).click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "确认" })
    .click();
  await expect
    .poll(() =>
      commands().some(
        (c) =>
          c.method === "DELETE" &&
          c.path === "/api/sessions/archivea" &&
          c.query === "?forget=1",
      ),
    )
    .toBe(true);
});

const linkedTree = worktree({
  id: "wt1",
  path: "/repos/webmux.worktrees/wt1",
  kind: "linked",
  branch: "wt/fix-sort",
});
const linkedChat = record({
  id: "archivew",
  title: "分支聊天",
  cwd: linkedTree.path,
  workspace: {
    repositoryId: "repo1",
    worktreeId: "wt1",
    worktreePath: linkedTree.path,
  },
});
function serveLinkedWorktreeChat() {
  respond(/^GET \/api\/repositories$/, () =>
    Response.json([repository({ worktrees: [worktree(), linkedTree] })]),
  );
  respond(/^GET \/api\/sessions$/, () => Response.json([linkedChat]));
  respond(
    /^POST \/api\/repositories\/repo1\/worktrees\/wt1\/removal-preview$/,
    () =>
      Response.json({
        confirmationId: "conf1234",
        expiresAt: "2026-10-28T01:00:00Z",
        worktree: linkedTree,
        sessions: [],
        archived: [linkedChat],
        commit: "c".repeat(40),
        push: { state: "unpushed", branches: [], more: 0 },
        blockers: [],
        warnings: [],
        forceAllowed: true,
      }),
  );
  respond(
    /^POST \/api\/repositories\/repo1\/worktrees\/wt1\/remove$/,
    () => new Response(null, { status: 204 }),
  );
  respond(
    /^DELETE \/api\/sessions\/archivew$/,
    () => new Response(null, { status: 204 }),
  );
}
async function openLinkedChatArchive() {
  workbenchLayout.value = "queue";
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  const archived = page.getByRole("region", { name: /的已归档会话$/ });
  await archived.getByRole("button", { name: /已归档/ }).click();
  return archived;
}

test("deleting an archived record leaves its linked worktree in place", async () => {
  serveLinkedWorktreeChat();
  const archived = await openLinkedChatArchive();
  await archived.getByRole("button", { name: "删除 分支聊天" }).click();
  const dialog = page.getByRole("alertdialog");
  await expect
    .element(dialog)
    .toMatchTextContent(/工作树与 agent 自己保存的会话不受影响/);
  await dialog.getByRole("button", { name: "确认" }).click();
  await expect
    .poll(() =>
      commands().some(
        (c) =>
          c.method === "DELETE" &&
          c.path === "/api/sessions/archivew" &&
          c.query === "?forget=1",
      ),
    )
    .toBe(true);
  expect(
    commands().some(
      (c) =>
        c.method === "POST" &&
        c.path === "/api/repositories/repo1/worktrees/wt1/removal-preview",
    ),
  ).toBe(false);
});

test("deleting an archived record and its linked worktree removes both", async () => {
  serveLinkedWorktreeChat();
  const archived = await openLinkedChatArchive();
  await archived.getByRole("button", { name: "更多操作 分支聊天" }).click();
  await page.getByRole("menuitem", { name: "删除记录和工作树" }).click();
  const dialog = page.getByRole("dialog", { name: "删除 worktree" });
  await expect.element(dialog).toBeVisible();
  await expect.element(dialog).toMatchTextContent(/wt\/fix-sort/);
  await expect.element(dialog).toMatchTextContent(/所选聊天记录会一并删除/);
  await expect
    .element(dialog)
    .toMatchTextContent(/将删除的已归档聊天.*分支聊天/);
  expect(dialog.getByText("保留的已归档会话").elements()).toEqual([]);
  expect(dialog.getByText("尚未推送").elements()).toEqual([]);
  await dialog.getByRole("button", { name: "确认安全删除" }).click();
  await expect
    .poll(() =>
      commands().some(
        (c) =>
          c.method === "POST" &&
          c.path === "/api/repositories/repo1/worktrees/wt1/remove",
      ),
    )
    .toBe(true);
  await expect
    .poll(() =>
      commands().some(
        (c) =>
          c.method === "DELETE" &&
          c.path === "/api/sessions/archivew" &&
          c.query === "?forget=1",
      ),
    )
    .toBe(true);
});

const branchTree = worktree({
  id: "wt1",
  path: "/repos/webmux.worktrees/wt1",
  kind: "linked",
  branch: "wt/fix-sort",
  head: "c".repeat(40),
});
const branchChat = record({
  id: "archivew",
  title: "分支聊天",
  cwd: branchTree.path,
  workspace: {
    repositoryId: "repo1",
    worktreeId: "wt1",
    worktreePath: branchTree.path,
  },
});
const releasedChat = record({
  ...branchChat,
  released: {
    id: "rel1",
    repositoryId: "repo1",
    path: branchTree.path,
    commit: "c".repeat(40),
    branch: "wt/fix-sort",
    releasedAt: "2026-10-28T00:00:00Z",
  },
});

test("the archived timeline releases a chat's worktree after a preview and marks the record", async () => {
  workbenchLayout.value = "queue";
  respond(/^GET \/api\/repositories$/, () =>
    Response.json([repository({ worktrees: [worktree(), branchTree] })]),
  );
  respond(/^GET \/api\/sessions$/, () => Response.json([branchChat]));
  respond(
    /^POST \/api\/repositories\/repo1\/worktrees\/wt1\/release-preview$/,
    () =>
      Response.json({
        confirmationId: "conf1234",
        expiresAt: "2026-10-28T01:00:00Z",
        worktree: branchTree,
        commit: "c".repeat(40),
        push: { state: "unpushed", branches: [], more: 0 },
        archived: [branchChat],
        ignored: [
          { path: "node_modules/", bytes: 3 * 1024 * 1024 },
          { path: ".env" },
        ],
        ignoredMore: 2,
        ignoredBytes: 3 * 1024 * 1024,
        sizeComplete: false,
        blockers: [],
      }),
  );
  respond(/^POST \/api\/repositories\/repo1\/worktrees\/wt1\/release$/, () => {
    respond(/^GET \/api\/repositories$/, () =>
      Response.json([repository({ worktrees: [worktree()] })]),
    );
    respond(/^GET \/api\/sessions$/, () => Response.json([releasedChat]));
    return Response.json({ releaseId: "rel1", recordIds: ["archivew"] });
  });
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  const archived = page.getByRole("region", { name: /的已归档会话$/ });
  await archived.getByRole("button", { name: /已归档/ }).click();
  await archived.getByRole("button", { name: "更多操作 分支聊天" }).click();
  await page.getByRole("menuitem", { name: "释放空间" }).click();
  const dialog = page.getByRole("dialog", { name: "释放空间" });
  // Deleted ignored files are warned about, not listed.
  await expect.element(dialog).not.toMatchTextContent(/node_modules\//);
  await expect.element(dialog).not.toMatchTextContent(/随目录删除的忽略文件/);
  await expect.element(dialog).not.toMatchTextContent(/另有 2 项/);
  await expect.element(dialog).toMatchTextContent(/重建工作树也无法找回/);
  await expect
    .element(dialog)
    .toMatchTextContent(/尚未推送到任何远程.*请先推送/);
  // An unpushed commit warns, never blocks.
  await dialog.getByRole("button", { name: "释放空间" }).click();
  await expect.element(archived.getByText(/已释放空间/)).toBeVisible();
  expect(commands().at(-1)).toMatchObject({
    path: "/api/repositories/repo1/worktrees/wt1/release",
    body: { confirmationId: "conf1234" },
  });
  // Released, the record offers no second release menu.
  expect(
    archived.getByRole("button", { name: "更多操作 分支聊天" }).elements(),
  ).toEqual([]);
});

test("a released archived chat rebuilds its worktree after confirmation before resuming", async () => {
  workbenchLayout.value = "queue";
  respond(/^GET \/api\/sessions$/, () => Response.json([releasedChat]));
  respond(/^POST \/api\/hybrid-sessions\/archivew\/rebuild-preview$/, () =>
    Response.json({
      releaseId: "rel1",
      path: branchTree.path,
      commit: "c".repeat(40),
      branch: "wt/fix-sort",
      mode: "detached",
      reason: "branchMissing",
      blockers: [],
    }),
  );
  respond(/^POST \/api\/hybrid-sessions\/archivew\/rebuild$/, () =>
    Response.json({ ...branchTree, branch: null, detached: true }),
  );
  respond(/^POST \/api\/sessions\/archivew\/resume$/, () =>
    Response.json(chat({ id: "archivew", title: "分支聊天" })),
  );
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  const archived = page.getByRole("region", { name: /的已归档会话$/ });
  await archived.getByRole("button", { name: /已归档/ }).click();
  const resumeBtn = archived.getByRole("button", { name: "恢复", exact: true });

  // Cancelling leaves the record released and resumes nothing.
  await resumeBtn.click();
  const dialog = page.getByRole("dialog", { name: "重建工作树后恢复" });
  await expect
    .element(dialog)
    .toMatchTextContent(/分支 wt\/fix-sort 已被删除.*detached/);
  await dialog.getByRole("button", { name: "取消" }).click();
  await expect.element(dialog).not.toBeInTheDocument();
  const released = () => commands().filter((c) => c.path.includes("archivew"));
  expect(released().map((c) => c.path)).toEqual([
    "/api/hybrid-sessions/archivew/rebuild-preview",
  ]);
  expect(location.pathname).toBe("/");

  await resumeBtn.click();
  await dialog.getByRole("button", { name: "重建并恢复" }).click();
  await expect
    .poll(() =>
      released().some((c) => c.path === "/api/sessions/archivew/resume"),
    )
    .toBe(true);
  expect(released().slice(-2)).toMatchObject([
    {
      path: "/api/hybrid-sessions/archivew/rebuild",
      body: { releaseId: "rel1", mode: "detached", commit: "c".repeat(40) },
    },
    { path: "/api/sessions/archivew/resume", body: { view: "chat" } },
  ]);
});

test("the workbench tree lists a released record under 已不可用工作区 and rebuilds it on click", async () => {
  workbenchLayout.value = "workspaces";
  history.replaceState(null, "", "/s/otherabc");
  respond(/^GET \/api\/repositories$/, () =>
    Response.json([repository({ worktrees: [worktree()] })]),
  );
  respond(/^GET \/api\/sessions$/, () => Response.json([releasedChat]));
  respond(/^POST \/api\/hybrid-sessions\/archivew\/rebuild-preview$/, () =>
    Response.json({
      releaseId: "rel1",
      path: branchTree.path,
      commit: "c".repeat(40),
      branch: "wt/fix-sort",
      mode: "branch",
      blockers: [],
    }),
  );
  respond(/^POST \/api\/hybrid-sessions\/archivew\/rebuild$/, () =>
    Response.json(branchTree),
  );
  respond(/^POST \/api\/sessions\/archivew\/resume$/, () =>
    Response.json(chat({ id: "archivew", title: "分支聊天" })),
  );
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot();
  const tree = sidebar().getByRole("tree", { name: "仓库与会话" });
  const repo = tree.getByRole("treeitem", { name: "webmux" });
  await expect.element(repo).toHaveAttribute("aria-expanded", "false");
  await tree.getByRole("button", { name: "展开webmux" }).click();
  // Only live sessions count: the record alone adds no number.
  const group = tree.getByRole("treeitem", { name: "已不可用工作区" });
  await expect.element(group).toHaveAttribute("aria-expanded", "false");
  await tree.getByRole("button", { name: "展开已不可用工作区" }).click();
  const row = tree.getByRole("treeitem", {
    name: "分支聊天（已释放空间，已结束）",
  });
  await row.click();
  const dialog = page.getByRole("dialog", { name: "重建工作树后恢复" });
  await dialog.getByRole("button", { name: "重建并恢复" }).click();
  await expect.poll(() => location.pathname).toBe("/s/archivew");
});

test("a session on a worktree this page has not read re-reads the repositories once", async () => {
  const fresh = worktree({
    id: "wt9",
    kind: "linked",
    path: "/repos/webmux-new",
    branch: "fresh-branch",
  });
  let reads = 0;
  respond(/^GET \/api\/repositories$/, () => {
    reads += 1;
    return Response.json([
      repository({ worktrees: reads > 1 ? [worktree(), fresh] : [worktree()] }),
    ]);
  });
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await expect.poll(() => reads).toBe(1);
  await snapshot([
    session({
      id: "elsewher",
      title: "别处创建",
      workspace: {
        repositoryId: "repo1",
        worktreeId: "wt9",
        worktreePath: fresh.path,
      },
    }),
  ]);
  await expect.poll(() => reads).toBe(2);
  await expect
    .element(sidebar().getByText("fresh-branch").first())
    .toBeVisible();
  expect(sidebar().getByText("已不可用工作区").elements()).toEqual([]);
  // The worktree is known now, so the settled list asks for nothing more.
  expect(reads).toBe(2);
});

test("shows a top-right toast when a session raises attention and navigates on click", async () => {
  render(ShellHarness, { drafts: createTerminalDrafts() });
  const socket = await snapshot([
    chat({ id: "chataaaa", title: "修复排序" }),
    session({ id: "shellbbb", title: "数据库迁移" }),
  ]);

  socket.frame({
    type: "updated",
    session: session({
      id: "shellbbb",
      title: "数据库迁移",
      status: "waiting",
      attention: {
        kind: "permission",
        source: "agent",
        since: "2026-11-05T12:00:00Z",
      },
    }),
  } as never);

  const region = page.getByRole("region", { name: "会话通知" });
  await expect.element(region).toBeVisible();
  const openButton = region.getByRole("button", {
    name: "打开会话 数据库迁移：等待授权 · Agent 请求执行工具操作",
  });
  await expect.element(openButton).toBeVisible();
  await openButton.click();
  expect(location.pathname).toBe("/s/shellbbb");
  await expect.element(region).not.toBeInTheDocument();
});

test("workbench sidebar offers a workspace refresh button and follows /ws/repositories changes", async () => {
  let reads = 0;
  const linked = worktree({
    id: "wt-ext",
    kind: "linked",
    branch: "feat/ext-sync",
    path: "/repos/webmux.worktrees/ext-sync",
  });
  let currentWorktrees = [worktree(), linked];
  respond(/^GET \/api\/repositories$/, () => {
    reads++;
    return Response.json([repository({ worktrees: currentWorktrees })]);
  });

  history.replaceState(null, "", "/s/shellaaa");
  render(ShellHarness, { drafts: createTerminalDrafts() });
  await snapshot([
    session({
      id: "shellaaa",
      title: "主终端",
      workspace: {
        repositoryId: "repo1",
        worktreeId: "wt-ext",
        worktreePath: linked.path,
      },
    }),
  ]);

  const newSessionBtn = sidebar().getByRole("button", { name: "新建会话" });
  const refreshBtn = sidebar().getByRole("button", { name: "刷新工作区" });
  await expect.element(refreshBtn).toBeVisible();
  expect(Math.round(refreshBtn.element().getBoundingClientRect().height)).toBe(
    Math.round(newSessionBtn.element().getBoundingClientRect().height),
  );

  await expect
    .element(sidebar().getByText("feat/ext-sync").first())
    .toBeVisible();

  // The refresh button is reachable in both Workbench groupings.
  await sidebar().getByRole("radio", { name: "按会话" }).click();
  await expect.element(refreshBtn).toBeVisible();
  await sidebar().getByRole("radio", { name: "按分支" }).click();
  await expect.element(refreshBtn).toBeVisible();

  // External removal reflected when clicking the sidebar refresh button.
  currentWorktrees = [worktree()];
  const beforeClickReads = reads;
  await refreshBtn.click();
  await expect.poll(() => reads).toBeGreaterThan(beforeClickReads);
  await expect
    .poll(() => sidebar().getByText("feat/ext-sync").elements().length)
    .toBe(0);

  // Live /ws/repositories "changed" frame automatically re-reads repositories without clicking.
  currentWorktrees = [worktree(), linked];
  const repoSocket = await socketAt("/ws/repositories");
  repoSocket.open();
  const beforePushReads = reads;
  repoSocket.frame({ type: "changed" } as never);
  await expect.poll(() => reads).toBeGreaterThan(beforePushReads);
  await expect
    .element(sidebar().getByText("feat/ext-sync").first())
    .toBeVisible();
});

test("sidebar hides idle inbox, balances scope radios, hands focus back on empty inbox Escape, and keeps non-Relay transport single-line", async () => {
  history.replaceState(null, "", "/");
  render(ShellHarness, { drafts: createTerminalDrafts() });
  const socket = await snapshot([
    session({ id: "shellbbb", title: "日常任务" }),
  ]);

  await sidebar().getByRole("radio", { name: "按会话" }).click();

  const scope = sidebar().getByRole("radiogroup", { name: "会话范围" });
  const allRadio = scope.getByRole("radio", { name: "全部", exact: true });
  const pendingRadio = scope.getByRole("radio", {
    name: "待处理",
    exact: true,
  });
  await expect.element(allRadio).toBeVisible();
  await expect.element(pendingRadio).toBeVisible();
  expect(
    Math.abs(
      allRadio.element().getBoundingClientRect().width -
        pendingRadio.element().getBoundingClientRect().width,
    ),
  ).toBeLessThanOrEqual(2);

  // Idle state renders no inbox row above the scope controls.
  const inboxTrigger = () =>
    sidebar().getByRole("button", { name: /^会话通知/ });
  expect(inboxTrigger().elements()).toEqual([]);

  // Non-Relay host status keeps WebSocket single-line without an invisible Relay spacer.
  await sidebar()
    .getByRole("button", { name: /^查看主机状态：/ })
    .click();
  const hostDialog = page.getByRole("dialog", { name: "主机状态详情" });
  await expect.element(hostDialog).toBeVisible();
  const wsSpan = hostDialog
    .getByText("WebSocket", { exact: true })
    .element() as HTMLElement;
  expect(wsSpan.parentElement?.querySelector(".invisible")).toBeNull();
  await userEvent.keyboard("{Escape}");

  // Raise urgent attention, open inbox, clear attention, and verify Escape focus handoff.
  socket.frame({
    type: "updated",
    session: session({
      id: "shellbbb",
      title: "日常任务",
      status: "waiting",
      attention: {
        kind: "permission",
        source: "agent",
        since: "2026-11-05T12:00:00Z",
      },
    }),
  } as never);

  await expect.element(inboxTrigger()).toBeVisible();
  await inboxTrigger().click();
  const inbox = page.getByRole("dialog", { name: "待我处理", exact: true });
  await expect.element(inbox).toBeVisible();

  socket.frame({
    type: "updated",
    session: session({
      id: "shellbbb",
      title: "日常任务",
      status: "running",
    }),
  } as never);
  await expect
    .element(inbox.getByText("当前没有等待处理的会话。"))
    .toBeVisible();

  await userEvent.keyboard("{Escape}");
  await expect.poll(() => inboxTrigger().elements().length).toBe(0);
  expect(sidebar().element().contains(document.activeElement)).toBe(true);
});

test("sidebar brand returns home, session switcher stays pinned across routes, and footer controls share one row and height", async () => {
  await page.viewport(1280, 800);
  localStorage.setItem("webmux.sidebarWidth", "256");
  history.replaceState(null, "", "/s/shellaaa");
  let switcherCalls = 0;
  render(ShellHarness, {
    drafts: createTerminalDrafts(),
    onSessionSwitcher: () => {
      switcherCalls += 1;
    },
  });
  await snapshot([session({ id: "shellaaa", title: "主终端" })]);

  // Session switcher stays pinned in the sidebar header even on session routes.
  const switcher = sidebar().getByRole("button", {
    name: "快速切换会话",
    exact: true,
  });
  await expect.element(switcher).toBeVisible();
  await switcher.click();
  expect(switcherCalls).toBe(1);

  // Footer controls align to a single row with uniform 32px desktop heights.
  const addRepo = sidebar().getByRole("button", {
    name: "添加仓库",
    exact: true,
  });
  const forward = sidebar().getByRole("button", { name: /^端口转发/ });
  const settings = sidebar().getByRole("link", { name: "设置", exact: true });
  const theme = sidebar().getByRole("button", {
    name: /^切换为(?:白色|黑色)主题$/,
  });
  const rects = [addRepo, forward, settings, theme].map((control) =>
    control.element().getBoundingClientRect(),
  );
  for (const rect of rects) {
    expect(Math.round(rect.height)).toBe(32);
    expect(Math.abs(rect.top - rects[0]!.top)).toBeLessThanOrEqual(1);
  }

  // Collapsed rail keeps the session switcher reachable on session routes too.
  await sidebar().getByRole("button", { name: "收起侧栏" }).click();
  const rail = page.getByRole("navigation", { name: "窄栏导航" });
  await expect
    .element(rail.getByRole("button", { name: "快速切换会话", exact: true }))
    .toBeVisible();
  await rail.getByRole("button", { name: "展开侧栏" }).click();

  // Brand link returns to workspace home.
  await sidebar().getByRole("link", { name: "WebMux", exact: true }).click();
  await expect.poll(() => location.pathname).toBe("/");
});
