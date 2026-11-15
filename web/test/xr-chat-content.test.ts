import { expect, test } from "vite-plus/test";
import type { ChatContent } from "../src/contracts/schemas";
import { emptyChat, type ToolEntry } from "../src/domains/chat/chat-model";
import { xrChatPanel } from "../src/hooks/terminal/xr-chat-content";

test.each<{
  name: string;
  content: ChatContent;
  added: string[];
  removed: string[];
}>([
  {
    name: "created file",
    content: { type: "diff", newText: "new line" },
    added: ["+ new line"],
    removed: [],
  },
  {
    name: "deleted file",
    content: { type: "diff", oldText: "old line" },
    added: [],
    removed: ["- old line"],
  },
  {
    name: "patch-only content",
    content: { type: "diff", text: "@@ -1 +1 @@\n-old\n+new" },
    added: [],
    removed: [],
  },
])(
  "XR $name has no phantom absent-side changes",
  ({ content, added, removed }) => {
    const tool: ToolEntry = {
      type: "tool",
      id: "edit",
      kind: "edit",
      title: "Edit",
      status: "completed",
      content: [content],
      locations: [],
      partial: false,
    };
    const panel = xrChatPanel({
      model: { ...emptyChat(), entries: [tool] },
      state: undefined,
      draft: "",
      answering: false,
      answer: () => Promise.resolve(),
      openForm() {},
    });
    expect(
      panel.lines
        .filter((line) => line.tone === "added")
        .map((line) => line.text),
    ).toEqual(added);
    expect(
      panel.lines
        .filter((line) => line.tone === "removed")
        .map((line) => line.text),
    ).toEqual(removed);
  },
);
