import type { FileDiffMetadata } from "@pierre/diffs";
import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vite-plus/test";
import {
  GitWorkingChangeSchema,
  GitWorktreeContextSchema,
} from "../src/contracts/git-workbench-schemas";
import { SessionInfoSchema } from "../src/contracts/schemas";
import {
  joinCommitMessageDraft,
  mergeCommitMessageDrafts,
  splitCommitMessageDraft,
} from "../src/domains/workspace/git-commit-draft-model";
import {
  createReworkDraftItem,
  extractDiffRangeSnippet,
  filterActiveWorktreeSessions,
  formatCommitDraftAssistPrompt,
  formatReworkFeedbackMarkdown,
  readGitReworkItems,
  resolveSnapshotVersionDescriptor,
  resolveStagingVersionDescriptor,
  writeGitReworkItems,
} from "../src/domains/workspace/git-rework-model";
import { isChangeReviewed } from "../src/domains/workspace/git-workbench-state";
import {
  computeLocalReviewProgress,
  findNextUnreviewedTaskFile,
  findNextUnreviewedWorkingChange,
} from "../src/hooks/workspace/git-review-sync.svelte";

const sampleContext = GitWorktreeContextSchema.parse({
  repositoryId: "repo-1",
  repositoryName: "webmux",
  worktreeId: "wt-feat",
  worktreePath: "/work/webmux.worktrees/wt-feat",
  worktreeKind: "linked",
  checkedOutBranch: "feat/v1-collab",
  headOid: "1111111111111111111111111111111111111111",
  headState: "branch",
  ahead: 1,
  behind: 0,
  authorName: "Rain Chan",
  authorEmail: "rain@example.invalid",
});

describe("git commit draft model", () => {
  it("splits and rejoins subject and body without losing multiline content", () => {
    const raw = "feat(git): sync commit draft\n\n- optimistic lock\n- 409 card";
    const parsed = splitCommitMessageDraft(raw);
    expect(parsed).toEqual({
      subject: "feat(git): sync commit draft",
      body: "- optimistic lock\n- 409 card",
    });
    expect(joinCommitMessageDraft(parsed.subject, parsed.body)).toBe(raw);
  });

  it("merges local and remote drafts preserving both sides when distinct", () => {
    const merged = mergeCommitMessageDrafts(
      { subject: "feat(git): local subject", body: "local note A" },
      { subject: "feat(git): remote subject", body: "remote note B" },
    );
    expect(merged.subject).toBe(
      "feat(git): local subject / feat(git): remote subject",
    );
    expect(merged.body).toContain("--- 本机当前输入 ---\nlocal note A");
    expect(merged.body).toContain("--- 另一设备已保存版本 ---\nremote note B");
  });
});

