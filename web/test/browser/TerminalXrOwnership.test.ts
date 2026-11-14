import "../../src/styles.css";
import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { render } from "vitest-browser-svelte/pure";
import Terminal from "../../src/pages/Terminal.svelte";
import ShellHarness from "./ShellHarness.svelte";
import type { SessionInfo } from "../../src/contracts/schemas";
import type { XrWorkspaceOptions } from "../../src/domains/terminal/xr-workspace";
import {
  createXrViewState,
  type XrViewState,
} from "../../src/hooks/terminal/xr-view-state";
import { createTerminalDrafts } from "../../src/domains/terminal/terminal-draft.svelte";
import {
  splitPane,
  type PaneLayout,
} from "../../src/domains/terminal/terminal-layout";
import { store } from "../../src/platform/api";
import { chat, record, session } from "./shell-fixture";
import {
  chatState,
  installChatFixture,
  respond,
  snapshot,
  socketAt,
  sockets,
} from "./chat-fixture";

const scene = vi.hoisted(() => ({ start: vi.fn(), dispose: vi.fn() }));
vi.mock("../../src/hooks/terminal/xr-workspace", () => ({
  startXrWorkspace: scene.start,
}));
installChatFixture();

const a = chat({ id: "xrownera", title: "Chat A", generation: "" });
const b = chat({ id: "xrownerb", title: "Chat B", generation: "" });
const previousXr = Object.getOwnPropertyDescriptor(navigator, "xr");
let options: XrWorkspaceOptions;
let xr: XRSession;
let originalPath: string;

beforeEach(async () => {
  await page.viewport(1280, 900);
  originalPath = location.pathname + location.search;
  history.replaceState(null, "", `/s/${a.id}`);
  store(`layout.${a.id}`, "");
  store(`layout.${a.id}.active`, "");
  const target = new EventTarget();
  xr = Object.assign(target, {
    end: vi.fn(async () => target.dispatchEvent(new Event("end"))),
  }) as unknown as XRSession;
  Object.defineProperty(navigator, "xr", {
    configurable: true,
    value: {
      isSessionSupported: async () => true,
      requestSession: async () => xr,
    },
  });
  scene.dispose.mockClear();
  scene.start
    .mockReset()
    .mockImplementation(
      async (_session: XRSession, next: XrWorkspaceOptions) => {
        options = next;
        return { dispose: scene.dispose };
      },
    );
});

afterEach(() => {
  history.replaceState(null, "", originalPath);
  store(`layout.${a.id}`, "");
  store(`layout.${a.id}.active`, "");
  if (previousXr) Object.defineProperty(navigator, "xr", previousXr);
  else Reflect.deleteProperty(navigator, "xr");
});

async function attach(info: SessionInfo) {
  const terminal = await socketAt(`/ws/s/${info.id}`);
  terminal.binary(2, info);
  const stream = await socketAt(`/ws/s/${info.id}/chat`);
  stream.frame(snapshot(chatState()));
  return { terminal, stream };
}

async function mount(records = [a, b], saved?: PaneLayout) {
  await page.viewport(saved ? 600 : 1280, 900);
  for (const info of records)
    respond(new RegExp(`^GET /api/sessions/${info.id}$`), () =>
      Response.json(info),
    );
  if (saved) store(`layout.${a.id}`, JSON.stringify(saved));
  const drafts = createTerminalDrafts();
  const view = await render(Terminal, {
    id: a.id,
    drafts,
    onUploadNotice: () => {},
  });
  await attach(a);
  await expect
    .element(page.getByRole("textbox", { name: "消息" }))
    .toBeVisible();
  const pane = page.getByRole("region", { name: "终端窗格 1", exact: true });
  if (saved) {
    await pane.getByRole("button", { name: "会话菜单", exact: true }).click();
    await page.getByRole("menuitem", { name: "进入 VR", exact: true }).click();
  } else {
    await pane.getByRole("button", { name: "进入 VR", exact: true }).click();
  }
  await expect.poll(() => scene.start.mock.calls.length).toBe(1);
  await options.observe(a.id);
  return { view, drafts };
}

async function membership(records: SessionInfo[], index = 0) {
  const stream = await socketAt("/ws/sessions", index);
  stream.onmessage?.(
    new MessageEvent("message", {
      data: JSON.stringify({
        type: "snapshot",
        sessions: records,
        forwards: [],
      }),
    }),
  );
}

