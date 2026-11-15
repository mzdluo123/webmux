import * as m from "$lib/paraglide/messages.js";
import { encodeBase64Url } from "../../platform/base64url";
import type { CodeDocument } from "../../contracts/schemas";

export function codeReference(doc: CodeDocument, start?: number, end?: number) {
  if (!doc.referenceable || doc.source === "empty")
    throw new Error(m.s5_path_not_referenceable());
  if (
    start !== undefined &&
    (!Number.isInteger(start) ||
      start < 1 ||
      start > doc.lineCount ||
      !Number.isInteger(end ?? start) ||
      (end ?? start) < start ||
      (end ?? start) > doc.lineCount ||
      doc.kind !== "text")
  )
    throw new Error(m.s5_invalid_line_range());
  // Keep control characters in the existing reversible display notation. Its
  // double quotes still need to suppress shell substitutions.
  // oxlint-disable-next-line no-control-regex -- Escape control bytes before sending paths to a terminal.
  const path = /[\x00-\x1f\x7f-\x9f\u2028\u2029]/.test(doc.absolutePath)
    ? JSON.stringify(doc.absolutePath).replace(
        /[\x7f-\x9f\u2028\u2029$`]/g,
        (c) =>
          c === "$" || c === "`"
            ? "\\" + c
            : "\\u" + c.charCodeAt(0).toString(16).padStart(4, "0"),
      )
    : /^[A-Za-z0-9_./:@%+=,-]+$/.test(doc.absolutePath)
      ? doc.absolutePath
      : "'" + doc.absolutePath.replaceAll("'", "'\\''") + "'";
  const range =
    start === undefined
      ? ""
      : `:${start}${end !== undefined && end !== start ? `-${end}` : ""}`;
  const source =
    doc.source === "index"
      ? m.s5_ref_suffix_index()
      : doc.source === "head"
        ? m.s5_ref_suffix_head()
        : "";
  return `${path}${range}${source} `;
}
export function nodeId(path: string) {
  return encodeBase64Url(path);
}
