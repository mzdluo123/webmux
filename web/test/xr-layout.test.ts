import { describe, expect, test } from "vite-plus/test";
import {
  boundXrPanel,
  decodeXrLayout,
  defaultXrPanel,
  encodeXrLayout,
  XR_PANEL_LIMIT,
} from "../src/domains/terminal/xr-layout";

const sessions = new Set(["one", "two"]);
const panel = () => defaultXrPanel("terminal:one", "one", "terminal", 1);

describe("device-local XR layout", () => {
  test("round-trips v2 instance bindings, order, visibility and geometry", () => {
    const first = panel();
    first.position = [2, -1, -3];
    first.size = [2.5, 1.4];
    first.visible = false;
    const second = defaultXrPanel("chat-copy", "two", "chat", 2, "chat");
    second.position = [1.2, 0.5, -3];
    const third = { ...second, id: "another-copy" };
    const encoded = encodeXrLayout([second, third, first]);
    expect(encoded.version).toBe(2);
    expect(
      decodeXrLayout(JSON.parse(JSON.stringify(encoded)), sessions),
    ).toEqual([second, third, first]);
  });

  test("migrates exact legacy prefixes without splitting colon-bearing source or session IDs", () => {
    const sessionId = "remote:one";
    const kinds = ["terminal", "chat", "chat", "chat", "code", "diff"] as const;
    const sourceIds = [
      "terminal",
      "chat",
      "draft",
      "notice",
      "path:a:b",
      "file:c:d",
    ];
    const legacy = kinds.map((kind, index) => {
      const sourceId = sourceIds[index]!;
      const value = defaultXrPanel(
        kind === "terminal"
          ? `terminal:${sessionId}`
          : `${kind}:${sessionId}:${sourceId}`,
        sessionId,
        kind,
        index,
        sourceId,
      );
      const { sourceId: _sourceId, ...record } = value;
      return { ...record, visible: index !== 2 };
    });
    const migrated = decodeXrLayout(
      { version: 1, panels: legacy },
      new Set([sessionId]),
    )!;
    expect(migrated.map((value) => value.sourceId)).toEqual(sourceIds);
    expect(migrated.map(({ sourceId: _sourceId, ...value }) => value)).toEqual(
      legacy,
    );
    expect(
      migrated.filter(
        (value) => value.kind === "chat" && value.sourceId === "chat",
      ),
    ).toHaveLength(1);
    expect(
      decodeXrLayout({
        version: 1,
        panels: [{ ...legacy[1], id: "chat:remote:other:chat" }],
      }),
    ).toBeUndefined();
    expect(encodeXrLayout(migrated).version).toBe(2);
  });

  test("drops invalid neighbours, stale sessions and duplicate instance IDs individually", () => {
    const valid = panel();
    const other = defaultXrPanel("chat:two:chat", "two", "chat", 2, "chat");
    const records = [
      valid,
      { ...valid, id: "bad", position: [Infinity, 0, -2] },
      { ...valid, id: "stale", sessionId: "deleted" },
      valid,
      other,
    ];
    expect(decodeXrLayout({ version: 2, panels: records }, sessions)).toEqual([
      valid,
      other,
    ]);
    expect(
      decodeXrLayout({ version: 2, panels: records })?.some(
        (value) => value.sessionId === "deleted",
      ),
    ).toBe(true);
  });

  test("distinguishes an intentionally empty layout from an unreadable document", () => {
    expect(decodeXrLayout({ version: 2, panels: [] })).toEqual([]);
    expect(decodeXrLayout({ version: 1, panels: [] })).toEqual([]);
    expect(
      decodeXrLayout({ version: 2, panels: [panel()] }, new Set()),
    ).toEqual([]);
    for (const value of [
      null,
      "broken",
      { version: 3, panels: [] },
      { version: 2, panels: Array.from({ length: XR_PANEL_LIMIT + 1 }, panel) },
    ])
      expect(decodeXrLayout(value, sessions)).toBeUndefined();
    const full = Array.from({ length: XR_PANEL_LIMIT }, (_, index) => ({
      ...panel(),
      id: `view-${index}`,
    }));
    expect(decodeXrLayout(encodeXrLayout(full), sessions)).toHaveLength(
      XR_PANEL_LIMIT,
    );
  });

  test.each([
    { position: [NaN, 0, -2] },
    { position: [0, 0, 0] },
    { position: [-9, 0, -2] },
    { size: [0, 1] },
    { size: [1, Infinity] },
    { size: [4.01, 1] },
    { visible: "yes" },
    { kind: "unknown" },
    { id: "" },
    { sourceId: "" },
    { sourceId: "not-terminal" },
  ])("rejects invalid panel values: %j", (invalid) => {
    expect(
      decodeXrLayout(
        { version: 2, panels: [{ ...panel(), ...invalid }] },
        sessions,
      ),
    ).toBeUndefined();
  });

  test("projects only binding and display metadata on both read and write", () => {
    const secret = {
      ...panel(),
      token: "token-value",
      text: "terminal-output",
      draft: "draft-body",
      lines: ["code-body"],
      offset: 82,
      pending: "grant",
      modifiers: { ctrl: true },
    };
    expect(encodeXrLayout([secret])).toEqual({ version: 2, panels: [panel()] });
    expect(
      decodeXrLayout(
        { version: 2, token: "another-token", panels: [secret] },
        sessions,
      ),
    ).toEqual([panel()]);
    expect(JSON.stringify(encodeXrLayout([secret]))).not.toMatch(
      /token-value|terminal-output|draft-body|code-body|offset|pending|modifiers/,
    );
  });

  test("bounds physical drag and resize", () => {
    const value = panel();
    value.position = [-100, 100, -100];
    value.size = [100, 0.01];
    boundXrPanel(value);
    expect(value.position).toEqual([-8, 8, -12]);
    expect(value.size).toEqual([4, 0.3]);
  });
});
