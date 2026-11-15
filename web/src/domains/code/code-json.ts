// Recursive-descent JSON parsing for the read-only tree preview. Numbers keep
// their source text so large integers and trailing zeros are shown verbatim.
import { m } from "$lib/paraglide/messages.js";

export type JsonNode = { line: number } & (
  | { kind: "object"; entries: { key: string; value: JsonNode }[] }
  | { kind: "array"; items: JsonNode[] }
  | { kind: "string"; value: string }
  | { kind: "number"; raw: string }
  | { kind: "boolean"; value: boolean }
  | { kind: "null" }
);

type JsonResult =
  | { ok: true; value: JsonNode }
  | { ok: false; line: number; column: number; detail: string };

// Deeper documents are rejected instead of overflowing the call stack.
export const maxJsonDepth = 1000;

class JsonSyntaxError extends Error {
  constructor(
    readonly offset: number,
    detail: string,
  ) {
    super(detail);
  }
}

const numberPattern = /-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?/y;
const hexPattern = /^[0-9a-fA-F]{4}$/;
const escapes: Record<string, string> = {
  '"': '"',
  "\\": "\\",
  "/": "/",
  b: "\b",
  f: "\f",
  n: "\n",
  r: "\r",
  t: "\t",
};

export function parseJson(text: string): JsonResult {
  const source = text.startsWith("\uFEFF") ? text.slice(1) : text;
  let index = 0;
  let line = 1;
  let lineStart = 0;
  const fail = (detail: string, at = index): never => {
    throw new JsonSyntaxError(at, detail);
  };
  const space = () => {
    for (; index < source.length; index++) {
      const char = source[index];
      if (char === "\n") {
        line++;
        lineStart = index + 1;
      } else if (char !== " " && char !== "\t" && char !== "\r") return;
    }
  };
  const expect = (char: string, detail: () => string) => {
    if (source[index] !== char) fail(detail());
    index++;
  };
  const string = () => {
    index++;
    let value = "";
    for (;;) {
      const char = source[index];
      if (char === undefined) return fail(m.s5_json_unclosed_string());
      if (char === '"') {
        index++;
        return value;
      }
      if (char < " ") fail(m.s5_json_unescaped_control());
      if (char !== "\\") {
        value += char;
        index++;
        continue;
      }
      const escape = source[index + 1];
      if (escape === "u") {
        const hex = source.slice(index + 2, index + 6);
        if (!hexPattern.test(hex)) fail(m.s5_json_invalid_unicode_escape());
        value += String.fromCharCode(Number.parseInt(hex, 16));
        index += 6;
      } else if (escape !== undefined && escape in escapes) {
        value += escapes[escape];
        index += 2;
      } else fail(m.s5_json_invalid_escape());
    }
  };
  // `depth` counts the containers around the value being parsed.
  const node = (depth: number): JsonNode => {
    space();
    const at = line;
    const char = source[index];
    if ((char === "{" || char === "[") && depth >= maxJsonDepth)
      fail(m.s5_json_max_depth_exceeded({ depth: String(maxJsonDepth) }));
    if (char === "{") {
      index++;
      const entries: { key: string; value: JsonNode }[] = [];
      space();
      if (source[index] === "}") {
        index++;
        return { line: at, kind: "object", entries };
      }
      for (;;) {
        space();
        if (source[index] !== '"') fail(m.s5_json_missing_property_name());
        const key = string();
        space();
        expect(":", () => m.s5_json_missing_colon());
        entries.push({ key, value: node(depth + 1) });
        space();
        if (source[index] === "}") {
          index++;
          return { line: at, kind: "object", entries };
        }
        expect(",", () => m.s5_json_missing_comma_or_brace());
      }
    }
    if (char === "[") {
      index++;
      const items: JsonNode[] = [];
      space();
      if (source[index] === "]") {
        index++;
        return { line: at, kind: "array", items };
      }
      for (;;) {
        items.push(node(depth + 1));
        space();
        if (source[index] === "]") {
          index++;
          return { line: at, kind: "array", items };
        }
        expect(",", () => m.s5_json_missing_comma_or_bracket());
      }
    }
    if (char === '"') return { line: at, kind: "string", value: string() };
    for (const [word, value] of [
      ["true", true],
      ["false", false],
      ["null", null],
    ] as const)
      if (source.startsWith(word, index)) {
        index += word.length;
        return value === null
          ? { line: at, kind: "null" }
          : { line: at, kind: "boolean", value };
      }
    numberPattern.lastIndex = index;
    const number = numberPattern.exec(source);
    if (number) {
      index += number[0].length;
      return { line: at, kind: "number", raw: number[0] };
    }
    return fail(
      char === undefined
        ? m.s5_json_unexpected_eof()
        : m.s5_json_unexpected_value(),
    );
  };
  try {
    const value = node(0);
    space();
    if (index < source.length) fail(m.s5_json_trailing_content());
    return { ok: true, value };
  } catch (error) {
    if (!(error instanceof JsonSyntaxError)) throw error;
    return {
      ok: false,
      line,
      column: error.offset - lineStart + 1,
      detail: error.message,
    };
  }
}
