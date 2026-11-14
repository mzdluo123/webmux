import { describe, expect, test, vi } from "vite-plus/test";
import {
  defaultXrPanel,
  XR_PANEL_LIMIT,
} from "../src/domains/terminal/xr-layout";
import type { XrPanelLayout } from "../src/domains/terminal/xr-layout";
import type {
  XrSessionSource,
  XrTextPanel,
  XrWorkspaceOptions,
} from "../src/domains/terminal/xr-workspace";
import { createXrViewState } from "../src/hooks/terminal/xr-view-state";

function fixture(initial: XrPanelLayout[] | undefined = []) {
  const chat: XrTextPanel = {
    id: "chat",
    kind: "chat",
    title: "Chat",
    lines: [{ text: "Live reply" }],
  };
  const extras: XrTextPanel[] = [
    { id: "file", kind: "code", title: "Code", lines: [] },
    { id: "draft", kind: "chat", title: "Draft", lines: [] },
    { id: "notice", kind: "chat", title: "Notice", lines: [] },
  ];
  const sources: XrSessionSource[] = ["a", "b", "c"].map((id) => ({
    id,
    title: id,
    status: "connected",
    terminal: undefined,
    graphicsUnsupported: false,
    primaryKind: "chat" as const,
    modifiers: { ctrl: false, alt: false, shift: false },
    shiftLocked: false,
    toggleModifier: vi.fn(),
    resetModifiers: vi.fn(),
    lockModifier: vi.fn(),
    panels: vi.fn(() => [chat, ...extras]),
    actions: () => [],
    input: vi.fn(),
    focus: vi.fn(),
    blur: vi.fn(),
    canInput: vi.fn(() => true),
    latest: vi.fn(),
    scroll: vi.fn(),
    resize: () => false,
    setImmersive: vi.fn(),
  }));
  let ready = true;
  let available = sources.map(({ id, title }) => ({ id, title }));
  const options: XrWorkspaceOptions = {
    initialId: "a",
    sources: () => sources,
    available: () => available,
    ready: () => ready,
    observe: vi.fn(async () => {}),
    unobserve: vi.fn(),
    reveal: vi.fn(),
    exit: vi.fn(),
    error: vi.fn(),
  };
  const changed = vi.fn();
  const state = createXrViewState(options, initial, changed, options.error);
  return {
    state,
    sources,
    options,
    changed,
    setReady(value: boolean) {
      ready = value;
    },
    setAvailable(ids: string[]) {
      available = ids.map((id) => ({ id, title: id }));
    },
  };
}

function binding(id: string, sessionId = "a", visible = true) {
  return { ...defaultXrPanel(id, sessionId, "chat", 1, "chat"), visible };
}

