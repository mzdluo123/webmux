import {
  GitBranchItemSchema,
  type GitWorkbenchViewState,
} from "../src/contracts/git-workbench-schemas";
import {
  GitBlameResultSchema,
  GitConflictFileDetailSchema,
  GitFileHistoryPageSchema,
  GitInProgressOperationSchema,
  GitStashDetailSchema,
  GitStashListSchema,
  RemoteSchema,
} from "../src/contracts/schemas";
import { longPath, longToken } from "./fixtures";
import type { GitWorkbenchScenario } from "./git-workbench-fixtures";

export function applyGitStoryV2bScenario(
  base: GitWorkbenchViewState,
  scenario: GitWorkbenchScenario,
  stressLongText = false,
): GitWorkbenchViewState {
  const wtPath = stressLongText ? longPath : "/work/webmux";
  const branchName = stressLongText
    ? `feat/${longToken}`
    : "feat/git-workbench-v2";

  if (scenario === "stash-view-untracked") {
    const stashList = GitStashListSchema.parse({
      stashes: [
        {
          stashRef: "stash@{0}",
          index: 0,
          oid: "aaaa1111bbbb2222cccc3333dddd4444eeee5555",
          shortOid: "aaaa111",
          baseOid: "1111111111111111111111111111111111111111",
          indexOid: "bbbb2222cccc3333dddd4444eeee5555ffff6666",
          untrackedOid: "cccc3333dddd4444eeee5555ffff666600001111",
          parents: [
            "1111111111111111111111111111111111111111",
            "bbbb2222cccc3333dddd4444eeee5555ffff6666",
            "cccc3333dddd4444eeee5555ffff666600001111",
          ],
          hasUntracked: true,
          branch: branchName,
          summary: `On ${branchName}: WIP auth session guard + untracked notes`,
          createdAt: "2026-04-12T08:30:00Z",
        },
        {
          stashRef: "stash@{1}",
          index: 1,
          oid: "dddd4444eeee5555ffff66660000111122223333",
          shortOid: "dddd444",
          baseOid: "2222222222222222222222222222222222222222",
          indexOid: "eeee5555ffff6666000011112222333344445555",
          parents: [
            "2222222222222222222222222222222222222222",
            "eeee5555ffff6666000011112222333344445555",
          ],
          hasUntracked: false,
          branch: "main",
          summary: "WIP on main: 2222222 chore: prepare base commit",
          createdAt: "2026-04-11T17:15:00Z",
        },
      ],
    });
    const detail = GitStashDetailSchema.parse({
      entry: stashList.stashes[0],
      authorName: "Rain Chan",
      authorEmail: "rain@example.invalid",
      trackedCount: 1,
      untrackedCount: 1,
      files: [
        {
          oldPath: "internal/workspace/stash.go",
          newPath: "internal/workspace/stash.go",
          status: "M",
          oldMode: "100644",
          newMode: "100644",
          oldOid: "1111aaaa",
          newOid: "2222bbbb",
          modeChanged: false,
          binary: false,
          untracked: false,
        },
        {
          oldPath: "",
          newPath: "docs/stash-design-draft.md",
          status: "A",
          oldMode: "000000",
          newMode: "100644",
          oldOid: "",
          newOid: "3333cccc",
          modeChanged: false,
          binary: false,
          untracked: true,
        },
      ],
    });
    return {
      ...base,
      mode: "stash",
      stashes: stashList.stashes,
      selectedStashOid: stashList.stashes[0]!.oid,
      selectedStashDetail: detail,
      statusBanner:
        "正在检视 stash@{0}（含第 3 父节点 S^3 未跟踪文件）：支持安全两步法弹出",
    };
  }

  if (scenario === "stash-pop-conflict") {
    const unstashOp = GitInProgressOperationSchema.parse({
      active: true,
      kind: "unstash",
      worktreeId: "wt-main",
      worktreePath: wtPath,
      currentBranch: branchName,
      detached: false,
      headOid: "1111111111111111111111111111111111111111",
      headSubject: "feat(git): stage S workbench foundations",
      sourceLabel: "贮藏变更 (stash@{0})",
      targetLabel: branchName,
      targetOid: "1111111111111111111111111111111111111111",
      targetSubject: "feat(git): stage S workbench foundations",
      conflictedCount: 1,
      conflictedFiles: [
        {
          path: "internal/workspace/staging.go",
          xy: "UU",
          kind: "text",
          stage1Mode: "100644",
          stage1Oid: "aaaaaaaa",
          stage2Mode: "100644",
          stage2Oid: "bbbbbbbb",
          stage3Mode: "100644",
          stage3Oid: "cccccccc",
        },
      ],
      canContinue: false,
      canAbort: false,
      continueBlockers: [],
    });
    return {
      ...base,
      mode: "commit",
      context: { ...base.context, operationInProgress: "unstash" },
      inProgress: unstashOp,
      conflictOpen: true,
      conflictDetail: GitConflictFileDetailSchema.parse({
        repositoryId: "repo-story",
        worktreeId: "wt-main",
        worktreePath: wtPath,
        path: "internal/workspace/staging.go",
        xy: "UU",
        kind: "text",
        textMergeable: true,
        operationKind: "unstash",
        identity: {
          path: "internal/workspace/staging.go",
          operationKind: "unstash",
          stage1Mode: "100644",
          stage1Oid: "aaaaaaaa",
          stage2Mode: "100644",
          stage2Oid: "bbbbbbbb",
          stage3Mode: "100644",
          stage3Oid: "cccccccc",
          leftStage: 2,
          rightStage: 3,
          worktreeMode: "100644",
          worktreeSha256: "unstash-sha256",
        },
        base: {
          side: "base",
          label: "共同祖先 (Base)",
          role: "共同祖先 (stage 1)",
          stage: 1,
          present: true,
          mode: "100644",
          blobOid: "aaaaaaaa11111111",
          sizeBytes: 92,
          binary: false,
          text: "const maxStashEntries = 50\nconst dropOnConflict = false\n",
          lineEnding: "lf",
        },
        left: {
          side: "left",
          label: `${branchName} · 1111111`,
          role: "当前工作树分支 (stage 2)",
          refName: branchName,
          commitOid: "1111111111111111111111111111111111111111",
          subject: "feat(git): stage S workbench foundations",
          stage: 2,
          present: true,
          mode: "100644",
          blobOid: "bbbbbbbb22222222",
          sizeBytes: 96,
          binary: false,
          text: "const maxStashEntries = 100\nconst dropOnConflict = false\n",
          lineEnding: "lf",
        },
        right: {
          side: "right",
          label: "stash@{0}",
          role: "贮藏变更 (stage 3)",
          stage: 3,
          present: true,
          mode: "100644",
          blobOid: "cccccccc33333333",
          sizeBytes: 96,
          binary: false,
          text: "const maxStashEntries = 200\nconst dropOnConflict = false\n",
          lineEnding: "lf",
        },
        segments: [
          {
            id: "seg-u-1",
            kind: "conflict",
            baseStart: 1,
            baseLines: ["const maxStashEntries = 50"],
            leftStart: 1,
            leftLines: ["const maxStashEntries = 100"],
            rightStart: 1,
            rightLines: ["const maxStashEntries = 200"],
          },
          {
            id: "seg-u-2",
            kind: "common",
            baseStart: 2,
            baseLines: ["const dropOnConflict = false"],
            leftStart: 2,
            leftLines: ["const dropOnConflict = false"],
            rightStart: 2,
            rightLines: ["const dropOnConflict = false"],
          },
        ],
        initialResultText:
          "const maxStashEntries = 50\nconst dropOnConflict = false\n",
        diskHasConflictMarker: true,
        diskModifiedFromBase: true,
      }),
      statusBanner:
        "安全弹出遇到合并冲突：原贮藏 stash@{0} 已完整保留（未丢弃），请解决工作树冲突后标记已解决",
    };
  }

  if (scenario === "commit-amend-pushed") {
    return {
      ...base,
      mode: "commit",
      context: {
        ...base.context,
        ahead: 0,
        behind: 0,
        committerName: "Rain Chan",
        committerEmail: "rain@example.invalid",
      },
      commitAmend: true,
      commitAmendPushed: true,
      commitAuthorOverride: "Co Author <co@example.invalid>",
      commitSignoff: true,
      commitSignoffTrailer: "Signed-off-by: Rain Chan <rain@example.invalid>",
      statusBanner:
        "已开启修正上一次提交（HEAD 已推送至远端）：一键提交并推送已自动禁用",
    };
  }

  if (scenario === "branches-tags-remotes") {
    return {
      ...base,
      mode: "branches",
      branches: [
        ...base.branches.filter((b) => b.kind !== "tag"),
        GitBranchItemSchema.parse({
          name: "v1.2.0",
          kind: "tag",
          head: "1111111111111111111111111111111111111111",
          shortHead: "1111111",
          subject: "Merge branch 'feat/git-workbench' into main",
          committedAt: "2026-04-12T08:05:00Z",
          tagKind: "annotated",
          tagOid: "9999888877776666555544443333222211110000",
          tagMessage: "Release v1.2.0: complete Git workbench V2 workflow.",
          taggerName: "Rain Chan",
          taggerEmail: "rain@example.invalid",
          taggedAt: "2026-04-12T08:10:00Z",
          tagSigned: true,
        }),
        GitBranchItemSchema.parse({
          name: "v1.1.0-rc1",
          kind: "tag",
          head: "2222222222222222222222222222222222222222",
          shortHead: "2222222",
          subject: "chore: prepare base commit",
          committedAt: "2026-04-10T09:00:00Z",
          tagKind: "lightweight",
          tagSigned: false,
        }),
      ],
      remotes: [
        RemoteSchema.parse({
          name: "origin",
          fetchUrl: "https://user:***@github.com/mzdluo123/webmux.git",
          pushUrl: "https://user:***@github.com/mzdluo123/webmux.git",
        }),
        RemoteSchema.parse({
          name: "upstream",
          fetchUrl: "https://github.com/upstream-org/webmux.git",
          pushUrl: "git@github.com:mzdluo123/webmux.git",
        }),
      ],
      statusBanner:
        "分支、标签与远程仓库管理：远端 URL 内嵌凭据已严格脱敏为 ***",
    };
  }

  if (scenario === "file-history-blame") {
    const historyPage = GitFileHistoryPageSchema.parse({
      status: "ready",
      path: "internal/workspace/commit_execute.go",
      startOid: "1111111111111111111111111111111111111111",
      commits: [
        {
          oid: "1111111111111111111111111111111111111111",
          shortOid: "1111111",
          parents: ["3333333333333333333333333333333333333333"],
          comparedParentOid: "3333333333333333333333333333333333333333",
          subject: "feat(git): add amend pushed protection and signoff",
          authorName: "Rain Chan",
          authorEmail: "rain@example.invalid",
          authorAt: "2026-04-12T08:05:00Z",
          status: "M",
          path: "internal/workspace/commit_execute.go",
        },
        {
          oid: "3333333333333333333333333333333333333333",
          shortOid: "3333333",
          parents: ["2222222222222222222222222222222222222222"],
          comparedParentOid: "2222222222222222222222222222222222222222",
          subject: "refactor(git): rename commit runner to commit_execute.go",
          authorName: "Alice",
          authorEmail: "alice@example.invalid",
          authorAt: "2026-04-11T11:30:00Z",
          status: "R",
          path: "internal/workspace/commit_execute.go",
          oldPath: "internal/workspace/commit_runner.go",
        },
      ],
    });
    const blame = GitBlameResultSchema.parse({
      commitOid: "1111111111111111111111111111111111111111",
      shortOid: "1111111",
      path: "internal/workspace/commit_execute.go",
      blobOid: "abcd1234ef567890",
      mode: "100644",
      kind: "text",
      totalLines: 3,
      commits: [
        {
          oid: "1111111111111111111111111111111111111111",
          shortOid: "1111111",
          summary: "feat(git): add amend pushed protection and signoff",
          authorName: "Rain Chan",
          authorEmail: "rain@example.invalid",
          authorAt: "2026-04-12T08:05:00Z",
          previousOid: "3333333333333333333333333333333333333333",
          previousPath: "internal/workspace/commit_execute.go",
        },
        {
          oid: "3333333333333333333333333333333333333333",
          shortOid: "3333333",
          summary: "refactor(git): rename commit runner to commit_execute.go",
          authorName: "Alice",
          authorEmail: "alice@example.invalid",
          authorAt: "2026-04-11T11:30:00Z",
          previousOid: "2222222222222222222222222222222222222222",
          previousPath: "internal/workspace/commit_runner.go",
        },
      ],
      lines: [
        {
          lineNumber: 1,
          origLine: 1,
          commitOid: "3333333333333333333333333333333333333333",
          shortOid: "3333333",
          origPath: "internal/workspace/commit_runner.go",
          previousOid: "2222222222222222222222222222222222222222",
          previousPath: "internal/workspace/commit_runner.go",
          text: "package workspace",
        },
        {
          lineNumber: 2,
          origLine: 2,
          commitOid: "1111111111111111111111111111111111111111",
          shortOid: "1111111",
          previousOid: "3333333333333333333333333333333333333333",
          previousPath: "internal/workspace/commit_execute.go",
          text: "const requirePushedAmendAck = true",
        },
        {
          lineNumber: 3,
          origLine: 3,
          commitOid: "1111111111111111111111111111111111111111",
          shortOid: "1111111",
          previousOid: "3333333333333333333333333333333333333333",
          previousPath: "internal/workspace/commit_execute.go",
          text: "const allowForcePush = false",
        },
      ],
    });
    return {
      ...base,
      mode: "log",
      fileInspectorOpen: true,
      fixtureHistoryRows: historyPage.commits,
      fixtureBlame: blame,
    };
  }

  return base;
}