async function removed(id: string, index = 0) {
  const stream = await socketAt("/ws/sessions", index);
  stream.onmessage?.(
    new MessageEvent("message", {
      data: JSON.stringify({ type: "removed", id }),
    }),
  );
}

function source(id: string) {
  const found = options.sources().find((current) => current.id === id);
  if (!found) throw new Error(`Missing mounted owner ${id}`);
  return found;
}

test("VR-only original editor receives physical and VR input without changing 2D", async () => {
  const { view, drafts } = await mount();
  await membership([a, b]);
  const ownerA = source(a.id);
  const route = location.pathname;
  await options.observe(b.id);
  await attach(b);
  const ownerB = source(b.id);
  await expect.poll(() => ownerB.canInput("chat", "chat")).toBe(true);
  expect(document.querySelectorAll(".split-terminal")).toHaveLength(1);
  expect(
    document
      .querySelector(".split-terminal[data-active='true']")
      ?.getAttribute("data-session-id"),
  ).toBe(a.id);
  expect(location.pathname).toBe(route);
  expect(document.activeElement?.closest(".session-terminal")).toBeNull();

  let selected = "B1";
  await ownerB.focus("chat", "chat", () => selected === "B1");
  expect(
    document.activeElement
      ?.closest("[data-session-id]")
      ?.getAttribute("data-session-id"),
  ).toBe(b.id);
  await userEvent.keyboard("physical-B");
  ownerB.input("!");
  expect(drafts.get(b.id).text).toBe("physical-B!");
  expect(drafts.get(a.id).text).toBe("");
  expect(
    document
      .querySelector(".split-terminal[data-active='true']")
      ?.getAttribute("data-session-id"),
  ).toBe(a.id);

  // A pending old focus cannot win after another view is selected.
  const stale = ownerB.focus("chat", "chat", () => selected === "B1");
  selected = "A2";
  ownerB.blur();
  await ownerA.focus("chat", "chat", () => selected === "A2");
  await stale;
  await userEvent.keyboard("only-A");
  expect(drafts.get(a.id).text).toBe("only-A");
  expect(drafts.get(b.id).text).toBe("physical-B!");

  ownerA.blur();
  selected = "closed";
  await userEvent.keyboard("ignored");
  ownerA.input("ignored");
  expect(drafts.get(a.id).text).toBe("only-A");
  expect(document.activeElement?.closest(".session-terminal")).toBeNull();

  // Observation -> 2D is a presentation transfer, not a second owner.
  view.component.reopenSession(b.id);
  await expect
    .poll(() => document.querySelectorAll(".split-terminal").length)
    .toBe(2);
  options.unobserve(b.id);
  await expect.poll(() => options.sources().includes(ownerB)).toBe(true);
  expect(
    sockets.filter((socket) => socket.path === `/ws/s/${b.id}/chat`),
  ).toHaveLength(1);
  expect(drafts.get(b.id).text).toBe("physical-B!");
  options.exit();
});

test("delayed authoritative membership preserves observations; deleting entry leaves B active", async () => {
  const { drafts } = await mount();
  expect(options.ready()).toBe(false);
  await options.observe(b.id);
  expect(source(b.id).primaryKind).toBeUndefined();
  await attach(b);
  const ownerB = source(b.id);
  await expect.poll(() => ownerB.canInput("chat", "chat")).toBe(true);
  expect(ownerB.primaryKind).toBe("chat");
  await membership([a, b]);
  await expect.poll(() => options.ready()).toBe(true);
  expect(options.sources().map((owner) => owner.id)).toEqual([a.id, b.id]);
  respond(new RegExp(`^GET /api/sessions/${a.id}$`), () =>
    Response.json({ error: "not found" }, { status: 404 }),
  );
  const deleted = await socketAt(`/ws/s/${a.id}`);
  deleted.drop(4404);
  await membership([b]);
  await expect
    .poll(() => options.sources().map((owner) => owner.id))
    .toEqual([b.id]);
  expect(xr.end).not.toHaveBeenCalled();
  expect(document.querySelectorAll(".split-terminal")).toHaveLength(0);
  expect(location.pathname).toBe(`/s/${a.id}`);
  await ownerB.focus("chat", "chat");
  await userEvent.keyboard("surviving-B");
  expect(drafts.get(b.id).text).toBe("surviving-B");
  options.exit();
  await expect.poll(() => location.pathname).not.toBe(`/s/${a.id}`);
  expect(xr.end).toHaveBeenCalledOnce();
});

