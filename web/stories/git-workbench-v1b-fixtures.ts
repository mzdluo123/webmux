import {
  GitCommitDraftSchema,
  type SessionInfo,
  SessionInfoSchema,
} from "../src/contracts/schemas";
import type { GitCommitDraftConflictState } from "../src/domains/workspace/git-commit-draft-model";
import { session as baseSession } from "./fixtures";
import type { GitWorkbenchScenario } from "./git-workbench-fixtures";

export function buildGitStoryV1bExtras(scenario: GitWorkbenchScenario): {
  initialDraftConflict: GitCommitDraftConflictState | null;
  simulatedActiveSessions: SessionInfo[];
} {
  const activeSessions: SessionInfo[] = [
    SessionInfoSchema.parse({
      ...baseSession,
      id: "aaaaaaa2",
      title: "Claude Code · 修复暂存与审阅同步",
      cwd: "/work/webmux",
      cmd: ["claude"],
      status: "waiting",
      views: ["terminal", "chat"],
      view: "terminal",
      workspace: {
        repositoryId: "repo-story",
        repositoryName: "webmux",
        worktreeId: "wt-main",
        worktreePath: "/work/webmux",
        branch: "main",
      },
    }),
    SessionInfoSchema.parse({
      ...baseSession,
      id: "bbbbbbb3",
      title: "Codex · 补充端到端回归测试",
      cwd: "/work/webmux",
      cmd: ["codex"],
      status: "idle",
      views: ["terminal", "chat"],
      view: "chat",
      workspace: {
        repositoryId: "repo-story",
        repositoryName: "webmux",
        worktreeId: "wt-main",
        worktreePath: "/work/webmux",
        branch: "main",
      },
    }),
  ];

  if (scenario === "commit-draft-conflict") {
    return {
      initialDraftConflict: {
        localSubject: "feat(git): sync cross-device commit drafts",
        localBody:
          "- preserve draft on preview expiry or hook failure\n- keep local edits side by side on 409 conflict",
        remoteDraft: GitCommitDraftSchema.parse({
          repositoryId: "repo-story",
          worktreeId: "wt-main",
          revision: 4,
          subject: "feat(git): add server-synced review progress & draft lock",
          body: "- bind review keys to blob OID and content sha256\n- clear draft only after real commit creation",
          updatedAt: "2026-04-12T08:12:00Z",
        }),
        message: "检测到另一设备已更新提交草稿（版本 v4），请选择保留策略。",
      },
      simulatedActiveSessions: activeSessions,
    };
  }

  return {
    initialDraftConflict: null,
    simulatedActiveSessions:
      scenario === "no-worktree" ? [] : activeSessions.slice(0, 1),
  };
}
