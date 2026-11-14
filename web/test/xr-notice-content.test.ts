import { expect, test } from "vitest";
import { buildHybridNotices } from "../src/domains/sessions/hybrid";
import { xrNoticePanel } from "../src/hooks/terminal/xr-notice-content";

const idle = {
  error: "",
  notices: [],
  awaitingChoice: false,
  forceSwitchNote: "",
  openChoice() {},
};

test("unavailable switches and failed operations retain TerminalNotices ordering and text", () => {
  const notices = buildHybridNotices(
    undefined,
    "terminal",
    "切换失败：连接已关闭",
    "现在不能切到聊天：请先在 TUI 中发送一条消息",
    [],
    new Set(),
  );
  const panel = xrNoticePanel({
    ...idle,
    notices,
    error: "消息提交失败\n草稿未发送",
  });
  expect(panel?.lines).toEqual([
    { text: "切换失败：连接已关闭", tone: "error" },
    {
      text: "现在不能切到聊天：请先在 TUI 中发送一条消息",
      tone: "muted",
    },
    { text: "消息提交失败", tone: "error" },
    { text: "草稿未发送", tone: "error" },
  ]);
  expect(panel?.actions).toEqual([]);
});

test("successful chat retry removes its error without masking hybrid notices", () => {
  const notices = [
    { key: "retained", tone: "warning" as const, text: "切换操作仍待处理" },
  ];
  const failed = xrNoticePanel({ ...idle, notices, error: "提交失败" });
  expect(failed?.lines.at(-1)).toEqual({ text: "提交失败", tone: "error" });
  const retried = xrNoticePanel({ ...idle, notices });
  expect(retried?.lines).toEqual([{ text: "切换操作仍待处理", tone: "muted" }]);
  expect(xrNoticePanel(idle)).toBeUndefined();
});
