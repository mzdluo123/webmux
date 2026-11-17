import { expect, test, vi } from "vite-plus/test";
import { defaultXrPanel } from "../../src/domains/terminal/xr-layout";
import type { XrSessionSource } from "../../src/domains/terminal/xr-workspace";
import { createXrKeyboard } from "../../src/hooks/terminal/xr-keyboard";
import type { XrViewSelection } from "../../src/hooks/terminal/xr-view-state";

test("VR keyboard revalidates the selected binding and cannot send historical chat text into a TUI", () => {
  let draft = "";
  let chatEditable = true;
  const source: XrSessionSource = {
    id: "shared-a",
    title: "Session A",
    status: "connected",
    terminal: undefined,
    primaryKind: "chat",
    graphicsUnsupported: false,
    modifiers: { ctrl: false, alt: false, shift: false },
    shiftLocked: false,
    toggleModifier: vi.fn(),
    resetModifiers: vi.fn(),
    lockModifier: vi.fn(),
    panels: () => [],
    actions: () => [],
    input: vi.fn((value) => {
      draft += value;
    }),
    canInput: (kind, sourceId) =>
      kind === "chat" && sourceId === "chat" && chatEditable,
    focus: vi.fn(),
    blur: vi.fn(),
    latest: vi.fn(),
    scroll: vi.fn(),
    resize: () => false,
    setImmersive: vi.fn(),
  };
  const first = defaultXrPanel("a-first", source.id, "chat", 1, "chat");
  const second = defaultXrPanel("a-second", source.id, "chat", 2, "chat");
  let selected: XrViewSelection | undefined = { source, layout: first };
  const keyboard = createXrKeyboard(() => selected);
  try {
    keyboard.surface.group.visible = true;
    keyboard.update();
    const cached = keyboard.surface.regions.find(
      (region) => region.id === "key:a",
    )!;
    expect(cached.disabled).toBe(false);
    cached.run?.();
    selected = { source, layout: second };
    keyboard.update();
    keyboard.surface.regions.find((region) => region.id === "key:b")!.run?.();
    expect(draft).toBe("ab");
    expect(source.input).toHaveBeenCalledTimes(2);
    chatEditable = false;
    cached.run?.();
    keyboard.update();
    expect(draft).toBe("ab");
    expect(
      keyboard.surface.regions.find((region) => region.id === "key:a")
        ?.disabled,
    ).toBe(true);
    expect(
      keyboard.surface.regions.find((region) => region.id === "modifier:ctrl")
        ?.disabled,
    ).toBe(true);
    selected = undefined;
    cached.run?.();
    expect(source.input).toHaveBeenCalledTimes(2);
    expect(source.focus).not.toHaveBeenCalled();
  } finally {
    keyboard.dispose();
  }
});
