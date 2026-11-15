import "../../src/styles.css";
import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { render } from "vitest-browser-svelte/pure";
import Terminal from "../../src/pages/Terminal.svelte";
import { createTerminalDrafts } from "../../src/domains/terminal/terminal-draft.svelte";
import { store } from "../../src/platform/api";
import { installChatFixture, socketAt, sockets } from "./chat-fixture";
import { session } from "./shell-fixture";
import ShellHarness from "./ShellHarness.svelte";
import { clearSessionListSnapshot } from "../../src/hooks/sessions/session-list.svelte";

installChatFixture([]);
const info = session({ id: "coreobs2", title: "Core terminal" });
let originalPath: string;
beforeEach(() => {
  originalPath = location.pathname + location.search;
  history.replaceState(null, "", `/s/${info.id}`);
  clearSessionListSnapshot();
  store(`layout.${info.id}`, "");
  store(`layout.${info.id}.active`, "");
});
afterEach(() => {
  history.replaceState(null, "", originalPath);
  store(`layout.${info.id}`, "");
  store(`layout.${info.id}.active`, "");
  clearSessionListSnapshot();
});

test.each([false, true])(
  "zero-feature raw owner retains its terminal and draft through list removal (shell=%s)",
  async (withShell) => {
    const drafts = createTerminalDrafts();
    drafts.edit(drafts.get(info.id), "keep the original draft");
    if (withShell) await render(ShellHarness, { drafts, terminalPage: true });
    else
      await render(Terminal, {
        id: info.id,
        drafts,
        onUploadNotice: () => {},
      });
    const terminal = await socketAt(`/ws/s/${info.id}`);
    terminal.binary(2, info);
    const feed = await socketAt("/ws/sessions");
    feed.frame({ type: "snapshot", sessions: [info], forwards: [] } as never);
    const owner = document.querySelector(`[data-session-id="${info.id}"]`);
    expect(owner).not.toBeNull();
    feed.frame({
      type: "updated",
      session: { ...info, title: "Updated core terminal" },
    } as never);
    expect(document.querySelector(`[data-session-id="${info.id}"]`)).toBe(
      owner,
    );
    expect(drafts.get(info.id).text).toBe("keep the original draft");
    expect(sockets.map((socket) => socket.path).sort()).toEqual(
      [`/ws/s/${info.id}`, "/ws/sessions"].sort(),
    );
    expect(
      vi
        .mocked(fetch)
        .mock.calls.some(([input]) =>
          /\/api\/(repositories|launch-configs|hybrid-sessions|forwards|voice|chat)(?:\/|\?|$)/.test(
            String(input),
          ),
        ),
    ).toBe(false);
    expect(
      page.getByRole("button", { name: "进入 VR", exact: true }).elements(),
    ).toEqual([]);
    feed.frame({ type: "removed", id: info.id } as never);
    terminal.drop(4404);
    await expect
      .poll(() => document.querySelector(`[data-session-id="${info.id}"]`))
      .toBe(owner);
    expect(drafts.get(info.id).text).toBe("keep the original draft");
    expect(location.pathname).toBe(`/s/${info.id}`);
  },
);

test("zero-feature home consumes core snapshot, update and removal observations", async () => {
  history.replaceState(null, "", "/");
  await render(ShellHarness, { drafts: createTerminalDrafts() });
  const feed = await socketAt("/ws/sessions");
  feed.frame({ type: "snapshot", sessions: [info], forwards: [] } as never);
  const list = page.getByRole("list", { name: "会话列表", exact: true });
  await expect.element(list).toMatchTextContent(/Core terminal/);
  feed.frame({
    type: "updated",
    session: { ...info, title: "Updated core terminal" },
  } as never);
  await expect.element(list).toMatchTextContent(/Updated core terminal/);
  feed.frame({ type: "removed", id: info.id } as never);
  await expect.element(list).not.toBeInTheDocument();
  await expect
    .element(page.getByText("暂无会话。新建会话以启动终端。", { exact: true }))
    .toBeVisible();
});
