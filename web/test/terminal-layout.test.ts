import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import type { SessionInfo } from "../src/contracts/schemas";
import {
  MAX_PANES,
  createSplitShellSession,
  layoutGeometry,
  paneIds,
  removePane,
  resizeSplit,
  restoreLayout,
  splitPane,
  type PaneLayout,
} from "../src/domains/terminal/terminal-layout";

const first: PaneLayout = { kind: "pane", sessionId: "aaaaaaaa" };
const nested = () =>
  splitPane(
    splitPane(first, "aaaaaaaa", "bbbbbbbb", "horizontal"),
    "bbbbbbbb",
    "cccccccc",
    "vertical",
  );

describe("terminal pane layout", () => {
  it("tiles nested panes without overlap and preserves total dimensions", () => {
    const geometry = layoutGeometry(nested(), 1001, 801);
    expect(geometry.panes).toEqual([
      { sessionId: "aaaaaaaa", x: 0, y: 0, width: 500, height: 801 },
      { sessionId: "bbbbbbbb", x: 501, y: 0, width: 500, height: 400 },
      { sessionId: "cccccccc", x: 501, y: 401, width: 500, height: 400 },
    ]);
    expect(
      layoutGeometry(nested(), 0, 0).panes.every(
        (pane) => pane.width >= 0 && pane.height >= 0,
      ),
    ).toBe(true);
  });

  it("collapses only the removed branch and clamps resize ratios", () => {
    const layout = nested();
    expect(paneIds(removePane(layout, "bbbbbbbb")!)).toEqual([
      "aaaaaaaa",
      "cccccccc",
    ]);
    expect(
      layoutGeometry(removePane(layout, "aaaaaaaa")!, 1006, 806).panes[0]!
        .width,
    ).toBe(1006);
    expect(removePane(first, "aaaaaaaa")).toBeUndefined();
    if (layout.kind !== "split") throw new Error("Expected split");
    const resized = resizeSplit(layout, layout.id, 10);
    expect(resized.kind === "split" && resized.ratio).toBe(0.9);
    expect(resizeSplit(layout, layout.id, -1)).toMatchObject({ ratio: 0.1 });
  });

  it("bounds pane creation and never attaches the same PTY twice", () => {
    let layout: PaneLayout = first;
    for (const letter of "bcdefghi")
      layout = splitPane(layout, "aaaaaaaa", letter.repeat(8), "horizontal");
    expect(paneIds(layout)).toHaveLength(MAX_PANES);
    expect(splitPane(layout, "aaaaaaaa", "zzzzzzzz", "vertical")).toBe(layout);
    expect(splitPane(first, "aaaaaaaa", "aaaaaaaa", "vertical")).toBe(first);
  });

  it("restores valid layouts and rejects corrupt, duplicate and excessive saved trees", () => {
    const layout = nested();
    expect(restoreLayout(JSON.stringify(layout), "dddddddd")).toEqual(layout);
    const invalid = [
      "null",
      "{}",
      "broken",
      JSON.stringify({ ...layout, ratio: 100 }),
      JSON.stringify({
        kind: "split",
        id: "duplicate",
        direction: "horizontal",
        ratio: 0.5,
        first,
        second: first,
      }),
      JSON.stringify({ kind: "pane", sessionId: "invalid" }),
      " ".repeat(4097),
    ];
    let deep: PaneLayout = first;
    for (let i = 0; i < 20; i++)
      deep = {
        kind: "split",
        id: String(i),
        direction: "vertical",
        ratio: 0.5,
        first,
        second: deep,
      };
    invalid.push(JSON.stringify(deep));
    for (const value of invalid)
      expect(restoreLayout(value, "aaaaaaaa")).toEqual(first);
  });
});

describe("createSplitShellSession", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  const baseSession: SessionInfo = {
    id: "aaaaaaaa",
    generation: "0123456789abcdef",
    title: "Shell",
    cwd: "/work/project/packages/app",
    cmd: ["/bin/zsh", "-l", "-i"],
    status: "running",
    exitCode: null,
    preview: "",
    createdAt: "2026-11-20T00:00:00Z",
    lastActiveAt: "2026-11-20T00:00:00Z",
    cols: 80,
    rows: 24,
    clients: 1,
    workspace: {
      repositoryId: "repo1",
      worktreeId: "wt1",
      worktreePath: "/work/project",
    },
    attention: null,
    reportedCwd: null,
    state: "active",
    views: ["terminal"],
    view: "terminal",
    transition: null,
    agent: null,
    listeningPorts: [],
  };

  it("omits cmd so the server starts its default login shell while preserving cwd and workspace", async () => {
    const created: SessionInfo = { ...baseSession, id: "bbbbbbbb" };
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ session: created, configWarnings: [] }), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const session = await createSplitShellSession(baseSession);

    expect(session.id).toBe("bbbbbbbb");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/sessions");
    expect(init?.method).toBe("POST");
    expect(JSON.parse(String(init?.body))).toEqual({
      cwd: "/work/project/packages/app",
      workspace: { repositoryId: "repo1", worktreeId: "wt1" },
    });
  });

  it("omits both cmd and workspace for direct sessions", async () => {
    const direct: SessionInfo = { ...baseSession, workspace: null };
    const created: SessionInfo = { ...direct, id: "cccccccc" };
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ session: created, configWarnings: [] }), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await createSplitShellSession(direct);

    const [, init] = fetchMock.mock.calls[0]!;
    expect(JSON.parse(String(init?.body))).toEqual({
      cwd: "/work/project/packages/app",
    });
  });
});
