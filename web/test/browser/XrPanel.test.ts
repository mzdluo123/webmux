import { afterEach, expect, test, vi } from "vite-plus/test";
import type { QuickCommand } from "../../src/contracts/schemas";
import { createTerminalDrafts } from "../../src/domains/terminal/terminal-draft.svelte";
import { executeSessionQuickCommand } from "../../src/domains/terminal/session-terminal-io";
import type {
  XrSessionSource,
  XrTextPanel,
} from "../../src/domains/terminal/xr-workspace";
import { XrPanel } from "../../src/hooks/terminal/xr-panel";

const panels: XrPanel[] = [];
afterEach(() => {
  for (const panel of panels.splice(0)) panel.dispose();
});

function createPanel(
  source: XrSessionSource,
  id = `chat:${source.id}`,
  select: (id: string) => void = () => {},
  split?: (id: string) => void,
) {
  const panel = new XrPanel(
    {
      id,
      sessionId: source.id,
      kind: "chat",
      sourceId: "chat",
      position: [0, 0, -2.4],
      size: [1.6, 1.1],
      visible: true,
    },
    select,
    () => {},
    () => {},
    (message) => {
      throw new Error(message);
    },
    split,
  );
  panels.push(panel);
  return panel;
}

function createSource(
  id: string,
  actions: XrSessionSource["actions"] = () => [],
): XrSessionSource {
  return {
    id,
    title: "Live session",
    status: "connected",
    primaryKind: "chat",
    terminal: undefined,
    graphicsUnsupported: false,
    modifiers: { ctrl: false, alt: false, shift: false },
    shiftLocked: false,
    toggleModifier() {},
    resetModifiers() {},
    lockModifier() {},
    panels: () => [],
    actions,
    input() {},
    focus() {},
    blur() {},
    canInput: () => true,
    latest() {},
    scroll() {},
    resize: () => false,
    setImmersive() {},
  };
}

test("an unchanged spatial command label uses the latest command body", async () => {
  const drafts = createTerminalDrafts();
  const draft = drafts.get("live-command");
  drafts.setEnabled(draft, true);
  const command: QuickCommand = {
    id: "live",
    name: "Live command",
    command: "old command",
    enter: false,
  };
  const context = {
    id: draft.sessionId,
    isMounted: () => true,
    isChatView: () => false,
    draft,
    drafts,
    beginInput: () => drafts.begin(draft),
    insertText: (text: string, operation = drafts.begin(draft)) =>
      drafts.insert(draft, text, operation, () => false),
    resetModifiers() {},
    submitTerminal() {
      throw new Error("a draft command must not submit terminal input");
    },
    getError: () => "",
    setError(message: string) {
      if (message) throw new Error(message);
    },
  };
  let actions = [
    {
      id: `quick:${command.id}`,
      label: command.name,
      run: () => executeSessionQuickCommand(command, context),
    },
  ];
  const source = createSource(draft.sessionId, () => actions);
  const panel = createPanel(source);
  panel.update(source, undefined, false);
  const cachedButton = panel.surface.regions.find(
    (region) => region.id === "action:quick:live",
  )!;
  const edited = { ...command, command: "updated command" };
  actions = [
    {
      id: `quick:${edited.id}`,
      label: edited.name,
      run: () => executeSessionQuickCommand(edited, context),
    },
  ];
  panel.update(source, undefined, false);
  cachedButton.run?.();
  await expect.poll(() => draft.text).toBe("updated command");
});

test("Latest leaves stable chat history for the current tail and resumes following", () => {
  const source = createSource("growing-chat");
  const panel = createPanel(source);
  const text = (length: number): XrTextPanel => ({
    id: "reply",
    kind: "chat",
    title: "Reply",
    lines: Array.from({ length }, (_, index) => ({
      text: `reply-line-${index + 1}`,
    })),
  });
  const draw = vi.spyOn(panel.surface.context, "fillText");
  const drawnLines = () =>
    draw.mock.calls
      .map(([value]) => value)
      .filter((value) => value.startsWith("reply-line-"));

  panel.update(source, text(50), false);
  expect(drawnLines().at(-1)).toBe("reply-line-50");
  panel.scroll(-8);
  draw.mockClear();
  panel.update(source, text(50), false);
  const history = drawnLines();
  expect(history).toHaveLength(18);
  expect(history.at(-1)).toBe("reply-line-42");
  const historyPixels = panel.surface.context.getImageData(20, 110, 960, 530);

  draw.mockClear();
  panel.update(source, text(80), false);
  expect(panel.surface.context.getImageData(20, 110, 960, 530).data).toEqual(
    historyPixels.data,
  );
  const latest = panel.surface.hit(300 / 1024, 1 - 87 / 768)!;
  draw.mockClear();

  latest.run?.();
  panel.update(source, text(80), false);
  expect(drawnLines()).toEqual(
    Array.from({ length: 18 }, (_, index) => `reply-line-${index + 63}`),
  );

  draw.mockClear();
  panel.update(source, text(90), false);
  expect(drawnLines()).toEqual(
    Array.from({ length: 18 }, (_, index) => `reply-line-${index + 73}`),
  );
});

