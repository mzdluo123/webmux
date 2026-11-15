import {
  GitCommitDraftSchema,
  GitReworkItemSchema,
  type GitReworkItem,
  type SessionInfo,
  SessionInfoSchema,
} from "../src/contracts/schemas";
import type { GitCommitDraftConflictState } from "../src/domains/workspace/git-commit-draft-model";
import { session as baseSession } from "./fixtures";
import type { GitWorkbenchScenario } from "./git-workbench-fixtures";

export function buildGitStoryV1bExtras(scenario: GitWorkbenchScenario): {
  initialDraftConflict: GitCommitDraftConflictState | null;
  initialReworkItems: GitReworkItem[];
  simulatedActiveSessions: SessionInfo[];
  initialReworkOpen: boolean;
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
      initialReworkItems: [],
      simulatedActiveSessions: activeSessions,
      initialReworkOpen: false,
    };
  }

  if (scenario === "rework-session-handoff") {
    return {
      initialDraftConflict: null,
      initialReworkItems: [
        GitReworkItemSchema.parse({
          id: "rw-story-1",
          path: "internal/workspace/staging.go",
          sourceKind: "worktree",
          versionLabel: "当前工作区",
          startLine: 10,
          endLine: 11,
          codeSnippet: " gate := s.gate.lock(loc.Path)\n+defer gate.unlock()",
          userNote: "锁获取失败时请先检查 ctx.Err()，避免重复包装超时错误。",
          capturedWorktreeIdentity: "blob:deadbeef1234",
          createdAt: Date.parse("2026-04-12T08:15:00Z"),
        }),
        GitReworkItemSchema.parse({
          id: "rw-story-2",
          path: "internal/workspace/history.go",
          sourceKind: "commit",
          versionLabel: "历史提交 1111111",
          startLine: 42,
          endLine: 43,
          codeSnippet:
            " func (s *Service) QueryHistory() {\n+\t// Real %P parent topology",
          userNote: "请同步更新空仓 unborn 分支的边界回归用例。",
          createdAt: Date.parse("2026-04-12T08:16:00Z"),
        }),
      ],
      simulatedActiveSessions: activeSessions,
      initialReworkOpen: true,
    };
  }

  return {
    initialDraftConflict: null,
    initialReworkItems: [],
    simulatedActiveSessions:
      scenario === "no-worktree" ? [] : activeSessions.slice(0, 1),
    initialReworkOpen: false,
  };
}