describe("XR binding lifecycle", () => {
  test("valid empty layouts stay empty, while absent layouts initialize one main view", async () => {
    const empty = fixture();
    await empty.state.initialize();
    empty.state.refresh();
    empty.state.refresh();
    expect(empty.state.layouts).toEqual([]);
    expect(empty.options.observe).not.toHaveBeenCalled();
    const missing = fixture(undefined);
    // The default parameter intentionally represents valid empty; explicitly create the absent document.
    const state = createXrViewState(
      missing.options,
      undefined,
      missing.changed,
      missing.options.error,
    );
    await state.initialize();
    expect(state.layouts).toHaveLength(1);
    expect(state.layouts[0]).toMatchObject({
      sessionId: "a",
      kind: "chat",
      sourceId: "chat",
    });
    expect(missing.options.observe).toHaveBeenCalledExactlyOnceWith("a");
    state.dispose();
  });

  test("restored hidden bindings acquire refs even when already mounted in 2D, and never resurrect closed defaults", async () => {
    const f = fixture([binding("a-old", "a", false), binding("b-old", "b")]);
    await f.state.initialize();
    expect(f.options.observe).toHaveBeenCalledWith("a");
    expect(f.options.observe).toHaveBeenCalledWith("b");
    for (const panel of f.state.layouts) panel.visible = false;
    f.state.reconcile();
    for (let index = 0; index < 3; index++) f.state.refresh();
    expect(f.state.layouts).toHaveLength(2);
    expect(f.state.layouts.every((panel) => !panel.visible)).toBe(true);
    expect(f.options.unobserve).not.toHaveBeenCalled();
    f.state.remove("a");
    expect(f.options.unobserve).toHaveBeenCalledWith("a");
    expect(f.state.layouts.map((panel) => panel.sessionId)).toEqual(["b"]);
  });

  test("delays permanent pruning until the authoritative list is ready, not mounted source membership", async () => {
    const f = fixture([binding("a-old"), binding("b-old", "b")]);
    f.setReady(false);
    f.setAvailable([]);
    await f.state.initialize();
    f.state.refresh();
    expect(f.state.layouts).toHaveLength(2);
    expect(f.options.observe).not.toHaveBeenCalled();
    f.setAvailable(["b"]);
    f.setReady(true);
    f.state.refresh();
    await Promise.all(f.state.pending.values());
    expect(f.state.layouts.map((panel) => panel.sessionId)).toEqual(["b"]);
    expect(f.options.observe).toHaveBeenCalledExactlyOnceWith("b");
    expect(f.sources.some((source) => source.id === "a")).toBe(true);
  });

  test("coalesces concurrent observes, reserves capacity and rolls back failures without touching old views or selection", async () => {
    const f = fixture(
      Array.from({ length: XR_PANEL_LIMIT - 1 }, (_, index) =>
        binding(`a-${index}`),
      ),
    );
    await f.state.initialize();
    f.state.refresh();
    f.state.select("a-0");
    const { promise, reject } = Promise.withResolvers<void>();
    f.options.observe = vi.fn(() => promise);
    const first = f.state.observe("b");
    const duplicate = f.state.observe("b");
    expect(first).toBe(duplicate);
    await Promise.resolve();
    await f.state.observe("c");
    expect(f.options.observe).toHaveBeenCalledExactlyOnceWith("b");
    expect(f.options.error).toHaveBeenCalledWith(expect.stringContaining("16"));
    expect(f.state.selectedViewId).toBe("a-0");
    reject(new Error("offline"));
    await first;
    expect(f.state.layouts).toHaveLength(15);
    expect(f.state.selectedViewId).toBe("a-0");
    expect(f.options.unobserve).toHaveBeenCalledWith("b");
    expect(f.state.pending.size).toBe(0);
  });

  test("a reserved hidden slot remains budgeted if its original session is removed while joining", async () => {
    const f = fixture([
      binding("hidden-a", "a", false),
      ...Array.from({ length: 15 }, (_, index) => binding(`b-${index}`, "b")),
    ]);
    await f.state.initialize();
    f.state.refresh();
    const { promise, resolve } = Promise.withResolvers<void>();
    f.options.observe = vi.fn(() => promise);
    const joining = f.state.observe("c");
    await Promise.resolve();
    f.state.toggle(f.state.layouts[0]!);
    expect(f.state.layouts[0]!.visible).toBe(false);
    f.state.remove("a");
    f.state.openContent("b", "code", "file");
    expect(f.state.layouts).toHaveLength(15);
    resolve();
    await joining;
    expect(f.state.layouts).toHaveLength(16);
    expect(
      f.state.layouts.filter((panel) => panel.sessionId === "c"),
    ).toHaveLength(1);
  });

  test("only reclaims hidden records and releases a session ref only after its last record is reclaimed", async () => {
    const values = [
      binding("hidden-a", "a", false),
      binding("visible-a"),
      ...Array.from({ length: 14 }, (_, index) => binding(`b-${index}`, "b")),
    ];
    const f = fixture(values);
    await f.state.initialize();
    f.state.refresh();
    await f.state.observe("c");
    expect(f.state.layouts).toHaveLength(16);
    expect(f.state.layouts.some((panel) => panel.id === "hidden-a")).toBe(
      false,
    );
    expect(f.options.unobserve).not.toHaveBeenCalledWith("a");
    const visible = f.state.layouts.find((panel) => panel.id === "visible-a")!;
    f.state.toggle(visible);
    f.state.openContent("b", "code", "file");
    expect(f.state.layouts.some((panel) => panel.sessionId === "a")).toBe(
      false,
    );
    expect(f.options.unobserve).toHaveBeenCalledWith("a");
    const snapshot = [...f.state.layouts];
    const selected = f.state.selectedViewId;
    f.state.split(f.state.layouts.find((panel) => panel.sessionId === "c")!.id);
    expect(f.state.layouts).toEqual(snapshot);
    expect(f.state.selectedViewId).toBe(selected);
  });

  test("split only clones true chat, independently copies geometry and reset keeps every observed main", async () => {
    const f = fixture([binding("history"), binding("b-main", "b")]);
    await f.state.initialize();
    f.state.refresh();
    const original = f.state.layouts[0]!;
    f.state.split(original.id);
    const clone = f.state.layouts[2]!;
    expect(clone).toMatchObject({
      kind: "chat",
      sourceId: "chat",
      sessionId: "a",
    });
    expect(clone.id).not.toBe(original.id);
    expect(clone.position).not.toBe(original.position);
    expect(clone.position[2]).toBeGreaterThan(original.position[2]);
    expect(clone.size).not.toBe(original.size);
    expect(f.state.selectedViewId).toBe(clone.id);
    for (const sourceId of ["draft", "notice"]) {
      f.state.openContent("a", "chat", sourceId);
      f.state.split(f.state.layouts.at(-1)!.id);
    }
    expect(f.state.layouts).toHaveLength(5);
    f.state.reset();
    expect(f.state.layouts).toHaveLength(2);
    expect(
      f.state.layouts.map((panel) => [
        panel.sessionId,
        panel.kind,
        panel.sourceId,
      ]),
    ).toEqual([
      ["a", "chat", "chat"],
      ["b", "chat", "chat"],
    ]);
    expect(f.state.selectedViewId).toBeUndefined();
    expect(f.options.observe).toHaveBeenCalledTimes(2);
    expect(f.options.unobserve).not.toHaveBeenCalled();
  });

  test("reset preserves restored bindings while any canonical primary identity is unknown", async () => {
    const original = [
      binding("a-history"),
      binding("a-tail"),
      binding("b-main", "b"),
    ];
    original[0]!.position = [1.2, -0.4, -3];
    const f = fixture(original);
    f.sources[0] = { ...f.sources[0]!, primaryKind: undefined };
    await f.state.initialize();
    f.state.refresh();
    expect(f.state.pending.size).toBe(0);
    const snapshot = structuredClone(f.state.layouts);
    f.changed.mockClear();
    f.state.reset();
    expect(f.state.layouts).toEqual(snapshot);
    expect(f.changed).not.toHaveBeenCalled();
    expect(f.options.error).toHaveBeenCalledWith(
      "观察会话内容类型尚未就绪；请等待后再恢复默认布局。",
    );
    f.sources[0] = { ...f.sources[0]!, primaryKind: "chat" };
    f.state.refresh();
    f.state.reset();
    expect(f.state.layouts).toHaveLength(2);
    expect(f.state.layouts.every((panel) => panel.kind === "chat")).toBe(true);
    expect(f.state.layouts.map((panel) => panel.sessionId)).toEqual(["a", "b"]);
  });

  test("fanout reads source panels once per refresh regardless of duplicate view count", async () => {
    const f = fixture(
      Array.from({ length: 10 }, (_, index) => binding(`a-${index}`)),
    );
    await f.state.initialize();
    for (const source of f.sources) vi.mocked(source.panels).mockClear();
    f.state.refresh();
    for (const source of f.sources)
      expect(source.panels).toHaveBeenCalledTimes(1);
    expect(f.state.layouts).toHaveLength(10);
  });

  test("late focus guards reject old view selections, close and owner replacement; readonly targets do not focus", async () => {
    const f = fixture([binding("a-1"), binding("a-2"), binding("b-1", "b")]);
    await f.state.initialize();
    f.state.refresh();
    f.state.select("a-1");
    const guard = vi.mocked(f.sources[0]!.focus).mock.calls.at(-1)![2]!;
    expect(guard()).toBe(true);
    f.state.select("b-1");
    expect(guard()).toBe(false);
    expect(f.sources[0]!.blur).toHaveBeenCalled();
    const bGuard = vi.mocked(f.sources[1]!.focus).mock.calls.at(-1)![2]!;
    f.state.toggle(f.state.layouts[2]!);
    expect(bGuard()).toBe(false);
    expect(f.state.selection()).toBeUndefined();
    expect(f.state.selectedViewId).toBeUndefined();
    vi.mocked(f.sources[0]!.canInput).mockReturnValue(false);
    vi.mocked(f.sources[0]!.focus).mockClear();
    f.state.select("a-2");
    expect(f.sources[0]!.focus).not.toHaveBeenCalled();
    expect(f.state.selection()).toBeUndefined();
    vi.mocked(f.sources[0]!.canInput).mockReturnValue(true);
    f.state.reconcile();
    expect(f.sources[0]!.focus).not.toHaveBeenCalled();
    f.state.select("a-2");
    const replacementGuard = vi
      .mocked(f.sources[0]!.focus)
      .mock.calls.at(-1)![2]!;
    f.sources[0] = { ...f.sources[0]! };
    expect(replacementGuard()).toBe(false);
    f.state.refresh();
    expect(f.state.selectedViewId).toBeUndefined();
    expect(f.state.selection()).toBeUndefined();
    f.state.dispose();
    expect(f.state.selection()).toBeUndefined();
  });

  test("a formerly editable selection is visibly cleared on disconnect and cannot regain input without explicit reselection", async () => {
    const f = fixture([binding("a-input"), binding("b-observer", "b")]);
    await f.state.initialize();
    f.state.refresh();
    f.state.select("a-input");
    const oldGuard = vi.mocked(f.sources[0]!.focus).mock.calls.at(-1)![2]!;
    expect(f.state.selection()?.layout.id).toBe("a-input");
    vi.mocked(f.sources[0]!.canInput).mockReturnValue(false);
    f.state.refresh();
    expect(f.state.selectedViewId).toBeUndefined();
    expect(f.state.selection()).toBeUndefined();
    expect(oldGuard()).toBe(false);
    expect(f.sources[0]!.blur).toHaveBeenCalled();
    vi.mocked(f.sources[0]!.focus).mockClear();
    vi.mocked(f.sources[0]!.canInput).mockReturnValue(true);
    f.state.refresh();
    expect(f.state.selectedViewId).toBeUndefined();
    expect(f.state.selection()).toBeUndefined();
    expect(f.sources[0]!.focus).not.toHaveBeenCalled();
    expect(f.sources[1]!.focus).not.toHaveBeenCalled();
    f.state.select("a-input");
    expect(f.state.selection()?.layout.id).toBe("a-input");
    expect(f.sources[0]!.focus).toHaveBeenCalledTimes(1);
  });

  test("new default observations retain their reservation until canonical content identity is known", async () => {
    const f = fixture();
    f.sources[0] = {
      ...f.sources[0]!,
      primaryKind: undefined,
      panels: vi.fn(() => []),
    };
    const joining = f.state.observe("a");
    await Promise.resolve();
    await Promise.resolve();
    expect(f.state.layouts).toEqual([]);
    expect(f.state.pending.has("a")).toBe(true);
    expect(f.state.selectedViewId).toBeUndefined();
    // Canonical identity can arrive before the chat projection itself is available.
    f.sources[0] = { ...f.sources[0]!, primaryKind: "chat" };
    f.state.refresh();
    await joining;
    expect(f.state.layouts).toHaveLength(1);
    expect(f.state.layouts[0]).toMatchObject({
      sessionId: "a",
      kind: "chat",
      sourceId: "chat",
    });
    f.state.refresh();
    expect(f.state.layouts).toHaveLength(1);
    expect(f.state.layouts.some((panel) => panel.kind === "terminal")).toBe(
      false,
    );
  });
});