test("two production Canvas chat views keep independent history, geometry and current tails", () => {
  const source = createSource("dual-chat");
  const history = createPanel(source, "history-view-001");
  const tail = createPanel(source, "tail-view-002");
  const projection = (length: number): XrTextPanel => ({
    id: "chat",
    kind: "chat",
    title: "Agent chat",
    lines: Array.from({ length }, (_, index) => ({
      text: `shared-reply-${index + 1}`,
    })),
  });
  let text = projection(50);
  source.panels = () => [text];
  const historyDraw = vi.spyOn(history.surface.context, "fillText");
  const tailDraw = vi.spyOn(tail.surface.context, "fillText");
  history.update(source, text, true);
  history.scroll(-16);
  history.update(source, text, true);
  const pixels = history.surface.context.getImageData(20, 110, 960, 530).data;
  tail.update(source, text, false);
  expect(tailDraw.mock.calls.some(([line]) => line === "shared-reply-50")).toBe(
    true,
  );
  expect(
    history.surface.regions.find((region) => region.id === "move")?.label,
  ).not.toBe(
    tail.surface.regions.find((region) => region.id === "move")?.label,
  );
  const originalPosition = [...history.layout.position];
  const originalSize = [...history.layout.size];
  tail.layout.position = [1.5, 0.8, -3];
  tail.layout.size = [2.4, 1.8];
  tail.applyLayout();
  expect(history.layout.position).toEqual(originalPosition);
  expect(history.layout.size).toEqual(originalSize);
  text = projection(85);
  historyDraw.mockClear();
  tailDraw.mockClear();
  history.update(source, text, true);
  tail.update(source, text, false);
  expect(history.surface.context.getImageData(20, 110, 960, 530).data).toEqual(
    pixels,
  );
  expect(tailDraw.mock.calls.some(([line]) => line === "shared-reply-85")).toBe(
    true,
  );
  expect(
    historyDraw.mock.calls.some(([line]) => line === "shared-reply-85"),
  ).toBe(false);
  history.surface.regions.find((region) => region.id === "latest")!.run?.();
  history.update(source, text, true);
  expect(
    historyDraw.mock.calls.some(([line]) => line === "shared-reply-85"),
  ).toBe(true);
  tail.surface.regions.find((region) => region.id === "close")!.run?.();
  expect(tail.layout.visible).toBe(false);
  expect(history.layout.visible).toBe(true);
  text = projection(100);
  historyDraw.mockClear();
  history.update(source, text, true);
  expect(
    historyDraw.mock.calls.some(([line]) => line === "shared-reply-100"),
  ).toBe(true);
});

test("cached ordinary and confirmation actions revalidate canonical requests in every copy", async () => {
  const execute = vi.fn();
  let text: XrTextPanel = {
    id: "chat",
    kind: "chat",
    title: "Permission",
    lines: [],
    actions: [
      {
        id: "permission:one",
        label: "Allow",
        confirm: "Grant access?",
        run: execute,
      },
      { id: "send", label: "Send", run: execute },
    ],
  };
  const source = createSource("canonical-actions");
  source.panels = () => [text];
  const first = createPanel(source, "first");
  const second = createPanel(source, "second");
  first.update(source, text, true);
  second.update(source, text, false);
  const staleSend = first.surface.regions.find(
    (region) => region.id === "action:send",
  )!;
  first.surface.regions
    .find((region) => region.id === "action:permission:one")!
    .run?.();
  second.surface.regions
    .find((region) => region.id === "action:permission:one")!
    .run?.();
  first.update(source, text, true);
  second.update(source, text, false);
  const staleFirst = first.surface.regions.find(
    (region) => region.id === "confirm-action",
  )!;
  const staleSecond = second.surface.regions.find(
    (region) => region.id === "confirm-action",
  )!;
  text = { ...text, actions: [] };
  staleSend.run?.();
  staleFirst.run?.();
  staleSecond.run?.();
  await Promise.resolve();
  expect(execute).not.toHaveBeenCalled();
  first.update(source, text, true);
  second.update(source, text, false);
  expect(
    first.surface.regions.some((region) => region.id === "confirm-action"),
  ).toBe(false);
  expect(
    second.surface.regions.some((region) => region.id === "confirm-action"),
  ).toBe(false);
  text = {
    ...text,
    actions: [
      {
        id: "permission:two",
        label: "Allow",
        confirm: "New request",
        disabled: true,
        run: execute,
      },
    ],
  };
  first.update(source, text, true);
  staleFirst.run?.();
  await Promise.resolve();
  expect(execute).not.toHaveBeenCalled();
});