test("readonly TUI history cannot focus or route chat text to the terminal", async () => {
  const tui = {
    ...b,
    view: "terminal" as const,
    generation: "0123456789abcdef",
  };
  const { drafts } = await mount([a, tui]);
  await membership([a, tui]);
  await options.observe(tui.id);
  await attach(tui);
  const owner = source(tui.id);
  await expect
    .poll(() =>
      owner.panels().some((panel) => panel.title === "聊天历史 · 只读"),
    )
    .toBe(true);
  expect(owner.canInput("chat", "chat")).toBe(false);
  await owner.focus("chat", "chat");
  owner.input("must-not-enter-TUI");
  await userEvent.keyboard("also-ignored");
  expect(drafts.get(tui.id).text).toBe("");
  expect(document.activeElement?.closest(".session-terminal")).toBeNull();
  expect(
    owner.panels().find((panel) => panel.id === "chat")?.lines[0]?.text,
  ).toContain("暂无可用聊天历史");
  options.exit();
});

test("eight 2D panes do not consume VR observation capacity or silently discard splits", async () => {
  const others = Array.from({ length: 7 }, (_, i) =>
    chat({
      id: `xrowner${String.fromCharCode(99 + i)}`,
      title: `Chat ${i}`,
      generation: "",
    }),
  );
  let layout: PaneLayout = { kind: "pane", sessionId: a.id };
  const targets = [a.id];
  for (const [index, info] of others.entries()) {
    const target = targets.shift()!;
    layout = splitPane(
      layout,
      target,
      info.id,
      index === 1 || index === 2 ? "vertical" : "horizontal",
    );
    targets.push(target, info.id);
  }
  await mount([a, b, ...others], layout);
  await membership([a, b, ...others]);
  await options.observe(b.id);
  await attach(b);
  await expect.poll(() => source(b.id).canInput("chat", "chat")).toBe(true);
  expect(document.querySelectorAll(".split-terminal")).toHaveLength(8);
  expect(
    document
      .querySelector(".split-terminal[data-active='true']")
      ?.getAttribute("data-session-id"),
  ).toBe(a.id);
  await options.reveal(b.id);
  expect(xr.end).not.toHaveBeenCalled();
  expect(document.querySelectorAll(".split-terminal")).toHaveLength(8);
  await expect
    .element(
      page.getByText(
        "无法返回会话：最多同时打开 8 个窗格；请先关闭一个再打开此会话。",
        { exact: true },
      ),
    )
    .toBeVisible();
  options.exit();
});

test.each([
  { reason: "config_missing", title: "启动配置已删除" },
  { reason: "session_missing", title: "找不到原来的 agent 会话" },
])(
  "archived 2D recovery stays mounted through $reason with a ready empty live list",
  async ({ reason, title }) => {
    const archived = record({
      id: a.id,
      title: "Archived recovery",
      view: "terminal",
    });
    respond(new RegExp(`^GET /api/sessions/${a.id}$`), () =>
      Response.json(archived),
    );
    respond(/^GET \/api\/sessions$/, ({ query }) =>
      Response.json(query?.includes("archived") ? [archived] : []),
    );
    respond(new RegExp(`^POST /api/sessions/${a.id}/resume$`), () =>
      Response.json(
        {
          error: title,
          reason,
          ...(reason === "session_missing"
            ? {
                candidates: [
                  {
                    agentSessionId: "candidate-1",
                    title: "Candidate",
                    updatedAt: archived.archive.updatedAt,
                  },
                ],
              }
            : {}),
        },
        { status: 409 },
      ),
    );
    const drafts = createTerminalDrafts();
    await render(ShellHarness, { drafts, terminalPage: true });
    await membership([]);
    const terminal = await socketAt(`/ws/s/${a.id}`);
    terminal.drop(4404);
    const recovery = page.getByRole("status", { name: "恢复会话" });
    await expect.element(recovery).toBeVisible();
    expect(location.pathname).toBe(`/s/${a.id}`);
    expect(document.querySelectorAll(".split-terminal")).toHaveLength(1);
    await recovery.getByRole("button", { name: "恢复", exact: true }).click();
    await expect
      .element(page.getByRole("alertdialog", { name: title }))
      .toBeVisible();
    expect(location.pathname).toBe(`/s/${a.id}`);
  },
);

