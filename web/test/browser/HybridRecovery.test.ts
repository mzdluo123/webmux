import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import HybridRecovery from "../../src/components/sessions/HybridRecovery.svelte";
import type { ArchivedSession, SessionInfo } from "../../src/contracts/schemas";

afterEach(async () => {
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  }
});

function archived(overrides: Partial<ArchivedSession> = {}): ArchivedSession {
  return {
    id: "sessaaaa",
    generation: "",
    title: "Resume demo",
    cwd: "/tmp/demo",
    cmd: ["agy"],
    status: "exited",
    exitCode: null,
    preview: "",
    createdAt: "2026-11-21T00:00:00Z",
    lastActiveAt: "2026-11-21T00:00:00Z",
    cols: 0,
    rows: 0,
    clients: 0,
    workspace: null,
    attention: null,
    reportedCwd: null,
    state: "interrupted",
    views: ["terminal", "chat"],
    view: "chat",
    transition: null,
    agent: {
      name: "Antigravity",
      title: "Antigravity",
      icon: "agy",
      sessionId: "agent-sess-1",
    },
    archive: {
      configMissing: false,
      previousAgentSessionIds: [],
      queuedCount: 0,
      resumeOnOpen: true,
      interruptedAt: "2026-11-21T00:00:00Z",
      updatedAt: "2026-11-21T00:00:00Z",
    },
    listeningPorts: [],
    ...overrides,
  };
}

function resumedSession(id: string): SessionInfo {
  return {
    id,
    generation: "",
    title: "Resume demo",
    cwd: "/tmp/demo",
    cmd: ["agy"],
    status: "running",
    exitCode: null,
    preview: "",
    createdAt: "2026-11-21T00:00:00Z",
    lastActiveAt: "2026-11-21T00:00:00Z",
    cols: 80,
    rows: 24,
    clients: 0,
    workspace: null,
    attention: null,
    reportedCwd: null,
    state: "active",
    views: ["terminal", "chat"],
    view: "terminal",
    transition: null,
    agent: {
      name: "Antigravity",
      title: "Antigravity",
      icon: "agy",
      sessionId: "agent-sess-1",
    },
    listeningPorts: [],
  };
}

test("auto-resuming session renders full-pane loading view without resume button and falls back on error", async () => {
  let rejectFetch!: (reason?: unknown) => void;
  vi.stubGlobal(
    "fetch",
    vi.fn(
      () =>
        new Promise<Response>((_, reject) => {
          rejectFetch = reject;
        }),
    ),
  );
  const busyStates: boolean[] = [];
  let claimed = false;
  const props = {
    record: archived(),
    autoResuming: true,
    claimAutoResume: () => {
      if (claimed) return false;
      claimed = true;
      return true;
    },
    onbusy: (active: boolean) => busyStates.push(active),
    onresumed: vi.fn(),
    leave: vi.fn(),
  };
  const view = await render(HybridRecovery, props);

  const status = page.getByRole("status", { name: "恢复会话" });
  await expect.element(status).toBeVisible();
  await expect
    .element(page.getByRole("heading", { name: "正在恢复聊天…" }))
    .toBeVisible();
  await expect
    .element(page.getByText("正在连接 Antigravity 并加载会话记录"))
    .toBeVisible();
  await expect
    .element(page.getByRole("button", { name: "恢复" }))
    .not.toBeInTheDocument();
  expect(busyStates).toContain(true);

  await view.rerender({ ...props, autoResuming: false });
  rejectFetch(new Error("agent unreachable"));
  await expect
    .element(page.getByRole("alert"))
    .toMatchTextContent(/agent unreachable/);
  await expect
    .element(page.getByRole("button", { name: "恢复" }))
    .toBeVisible();
  expect(busyStates.at(-1)).toBe(false);
});

test("manual resume enters full-pane loading view and notifies on success", async () => {
  let resolveFetch!: (value: Response) => void;
  vi.stubGlobal(
    "fetch",
    vi.fn(
      () =>
        new Promise<Response>((resolve) => {
          resolveFetch = resolve;
        }),
    ),
  );
  const onresumed = vi.fn();
  const busyStates: boolean[] = [];
  await render(HybridRecovery, {
    record: archived({
      state: "closed",
      view: "terminal",
      archive: {
        configMissing: false,
        previousAgentSessionIds: [],
        queuedCount: 0,
        resumeOnOpen: false,
        interruptedAt: null,
        updatedAt: "2026-11-21T00:00:00Z",
      },
    }),
    autoResuming: false,
    claimAutoResume: () => true,
    onbusy: (active: boolean) => busyStates.push(active),
    onresumed,
    leave: vi.fn(),
  });

  const resumeButton = page.getByRole("button", { name: "恢复" });
  await expect.element(resumeButton).toBeVisible();
  await resumeButton.click();

  await expect
    .element(page.getByRole("heading", { name: "正在恢复终端…" }))
    .toBeVisible();
  await expect.element(resumeButton).not.toBeInTheDocument();
  expect(busyStates).toContain(true);

  const session = resumedSession("sessaaaa");
  resolveFetch(
    new Response(JSON.stringify(session), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }),
  );
  await vi.waitFor(() => {
    expect(onresumed).toHaveBeenCalledWith(session);
  });
});