test("scrolling or transforming B never selects it; an explicit body click selects its view instance", () => {
  const selected = vi.fn();
  const source = createSource("observed-b");
  const panel = createPanel(source, "b-view", selected);
  const text: XrTextPanel = {
    id: "chat",
    kind: "chat",
    title: "B",
    lines: Array.from({ length: 60 }, (_, index) => ({
      text: `line ${index}`,
    })),
  };
  panel.update(source, text, false);
  panel.scroll(-12);
  panel.layout.position = [2, -1, -3];
  panel.layout.size = [2, 1.6];
  panel.applyLayout();
  panel.update(source, text, false);
  expect(selected).not.toHaveBeenCalled();
  panel.surface.regions.find((region) => region.id === "body")!.run?.();
  expect(selected).toHaveBeenCalledExactlyOnceWith("b-view");
});

test("only chat/chat exposes the explicit split control, never draft or notice projections", () => {
  const source = createSource("split-controls");
  const split = vi.fn();
  const panel = createPanel(source, "real-chat-view", () => {}, split);
  for (const sourceId of ["draft", "notice", "chat"]) {
    panel.layout.sourceId = sourceId;
    const text: XrTextPanel = {
      id: sourceId,
      kind: "chat",
      title: sourceId,
      lines: [],
    };
    panel.update(source, text, false);
    const button = panel.surface.regions.find(
      (region) => region.id === "split",
    );
    expect(!!button).toBe(sourceId === "chat");
    button?.run?.();
  }
  expect(split).toHaveBeenCalledExactlyOnceWith("real-chat-view");
});

test("readonly real chat removes generic source actions and invalidates cached actions and confirmations", async () => {
  const sent: string[] = [];
  const returnTo2D = vi.fn();
  let editable = true;
  const source = createSource("readonly-history", () => [
    {
      id: "generic-input",
      label: "Generic input",
      run: () => {
        sent.push("tui-bytes");
      },
    },
    {
      id: "generic-confirm",
      label: "Generic confirmed input",
      confirm: "Send to TUI?",
      run: () => {
        sent.push("confirmed-tui-bytes");
      },
    },
  ]);
  source.canInput = () => editable;
  const text: XrTextPanel = {
    id: "chat",
    kind: "chat",
    title: "Chat history",
    lines: [{ text: "Historical agent reply" }],
    actions: [{ id: "chat-2d", label: "返回 2D 切换到聊天", run: returnTo2D }],
  };
  source.panels = () => [text];
  const first = createPanel(source, "readonly-first");
  const second = createPanel(source, "readonly-second");
  first.update(source, text, true);
  second.update(source, text, false);
  const staleOrdinary = first.surface.regions.find(
    (region) => region.id === "action:generic-input",
  )!;
  for (const panel of [first, second]) {
    panel.surface.regions
      .find((region) => region.id === "action:generic-confirm")!
      .run?.();
    panel.update(source, text, false);
  }
  const staleConfirm = first.surface.regions.find(
    (region) => region.id === "confirm-action",
  )!;
  editable = false;
  staleOrdinary.run?.();
  staleConfirm.run?.();
  first.update(source, text, false);
  second.update(source, text, false);
  await Promise.resolve();
  expect(sent).toEqual([]);
  for (const panel of [first, second]) {
    expect(
      panel.surface.regions.some((region) => region.id === "confirm-action"),
    ).toBe(false);
    expect(
      panel.surface.regions.some((region) =>
        region.id.startsWith("action:generic"),
      ),
    ).toBe(false);
    expect(
      panel.surface.regions.some((region) => region.id === "action:chat-2d"),
    ).toBe(true);
  }
  first.surface.regions
    .find((region) => region.id === "action:chat-2d")!
    .run?.();
  await Promise.resolve();
  expect(returnTo2D).toHaveBeenCalledTimes(1);
});
