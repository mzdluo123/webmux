import {
  GitDiscardPreviewSchema,
  GitOperationSchema,
  SessionInfoSchema,
} from "../src/contracts/schemas";
import type { GitWorkbenchViewState } from "../src/contracts/git-workbench-schemas";
import { longPath, longToken, session as baseSession } from "./fixtures";
import type { GitWorkbenchScenario } from "./git-workbench-fixtures";

export function applyGitStoryV3Scenario(
  base: GitWorkbenchViewState,
  scenario: GitWorkbenchScenario,
  stressLongText = false,
): GitWorkbenchViewState {
  if (scenario === "sub-hunk-line-staging") {
    return {
      ...base,
      mode: "commit",
      defaultLinePickerOpen: true,
      operations: [
        GitOperationSchema.parse({
          id: "op-discard-v3",
          repositoryId: base.context.repositoryId,
          worktreeId: base.context.worktreeId ?? "wt-main",
          kind: "discard",
          status: "succeeded",
          startedAt: "2026-04-12T08:40:00Z",
          finishedAt: "2026-04-12T08:40:01Z",
          summary: "已丢弃 1 个文件的未暂存修改并创建短期恢复快照",
          recoveryOid: "9999aaaabbbbccccddddeeeeffff000011112222",
          recoveryCommand:
            "git checkout 9999aaa -- internal/workspace/staging.go",
          fetchResults: [],
          preview: {
            headOid: base.context.headOid,
            headState: "branch",
            branch: base.context.checkedOutBranch,
            worktreePath: base.context.worktreePath,
            expectedParents: [],
          },
          parents: [],
          reviewReasons: [],
        }),
        ...(base.operations ?? []),
      ],
      statusBanner:
        "逐行暂存模式：支持勾选任意增删行或按住 Shift 连续范围选择，独立暂存、取消暂存或丢弃所选行",
    };
  }

  if (scenario === "discard-preview-dialog") {
    const wtPath = stressLongText ? longPath : "/work/webmux";
    const trackedPath = stressLongText
      ? `internal/workspace/staging-${longToken}.go`
      : "internal/workspace/staging.go";
    const untrackedPath = stressLongText
      ? `docs/scratch-${longToken}.md`
      : "docs/scratch-notes.md";
    const activeAgent = SessionInfoSchema.parse({
      ...baseSession,
      id: "aaaaaaa2",
      title: `Claude Code · ${stressLongText ? longToken : "正在编辑工作区代码"}`,
      cwd: wtPath,
      cmd: ["claude"],
      status: "running",
      agent: {
        name: "claude",
        title: "Claude Code",
        icon: "claudecode",
        sessionId: "claude-v3",
      },
    });
    return {
      ...base,
      mode: "commit",
      discardPreview: GitDiscardPreviewSchema.parse({
        confirmationId: "story-discard-confirm-1",
        expiresAt: "2026-04-12T09:05:00Z",
        repositoryId: base.context.repositoryId,
        worktreeId: base.context.worktreeId ?? "wt-main",
        worktreePath: wtPath,
        scope: "files",
        trackedOverwritePaths: [trackedPath],
        untrackedDeletePaths: [untrackedPath],
        items: [
          {
            path: trackedPath,
            status: "M",
            action: "overwrite_tracked",
          },
          {
            path: untrackedPath,
            status: "?",
            action: "delete_untracked",
          },
        ],
        hunkCount: 0,
        lineCount: 0,
        summary:
          "丢弃 2 个路径的未暂存/未跟踪修改（覆盖已跟踪 1 个，删除未跟踪 1 个）",
        sessions: [activeAgent],
        blockers: [],
        warnings: [
          {
            code: "destructive_discard",
            message:
              "丢弃工作区修改会直接覆盖磁盘文件或删除未跟踪文件（已暂存的 index 条目不受影响）；执行前会自动保存短期恢复快照。",
          },
        ],
      }),
    };
  }

  return base;
}