test.each([
  { status: 503, response: { error: "store unavailable" } },
  { status: 200, response: a },
])(
  "canonical lookup $status does not falsely declare a 2D owner deleted",
  async ({ status, response }) => {
    const lookup = vi.fn(() => Response.json(response, { status }));
    respond(new RegExp(`^GET /api/sessions/${a.id}$`), lookup);
    const drafts = createTerminalDrafts();
    drafts.edit(drafts.get(a.id), "keep pending recovery draft");
    await render(ShellHarness, { drafts, terminalPage: true });
    await membership([]);
    const terminal = await socketAt(`/ws/s/${a.id}`);
    terminal.drop(4404);
    await expect.poll(() => lookup.mock.calls.length).toBeGreaterThan(0);
    await expect
      .element(page.getByRole("region", { name: "终端窗格 1", exact: true }))
      .toBeVisible();
    expect(location.pathname).toBe(`/s/${a.id}`);
    expect(drafts.get(a.id).text).toBe("keep pending recovery draft");
  },
);

test("a postcommit removed event invalidates cached 2D recovery after the live row already left", async () => {
  const archived = record({ id: a.id, title: "Before forget commit" });
  const lookup = vi.fn(() => Response.json(archived));
  respond(new RegExp(`^GET /api/sessions/${a.id}$`), lookup);
  respond(/^GET \/api\/sessions$/, ({ query }) =>
    Response.json(query?.includes("archived") ? [archived] : [a]),
  );
  const drafts = createTerminalDrafts();
  await render(ShellHarness, { drafts, terminalPage: true });
  await membership([a]);
  // Runtime teardown publishes before the archive transaction commits.
  await removed(a.id);
  const terminal = await socketAt(`/ws/s/${a.id}`);
  terminal.drop(4404);
  await expect
    .element(page.getByRole("status", { name: "恢复会话" }))
    .toBeVisible();
  expect(location.pathname).toBe(`/s/${a.id}`);
  const readsBeforeCommit = lookup.mock.calls.length;
  lookup.mockImplementation(() =>
    Response.json({ error: "not found" }, { status: 404 }),
  );
  // No second live-list change occurs: only the identical final event.
  await removed(a.id);
  await expect
    .poll(() => lookup.mock.calls.length)
    .toBeGreaterThan(readsBeforeCommit);
  await expect.poll(() => location.pathname).not.toBe(`/s/${a.id}`);
  expect(
    document.querySelector(`.session-owner[data-session-id="${a.id}"]`),
  ).toBeNull();
  await expect
    .element(page.getByRole("status", { name: "恢复会话" }))
    .not.toBeInTheDocument();
});

test("a slow precommit archive response cannot restore A after final 404 while B survives XR", async () => {
  const { drafts } = await mount();
  await membership([a, b]);
  await options.observe(b.id);
  await attach(b);
  const ownerB = source(b.id);
  await expect.poll(() => ownerB.canInput("chat", "chat")).toBe(true);
  const archived = record({ id: a.id, title: "Stale precommit archive" });
  const slow = Promise.withResolvers<Response>();
  const early = vi.fn(() => slow.promise);
  respond(new RegExp(`^GET /api/sessions/${a.id}$`), early);
  await removed(a.id);
  await expect.poll(() => early.mock.calls.length).toBeGreaterThan(0);
  const final = vi.fn(() =>
    Response.json({ error: "not found" }, { status: 404 }),
  );
  respond(new RegExp(`^GET /api/sessions/${a.id}$`), final);
  await removed(a.id);
  await expect.poll(() => final.mock.calls.length).toBeGreaterThan(0);
  await expect
    .poll(() => options.sources().map((owner) => owner.id))
    .toEqual([b.id]);
  expect(xr.end).not.toHaveBeenCalled();
  expect(document.querySelectorAll(".split-terminal")).toHaveLength(0);
  expect(location.pathname).toBe(`/s/${a.id}`);

  // The transport deliberately ignores AbortSignal, as an already received
  // response may still finish parsing after the final lookup has won.
  const response = Response.json(archived);
  const parsed = vi.spyOn(response, "json");
  slow.resolve(response);
  await expect.poll(() => parsed.mock.calls.length).toBeGreaterThan(0);
  await ownerB.focus("chat", "chat");
  await userEvent.keyboard("B-after-forget");
  expect(drafts.get(b.id).text).toBe("B-after-forget");
  expect(options.sources().map((owner) => owner.id)).toEqual([b.id]);
  expect(
    document.querySelector(`.session-owner[data-session-id="${a.id}"]`),
  ).toBeNull();
  options.exit();
  await expect.poll(() => location.pathname).not.toBe(`/s/${a.id}`);
  await expect
    .element(page.getByRole("status", { name: "恢复会话" }))
    .not.toBeInTheDocument();
});

