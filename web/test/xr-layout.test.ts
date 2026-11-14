import { describe, expect, test } from "vitest";
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
  test("round-trips order, visibility and physical transforms", () => {
    const first = panel();
    first.position = [2, -1, -3];
    first.size = [2.5, 1.4];
    first.visible = false;
    const second = defaultXrPanel("diff:two:file", "two", "diff", 2);
    second.position = [-1.2, 0.6, -4];
    const encoded = encodeXrLayout([second, first]);
    expect(
      decodeXrLayout(JSON.parse(JSON.stringify(encoded)), sessions),
    ).toEqual([second, first]);
  });

  test("drops invalid neighbours, stale sessions and duplicate panel IDs individually", () => {
    const valid = panel();
    const other = defaultXrPanel("chat:two", "two", "chat", 2);
    const records = [
      valid,
      { ...valid, id: "bad", position: [Infinity, 0, -2] },
      { ...valid, id: "stale", sessionId: "deleted" },
      valid,
      other,
    ];
    expect(decodeXrLayout({ version: 1, panels: records }, sessions)).toEqual([
      valid,
      other,
    ]);
  });

  test.each([
    null,
    "broken",
    { version: 2, panels: [panel()] },
    { version: 1, panels: Array.from({ length: XR_PANEL_LIMIT + 1 }, panel) },
  ])("rejects an unreadable or unbounded document: %j", (value) => {
    expect(decodeXrLayout(value, sessions)).toEqual([]);
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
  ])("rejects invalid panel values: %j", (invalid) => {
    expect(
      decodeXrLayout(
        { version: 1, panels: [{ ...panel(), ...invalid }] },
        sessions,
      ),
    ).toEqual([]);
  });

  test("strips output, authentication and other unrecognized fields on both read and write", () => {
    const secret = {
      ...panel(),
      token: "token-value",
      text: "terminal-output",
      draft: "draft-body",
      lines: ["code-body"],
    };
    expect(encodeXrLayout([secret])).toEqual({ version: 1, panels: [panel()] });
    expect(
      decodeXrLayout(
        { version: 1, token: "another-token", panels: [secret] },
        sessions,
      ),
    ).toEqual([panel()]);
    expect(JSON.stringify(encodeXrLayout([secret]))).not.toMatch(
      /token-value|terminal-output|draft-body|code-body/,
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
