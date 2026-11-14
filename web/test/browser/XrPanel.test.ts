import { afterEach, expect, test, vi } from "vite-plus/test";
import type { QuickCommand } from "../../src/contracts/schemas";
import { createTerminalDrafts } from "../../src/domains/terminal/terminal-draft.svelte";
import { executeSessionQuickCommand } from "../../src/hooks/terminal/session-quick-command";
import type {
  XrSessionSource,
  XrTextPanel,
} from "../../src/domains/terminal/xr-workspace";
import { XrPanel } from "../../src/hooks/terminal/xr-panel";

const panels: XrPanel[] = [];
afterEach(() => {
  for (const panel of panels.splice(0)) panel.dispose();
});

function createPanel(source: XrSessionSource) {
  const panel = new XrPanel(
    {
      id: `chat:${source.id}`,
      sessionId: source.id,
      kind: "chat",
      position: [0, 0, -2.4],
      size: [1.6, 1.1],
      visible: true,
    },
    () => {},
    () => {},
    () => {},
    (message) => {
      throw new Error(message);
    },
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