test("API-created 2D owner can enter XR before its delayed create event or reconnect snapshot", async () => {
  const created = session({ id: "newpanea", title: "New shell" });
  let views!: XrViewState;
  const failures: string[] = [];
  scene.start.mockImplementation(
    async (_session: XRSession, next: XrWorkspaceOptions) => {
      options = next;
      views = createXrViewState(
        next,
        undefined,
        () => {},
        (message) => failures.push(message),
      );
      await views.initialize();
      views.refresh();
      return { dispose: () => views.dispose() };
    },
  );
  respond(new RegExp(`^GET /api/sessions/${a.id}$`), () => Response.json(a));
  respond(new RegExp(`^GET /api/sessions/${created.id}$`), () =>
    Response.json(created),
  );
  respond(/^POST \/api\/sessions$/, () =>
    Response.json({ session: created, configWarnings: [] }),
  );
  const drafts = createTerminalDrafts();
  await render(ShellHarness, { drafts, terminalPage: true });
  await membership([a]);
  await attach(a);
  await page
    .getByRole("region", { name: "终端窗格 1", exact: true })
    .getByRole("button", { name: "分屏", exact: true })
    .click();
  await page.getByRole("menuitem", { name: /左右分屏 · 新建 Shell/ }).click();
  const terminal = await socketAt(`/ws/s/${created.id}`);
  terminal.binary(2, created);
  await expect
    .poll(() => document.querySelectorAll(".split-terminal").length)
    .toBe(2);
  const originalHost = document.querySelector(
    `[data-session-id="${created.id}"]`,
  );

  // The authoritative live list is still stale while its connection returns.
  const list = await socketAt("/ws/sessions");
  const reconnectIndex = sockets.filter(
    (socket) => socket.path === "/ws/sessions",
  ).length;
  list.drop(1006);
  await page.getByRole("button", { name: "放大窗格", exact: true }).click();
  await page
    .getByRole("region", { name: "终端窗格 2", exact: true })
    .getByRole("button", { name: "进入 VR", exact: true })
    .click();
  await expect.poll(() => scene.start.mock.calls.length).toBe(1);
  expect(options.ready()).toBe(true);
  expect(options.available().some((item) => item.id === created.id)).toBe(true);
  await options.observe(created.id);
  const owner = source(created.id);
  expect(owner.primaryKind).toBe("terminal");
  await expect
    .poll(() =>
      views.layouts.some(
        (layout) => layout.sessionId === created.id && layout.visible,
      ),
    )
    .toBe(true);

  await membership([a], reconnectIndex);
  expect(options.available().some((item) => item.id === created.id)).toBe(true);
  expect(source(created.id)).toBe(owner);
  views.refresh();
  expect(
    views.layouts.some(
      (layout) => layout.sessionId === created.id && layout.visible,
    ),
  ).toBe(true);
  const live = await socketAt("/ws/sessions", reconnectIndex);
  live.onmessage?.(
    new MessageEvent("message", {
      data: JSON.stringify({ type: "created", session: created }),
    }),
  );
  await expect
    .poll(() => options.available().find((item) => item.id === created.id))
    .toMatchObject({
      id: created.id,
      title: created.title,
      generation: created.generation,
    });
  views.refresh();
  expect(
    views.layouts.some(
      (layout) => layout.sessionId === created.id && layout.visible,
    ),
  ).toBe(true);
  expect(failures).toEqual([]);
  expect(document.querySelector(`[data-session-id="${created.id}"]`)).toBe(
    originalHost,
  );
  expect(source(created.id)).toBe(owner);
  expect(
    sockets.filter((socket) => socket.path === `/ws/s/${created.id}`),
  ).toHaveLength(1);
  options.exit();
});
