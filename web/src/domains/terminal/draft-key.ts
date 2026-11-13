import type { TerminalDraft, TerminalDrafts } from "./terminal-draft.svelte";

const graphemes =
  typeof Intl.Segmenter === "function"
    ? new Intl.Segmenter(undefined, { granularity: "grapheme" })
    : undefined;

function characterAt(text: string, offset: number) {
  if (offset < 0 || offset >= text.length) return;
  const match = graphemes?.segment(text).containing(offset);
  if (match)
    return { start: match.index, end: match.index + match.segment.length };
  const start =
    (text.codePointAt(offset - 1) ?? 0) > 0xffff ? offset - 1 : offset;
  return {
    start,
    end: start + ((text.codePointAt(start) ?? 0) > 0xffff ? 2 : 1),
  };
}

/** Soft keyboards edit the same saved selection as the real textarea. */
export function editDraftKey(
  draft: TerminalDraft,
  drafts: TerminalDrafts,
  input: string,
): boolean {
  if (draft.submission) return false;
  const text = draft.text;
  let start = draft.selectionStart ?? text.length;
  let end = draft.selectionEnd ?? start;
  const backspace = input === "\x7f" || input === "\b";
  let insertion = "";
  if (backspace || input === "\x1b[3~") {
    if (start === end) {
      const character = characterAt(text, backspace ? start - 1 : end);
      if (character) ({ start, end } = character);
    }
  } else if (
    ["\x1b[D", "\x1b[C", "\x1b[A", "\x1b[B", "\x1b[H", "\x1b[F"].includes(input)
  ) {
    let position = start;
    const lineStart = start === 0 ? 0 : text.lastIndexOf("\n", start - 1) + 1;
    const lineEnd = text.indexOf("\n", end);
    if (input === "\x1b[D")
      position =
        start !== end ? start : (characterAt(text, start - 1)?.start ?? 0);
    if (input === "\x1b[C")
      position =
        start !== end ? end : (characterAt(text, end)?.end ?? text.length);
    if (input === "\x1b[H") position = lineStart;
    if (input === "\x1b[F") position = lineEnd < 0 ? text.length : lineEnd;
    if (input === "\x1b[A") {
      const previousStart =
        lineStart < 2 ? 0 : text.lastIndexOf("\n", lineStart - 2) + 1;
      position =
        lineStart === 0
          ? 0
          : Math.min(lineStart - 1, previousStart + start - lineStart);
    }
    if (input === "\x1b[B") {
      const nextStart = lineEnd < 0 ? text.length : lineEnd + 1;
      const nextEnd = text.indexOf("\n", nextStart);
      position = Math.min(
        nextEnd < 0 ? text.length : nextEnd,
        nextStart + start - lineStart,
      );
    }
    drafts.select(draft, position, position);
    return true;
  } else if (
    input === "\r" ||
    input === "\n" ||
    input === "\t" ||
    !Array.from(input).some((character) => {
      const code = character.codePointAt(0)!;
      return code < 32 || code === 127;
    })
  ) {
    insertion = input === "\r" ? "\n" : input;
  } else return false;
  drafts.edit(
    draft,
    text.slice(0, start) + insertion + text.slice(end),
    start + insertion.length,
  );
  return true;
}
