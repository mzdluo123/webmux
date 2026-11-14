import { afterEach, expect, test } from "vitest";
import type { QuickCommand } from "../../src/contracts/schemas";
import { createTerminalDrafts } from "../../src/domains/terminal/terminal-draft.svelte";
import { executeSessionQuickCommand } from "../../src/domains/terminal/session-terminal-io";
import type { XrSessionSource } from "../../src/domains/terminal/xr-workspace";
import { XrPanel } from "../../src/hooks/terminal/xr-panel";

const panels: XrPanel[] = [];
afterEach(() => {
  for (const panel of panels.splice(0)) panel.dispose();
});

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
  const source: XrSessionSource = {
    id: draft.sessionId,
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
    actions: () => actions,
    input() {},
    focus() {},
    latest() {},
    scroll() {},
    resize: () => false,
    setImmersive() {},
  };
  const panel = new XrPanel(
    {
      id: "chat:live-command",
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
