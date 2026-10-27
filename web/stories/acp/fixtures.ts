import { CodeChangeSchema } from "../../src/schemas";

// Sample data for the ACP chat-view prototype (issue #181). Shapes follow the
// ACP v1 session updates the view would render; nothing here is wired to a
// backend or to `src/schemas.ts` yet.

export type Agent = "omp" | "opencode";
export type Side = "acp" | "tui";
export type Scenario =
  | "conversation"
  | "permission"
  | "commands"
  | "cancelled"
  | "switching"
  | "terminal";

export type ToolKind = "read" | "edit" | "execute" | "search" | "other";
export type ToolStatus = "pending" | "in_progress" | "completed" | "failed";

export type PermissionOption = {
  optionId: string;
  name: string;
  kind: "allow_once" | "allow_always" | "reject_once" | "reject_always";
};

export type ToolCall = {
  toolCallId: string;
  kind: ToolKind;
  title: string;
  status: ToolStatus;
  diff?: { path: string; added: number; removed: number };
  output?: string;
};

export type TimelineEntry =
  | { type: "user"; id: string; text: string }
  | { type: "agent"; id: string; text: string }
  | { type: "thought"; id: string; seconds: number; text: string }
  | { type: "reads"; id: string; calls: ToolCall[] }
  | {
      type: "tool";
      id: string;
      call: ToolCall;
      permission?: PermissionOption[];
    }
  | {
      type: "compaction";
      id: string;
      before: string;
      after: string;
      summary: string;
    }
  | { type: "notice"; id: string; tone: "info" | "warning"; text: string }
  | { type: "turnEnd"; id: string; stopReason: "end_turn" | "cancelled" };

export type PlanEntry = {
  content: string;
  status: "pending" | "in_progress" | "completed";
};

export type ConfigOption = {
  id: string;
  name: string;
  value: string;
  options: { value: string; name: string }[];
};

export type Command = { name: string; description: string; hint?: string };
export type QueuedPrompt = { id: string; text: string };
export type Change = { path: string; added: number; removed: number };

export type ChatState = {
  agent: Agent;
  title: string;
  side: Side;
  running: boolean;
  steering: boolean;
  timeline: TimelineEntry[];
  plan: PlanEntry[];
  config: ConfigOption[];
  commands: Command[];
  queue: QueuedPrompt[];
  queuePaused: boolean;
  changes: Change[];
  usage: { used: number; size: number; cost: number };
  draft: string;
  agentSessionId: string;
  previousSessionId?: string;
};

const permission: PermissionOption[] = [
  { optionId: "once", name: "允许一次", kind: "allow_once" },
  { optionId: "always", name: "始终允许", kind: "allow_always" },
  { optionId: "reject", name: "拒绝", kind: "reject_once" },
];

const history: TimelineEntry[] = [
  {
    type: "user",
    id: "u1",
    text: "修复首页搜索的排序：置顶的工作区应排在最前。",
  },
  {
    type: "thought",
    id: "t1",
    seconds: 3,
    text: "先找到首页的排序函数，确认置顶状态从哪里来，再补一个回归测试。",
  },
  {
    type: "reads",
    id: "r1",
    calls: [
      ["web/src/pages/Home.svelte", "read"],
      ["web/src/homeWorktrees.ts", "read"],
      ["web/src/session-list.ts", "read"],
      ["sortWorkspaces", "search"],
    ].map(([title, kind], i) => ({
      toolCallId: `read-${i}`,
      kind: kind as ToolKind,
      title: title!,
      status: "completed",
    })),
  },
  {
    type: "compaction",
    id: "c1",
    before: "26.8k",
    after: "20.6k",
    summary:
      "已定位 sortWorkspaces；置顶状态来自 homeWorktrees。计划先写测试再改实现。",
  },
  {
    type: "agent",
    id: "a1",
    text: "排序只比较了最近活动时间，没有考虑置顶。我先补测试，再修改 `sortWorkspaces`。",
  },
  {
    type: "tool",
    id: "e1",
    call: {
      toolCallId: "edit-1",
      kind: "edit",
      title: "编辑 web/src/homeWorktrees.ts",
      status: "completed",
      diff: { path: "web/src/homeWorktrees.ts", added: 30, removed: 15 },
    },
  },
  {
    type: "tool",
    id: "e2",
    call: {
      toolCallId: "edit-2",
      kind: "edit",
      title: "编辑 web/src/pages/Home.svelte",
      status: "in_progress",
      diff: { path: "web/src/pages/Home.svelte", added: 12, removed: 3 },
    },
  },
];

const pendingRun: TimelineEntry = {
  type: "tool",
  id: "x1",
  call: {
    toolCallId: "exec-1",
    kind: "execute",
    title: "pnpm --dir web exec vitest run test/homeWorktrees.test.ts",
    status: "pending",
  },
  permission,
};

const plan: PlanEntry[] = [
  { content: "定位排序函数", status: "completed" },
  { content: "写回归测试", status: "completed" },
  { content: "修改实现", status: "in_progress" },
  { content: "运行测试", status: "pending" },
];