describe("git review progress & next unreviewed jump", () => {
  const changes = [
    GitWorkingChangeSchema.parse({
      id: "staged:a.go",
      path: "a.go",
      group: "staged",
      status: "M",
      hunkStageAvailable: true,
      identityKey: "v1",
    }),
    GitWorkingChangeSchema.parse({
      id: "unstaged:b.go",
      path: "b.go",
      group: "unstaged",
      status: "M",
      hunkStageAvailable: true,
      identityKey: "v1",
    }),
    GitWorkingChangeSchema.parse({
      id: "untracked:c.go",
      path: "c.go",
      group: "untracked",
      status: "?",
      hunkStageAvailable: false,
      identityKey: "v1",
    }),
  ];

  it("computes local review progress and wraps around to next unreviewed item", () => {
    const reviewedPaths = ["staged:a.go::v=v1"];
    const summary = computeLocalReviewProgress(
      "uncommitted",
      changes,
      [],
      reviewedPaths,
      isChangeReviewed,
    );
    expect(summary).toEqual({ total: 3, reviewed: 1, remaining: 2 });

    const nextFromB = findNextUnreviewedWorkingChange(
      changes,
      "unstaged:b.go",
      (c) => isChangeReviewed(reviewedPaths, c),
    );
    expect(nextFromB?.id).toBe("untracked:c.go");

    const wrappedFromC = findNextUnreviewedWorkingChange(
      changes,
      "untracked:c.go",
      (c) => isChangeReviewed(reviewedPaths, c),
    );
    expect(wrappedFromC?.id).toBe("unstaged:b.go");
  });

  it("auto-invalidates local review token when identityKey changes", () => {
    const reviewedPaths = ["unstaged:b.go::v=v1"];
    expect(isChangeReviewed(reviewedPaths, changes[1]!)).toBe(true);
    expect(
      isChangeReviewed(reviewedPaths, { ...changes[1]!, identityKey: "v2" }),
    ).toBe(false);
  });

  it("finds next unreviewed task baseline file", () => {
    const taskFiles = [
      {
        oldPath: "",
        newPath: "one.ts",
        status: "M" as const,
        binary: false,
        additions: 2,
        deletions: 1,
        hunks: [],
      },
      {
        oldPath: "",
        newPath: "two.ts",
        status: "A" as const,
        binary: false,
        additions: 5,
        deletions: 0,
        hunks: [],
      },
    ];
    const next = findNextUnreviewedTaskFile(
      taskFiles,
      "one.ts",
      (p) => p === "one.ts",
    );
    expect(next).toBe("two.ts");
  });
});

class MemoryStorage implements Storage {
  data = new Map<string, string>();
  get length() {
    return this.data.size;
  }
  clear() {
    this.data.clear();
  }
  getItem(key: string) {
    return this.data.get(key) ?? null;
  }
  key(index: number) {
    return [...this.data.keys()][index] ?? null;
  }
  removeItem(key: string) {
    this.data.delete(key);
  }
  setItem(key: string, value: string) {
    this.data.set(key, value);
  }
}

