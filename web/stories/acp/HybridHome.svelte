<script lang="ts">
  import WorkspaceGroup from "../../src/components/WorkspaceGroup.svelte";
  import type { SessionInfo } from "../../src/schemas";
  import { now, session } from "../fixtures";

  // The existing home page group with two hybrid sessions beside a plain
  // shell: the card names the side and, on the chat side, previews the
  // latest agent message instead of terminal lines.
  let notice = $state("");
  const base = {
    ...session,
    cwd: "/home/developer/webmux",
    reportedCwd: null,
    clients: 0,
  };
  const sessions: SessionInfo[] = [
    {
      ...base,
      id: "hybridacp",
      title: "omp · 修复首页排序",
      cmd: ["omp"],
      status: "waiting",
      attention: {
        kind: "permission",
        source: "agent",
        since: "2026-01-01T11:59:00Z",
      },
      preview: "",
    },
    {
      ...base,
      id: "hybridtui",
      title: "OpenCode · 更新 README",
      cmd: ["opencode"],
      status: "running",
      attention: null,
      preview: [
        "› 继续：更新 README 中的排序说明",
        "",
        "  ✓ Read README.md",
        "  ● Editing README.md …",
      ].join("\n"),
    },
    {
      ...base,
      id: "plainshell",
      title: "bash",
      cmd: ["/bin/bash"],
      status: "idle",
      attention: null,
      preview: "$ git status\nOn branch main\nnothing to commit",
    },
  ];
</script>

<main class="mx-auto max-w-5xl px-4 py-8 sm:px-8 sm:py-12">
  <WorkspaceGroup
    title="webmux"
    path="/home/developer/webmux"
    {sessions}
    {now}
    hybrid={{
      hybridacp: {
        side: "acp",
        summary:
          "排序只比较了最近活动时间，没有考虑置顶。我先补测试，再修改 sortWorkspaces。需要授权运行 pnpm --dir web exec vitest。",
      },
      hybridtui: { side: "tui" },
    }}
    onclose={() => (notice = "此预览使用示例数据，未关闭真实会话。")}
  />
  {#if notice}<p role="status" class="mt-4 text-sm text-muted-foreground">
      {notice}
    </p>{/if}
</main>
