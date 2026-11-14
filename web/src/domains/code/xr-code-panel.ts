import type { FileDiffMetadata } from "@pierre/diffs";
import type { CodeDiff, CodeDocument } from "../../contracts/schemas";
import type { XrTextLine, XrTextPanel } from "../terminal/xr-workspace";

/** Reuses the reader's calculated diff; never calculates on the XR frame. */
export function xrCodePanel(
  contextId: string,
  document: CodeDocument | undefined,
  diff: CodeDiff | undefined,
  metadata: FileDiffMetadata | undefined,
  reason: string,
): XrTextPanel {
  const lines: XrTextLine[] = [];
  const id = `${contextId}:${document?.nodeId ?? ""}:${diff ? diff.change.group : document?.source}`;
  const title = document?.path || diff?.new.path || diff?.old.path || "代码";
  if (diff && metadata) {
    for (const hunk of metadata.hunks) {
      lines.push({
        text: `@@ -${hunk.deletionStart},${hunk.deletionCount} +${hunk.additionStart},${hunk.additionCount} @@`,
        tone: "muted",
      });
      for (const block of hunk.hunkContent) {
        if (block.type === "context") {
          for (let i = 0; i < block.lines; i++) {
            const index = block.additionLineIndex + i;
            lines.push({
              text: ` ${index + 1} ${metadata.additionLines[index] ?? ""}`,
            });
          }
        } else {
          for (let i = 0; i < block.deletions; i++) {
            const index = block.deletionLineIndex + i;
            lines.push({
              text: `-${index + 1} ${metadata.deletionLines[index] ?? ""}`,
              tone: "removed",
            });
          }
          for (let i = 0; i < block.additions; i++) {
            const index = block.additionLineIndex + i;
            lines.push({
              text: `+${index + 1} ${metadata.additionLines[index] ?? ""}`,
              tone: "added",
            });
          }
        }
      }
    }
    if (!metadata.hunks.length)
      lines.push({ text: "没有文本差异。", tone: "muted" });
  } else if (diff) {
    lines.push({
      text: reason || diff.reason || "正在计算差异…",
      tone: "muted",
    });
    if (diff.kind !== "text")
      lines.push({
        text: "此类内容请退出 VR 后在 2D 代码工作区查看。",
        tone: "muted",
      });
  } else if (document?.kind === "text") {
    for (const [index, text] of document.text.split("\n").entries())
      lines.push({ text: `${index + 1} ${text}` });
  } else {
    lines.push({
      text: document?.reason || "此内容请退出 VR 后在 2D 代码工作区查看。",
      tone: "muted",
    });
  }
  return { id, kind: diff ? "diff" : "code", title, lines };
}
