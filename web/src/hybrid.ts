import type { HybridSupport, SessionHybrid } from "./schemas";

// User-facing text for the server's hybrid support reason codes. The rules
// themselves stay on the server (agent profiles); only the wording lives here.
const hybridSupportReasons: Record<
  NonNullable<HybridSupport["reason"]>,
  string
> = {
  unknown_agent: "未识别到支持的 Agent",
  unsupported_form: "命令写法不受支持",
  session_store: "该参数使聊天视图无法找到会话",
  directory_arg: "请在新建会话时选择目录，不要在命令中写目录",
  non_interactive: "该参数不启动交互界面",
};

export function hybridSupportReason(support: HybridSupport | null) {
  if (!support || support.supported) return "";
  return hybridSupportReasons[support.reason ?? "unknown_agent"];
}

export type HybridSideLabel = "终端" | "聊天" | "聊天·休眠" | "切换中";

/** Where a hybrid session currently runs, as shown on cards and pages. */
export function hybridSideLabel(hybrid: SessionHybrid): HybridSideLabel {
  if (hybrid.side === "switching" || hybrid.phase) return "切换中";
  if (hybrid.side === "acp") return hybrid.dormant ? "聊天·休眠" : "聊天";
  return "终端";
}
