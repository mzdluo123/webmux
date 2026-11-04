// Sample chat history for the quick-start prototype: live hybrid sessions
// and archived records merged, newest activity first, as the sidebar would.
export type ChatStatus = "working" | "waiting" | "idle" | "ended";
export type ChatEntry = {
  id: string;
  title: string;
  agentIcon: string;
  agentTitle: string;
  status: ChatStatus;
};

export const statusLabels: Record<ChatStatus, string> = {
  working: "工作中",
  waiting: "等待",
  idle: "空闲",
  ended: "已结束",
};

export const history: ChatEntry[] = [
  ["s1", "修复首页置顶会话排序", "claudecode", "Claude Code", "working"],
  ["s2", "项目文档治理", "pi", "omp", "waiting"],
  ["s3", "Markdown 文件行长校验", "opencode", "OpenCode", "idle"],
  ["s4", "快照回放回调问题", "claudecode", "Claude Code", "idle"],
  [
    "s5",
    "终端缓冲区调整大小性能：减少重排时的整屏重绘并保留滚动位置",
    "codex",
    "Codex",
    "ended",
  ],
  ["s6", "CI 配置优化", "pi", "omp", "ended"],
  ["s7", "AGENTS.md 文件大小优化", "claudecode", "Claude Code", "ended"],
  ["s8", "移动端加载闪屏问题", "opencode", "OpenCode", "ended"],
  ["s9", "ACP 代码审查与接口抽象", "claudecode", "Claude Code", "ended"],
].map(([id, title, agentIcon, agentTitle, status]) => ({
  id: id!,
  title: title!,
  agentIcon: agentIcon!,
  agentTitle: agentTitle!,
  status: status as ChatStatus,
}));