describe("git rework feedback anti-spoofing & session routing", () => {
  beforeEach(() => {
    vi.stubGlobal("localStorage", new MemoryStorage());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("extracts snippet lines accurately from FileDiffMetadata", () => {
    const fakeDiff = {
      additionLines: ["line 1\n", "line 2\n", "line 3\n"],
      deletionLines: ["old 1\n", "old 2\n"],
    } as unknown as FileDiffMetadata;

    const extracted = extractDiffRangeSnippet(fakeDiff, {
      side: "additions",
      start: 3,
      end: 2,
    });
    expect(extracted).toEqual({
      side: "new",
      startLine: 2,
      endLine: 3,
      snippet: "line 2\nline 3",
    });
  });

  it("distinguishes live worktree line numbers from index, HEAD, task-base and historical commits", () => {
    expect(
      resolveStagingVersionDescriptor({
        group: "unstaged",
        side: "new",
        headOid: sampleContext.headOid,
        liveWorktreeIdentity: "blob:abc",
      }),
    ).toEqual({
      sourceKind: "worktree",
      versionLabel: "当前工作区",
      capturedWorktreeIdentity: "blob:abc",
    });

    expect(
      resolveStagingVersionDescriptor({
        group: "staged",
        side: "old",
        headOid: sampleContext.headOid,
      }),
    ).toEqual({
      sourceKind: "head",
      versionLabel: "HEAD (1111111) 删除侧",
    });

    expect(
      resolveSnapshotVersionDescriptor({
        isTaskScope: true,
        side: "old",
        leftLabel: "2222222",
        rightLabel: "WORKTREE",
        baseOid: "2222222",
        targetOid: "WORKTREE",
      }),
    ).toEqual({
      sourceKind: "task-base",
      versionLabel: "任务起点 (2222222) 旧侧",
    });

    expect(
      resolveSnapshotVersionDescriptor({
        isTaskScope: false,
        side: "new",
        leftLabel: "2222222",
        rightLabel: "1111111",
        baseOid: "2222222",
        targetOid: "1111111",
      }),
    ).toEqual({
      sourceKind: "commit",
      versionLabel: "历史提交 1111111",
    });
  });

  it("formats structured rework Markdown with explicit non-worktree and stale warnings", () => {
    const liveItem = createReworkDraftItem({
      path: "internal/api/git.go",
      sourceKind: "worktree",
      versionLabel: "当前工作区",
      startLine: 18,
      endLine: 20,
      codeSnippet: "if err != nil {\n\treturn err\n}",
      userNote: "请映射 ErrConflict 到 409。",
      capturedWorktreeIdentity: "blob:123",
    });
    const histItem = createReworkDraftItem({
      path: "internal/workspace/history.go",
      sourceKind: "commit",
      versionLabel: "历史提交 1111111",
      startLine: 42,
      endLine: 42,
      codeSnippet: "return nil",
      userNote: "补充空仓处理。",
    });

    const md = formatReworkFeedbackMarkdown(
      sampleContext,
      [liveItem, histItem],
      new Set(["internal/api/git.go"]),
    );
    expect(md).toContain("internal/api/git.go");
    expect(md).toContain("自摘录后已有新改动");
    expect(md).toContain("并非当前工作区实时行号");
    expect(md).toContain("请映射 ErrConflict 到 409。");
  });

  it("persists rework items per worktree and filters active sessions strictly", () => {
    const saved = writeGitReworkItems("repo-1", "wt-feat", [
      createReworkDraftItem({
        path: "main.go",
        sourceKind: "worktree",
        versionLabel: "当前工作区",
        startLine: 1,
        endLine: 1,
        codeSnippet: "package main",
      }),
    ]);
    expect(saved).toHaveLength(1);
    expect(readGitReworkItems("repo-1", "wt-feat")).toHaveLength(1);
    expect(readGitReworkItems("repo-1", "wt-other")).toHaveLength(0);

    const prompt = formatCommitDraftAssistPrompt(sampleContext, [
      GitWorkingChangeSchema.parse({
        id: "staged:main.go",
        path: "main.go",
        group: "staged",
        status: "M",
        hunkStageAvailable: true,
      }),
    ]);
    expect(prompt).toContain("不要代我执行 `git commit`");
    expect(prompt).toContain("- [M] main.go");

    const baseSess = {
      generation: "0123456789abcdef",
      title: "Agent",
      cwd: "/other",
      cmd: ["claude"],
      exitCode: null,
      listeningPorts: [],
      views: ["terminal" as const],
      view: "terminal" as const,
      transition: null,
      agent: null,
      preview: "",
      createdAt: "2026-04-12T08:00:00Z",
      lastActiveAt: "2026-04-12T08:01:00Z",
      cols: 80,
      rows: 24,
      clients: 1,
      attention: null,
      reportedCwd: null,
    };
    const sessions = [
      SessionInfoSchema.parse({
        ...baseSess,
        id: "aaaaaaa2",
        status: "running",
        state: "active",
        workspace: {
          repositoryId: "repo-1",
          repositoryName: "webmux",
          worktreeId: "wt-feat",
          worktreePath: "/work/webmux.worktrees/wt-feat",
          branch: "feat/v1-collab",
        },
      }),
      SessionInfoSchema.parse({
        ...baseSess,
        id: "bbbbbbb3",
        status: "exited",
        state: "active",
        workspace: {
          repositoryId: "repo-1",
          repositoryName: "webmux",
          worktreeId: "wt-feat",
          worktreePath: "/work/webmux.worktrees/wt-feat",
          branch: "feat/v1-collab",
        },
      }),
      SessionInfoSchema.parse({
        ...baseSess,
        id: "ccccccc4",
        status: "running",
        state: "active",
        workspace: {
          repositoryId: "repo-1",
          repositoryName: "webmux",
          worktreeId: "wt-main",
          worktreePath: "/work/webmux",
          branch: "main",
        },
      }),
    ];
    expect(
      filterActiveWorktreeSessions(
        sessions,
        "wt-feat",
        "/work/webmux.worktrees/wt-feat",
      ).map((s) => s.id),
    ).toEqual(["aaaaaaa2"]);
  });
});