const configs: Record<Agent, ConfigOption[]> = {
  omp: [
    {
      id: "model",
      name: "模型",
      value: "claude-haiku-4-5",
      options: [
        { value: "claude-haiku-4-5", name: "haiku-4-5" },
        { value: "claude-sonnet-4-5", name: "sonnet-4-5" },
      ],
    },
    {
      id: "thinking",
      name: "思考",
      value: "off",
      options: ["off", "minimal", "low", "medium", "high"].map((value) => ({
        value,
        name: value,
      })),
    },
    {
      id: "mode",
      name: "模式",
      value: "default",
      options: [
        { value: "default", name: "default" },
        { value: "plan", name: "plan" },
      ],
    },
  ],
  opencode: [
    {
      id: "model",
      name: "模型",
      value: "deepseek-v4.1-flash",
      options: [
        { value: "deepseek-v4.1-flash", name: "deepseek-v4.1-flash" },
        { value: "deepseek-v4-pro", name: "deepseek-v4-pro" },
      ],
    },
    {
      id: "effort",
      name: "推理强度",
      value: "medium",
      options: ["low", "medium", "high"].map((value) => ({
        value,
        name: value,
      })),
    },
    {
      id: "mode",
      name: "模式",
      value: "build",
      options: [
        { value: "build", name: "build" },
        { value: "plan", name: "plan" },
      ],
    },
  ],
};

const commands: Command[] = [
  { name: "compact", description: "压缩上下文，保留摘要" },
  { name: "model", description: "切换模型", hint: "模型名称" },
  { name: "plan", description: "进入计划模式" },
  { name: "review", description: "审阅当前改动", hint: "文件或提交" },
  { name: "commit", description: "生成提交说明并提交" },
];

export function chatState(scenario: Scenario, agent: Agent): ChatState {
  const running = !["commands", "cancelled"].includes(scenario);
  const timeline = [...history];
  if (scenario === "permission") timeline.push(pendingRun);
  if (scenario === "commands" || scenario === "cancelled") {
    timeline[timeline.length - 1] = {
      type: "tool",
      id: "e2",
      call: {
        toolCallId: "edit-2",
        kind: "edit",
        title: "编辑 web/src/pages/Home.svelte",
        status: scenario === "cancelled" ? "failed" : "completed",
        diff: { path: "web/src/pages/Home.svelte", added: 12, removed: 3 },
      },
    };
    timeline.push(
      scenario === "cancelled"
        ? { type: "turnEnd", id: "end", stopReason: "cancelled" }
        : {
            type: "agent",
            id: "a2",
            text: "已修复排序并补充测试，12 个用例全部通过。置顶的工作区现在始终排在最前。",
          },
    );
    if (scenario === "commands")
      timeline.push({ type: "turnEnd", id: "end", stopReason: "end_turn" });
  }
  return {
    agent,
    title: agent === "omp" ? "omp · webmux" : "OpenCode · webmux",
    side: scenario === "terminal" ? "tui" : "acp",
    running,
    steering: agent === "omp",
    timeline,
    plan:
      scenario === "commands"
        ? plan.map((entry) => ({ ...entry, status: "completed" }))
        : plan,
    config: configs[agent],
    commands,
    queue:
      scenario === "conversation" || scenario === "cancelled"
        ? [
            { id: "q1", text: "然后把 README 里的排序说明也更新一下" },
            { id: "q2", text: "最后运行一次 make test-fast" },
          ]
        : [],
    queuePaused: scenario === "cancelled",
    changes: [
      { path: "web/src/pages/Home.svelte", added: 12, removed: 3 },
      { path: "web/src/homeWorktrees.ts", added: 30, removed: 15 },
    ],
    usage: { used: 26_800, size: 200_000, cost: 0.07 },
    draft: scenario === "commands" ? "/" : "",
    agentSessionId:
      agent === "omp" ? "01a17e26c4b84f0e9d2a" : "ses_7f3c2a91d0e4b5",
    previousSessionId:
      scenario === "terminal"
        ? agent === "omp"
          ? "01a17e25a9f04c11b7e3"
          : "ses_6e1b0f82c3d7a4"
        : undefined,
  };
}

export const terminalLines = [
  " omp  claude-haiku-4-5 · webmux",
  "",
  "› 修复首页搜索的排序：置顶的工作区应排在最前。",
  "",
  "  ✓ Read web/src/pages/Home.svelte",
  "  ✓ Read web/src/homeWorktrees.ts",
  "  ✓ Edit web/src/homeWorktrees.ts (+30 −15)",
  "  ✓ Edit web/src/pages/Home.svelte (+12 −3)",
  "",
  "  排序已修复，测试通过。",
  "",
  "› /new",
  "",
  "  New session started.",
  "",
  "› 继续：更新 README 中的排序说明",
  "  ● Editing README.md …",
  "",
].join("\n");

// The same two files as the chat's change summary, for the code panel.
export const codeChanges = [
  ["web/src/pages/Home.svelte", "M"],
  ["web/src/homeWorktrees.ts", "M"],
].map(([path, status], i) =>
  CodeChangeSchema.parse({
    nodeId: `acp-change-${i}`,
    changeId: `acp-change-${i}`,
    group: "unstaged",
    oldPath: path,
    newPath: path,
    status,
    oldOid: "a".repeat(40),
    newOid: "b".repeat(40),
    oldMode: "100644",
    newMode: "100644",
  }),
);
