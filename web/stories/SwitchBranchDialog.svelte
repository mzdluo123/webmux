<script lang="ts">
  import { untrack } from "svelte";
  import type { SwitchSource } from "../src/components/workspace/SwitchBranch.svelte";
  import SwitchBranchDialog from "../src/components/workspace/SwitchBranchDialog.svelte";
  import { Button } from "../src/lib/components/ui/button";
  import type {
    Branches,
    GitSmartCheckoutPreview,
    Repository,
    SwitchPreview,
    Worktree,
  } from "../src/contracts/schemas";
  import { longPath, longToken, session, worktree } from "./fixtures";

  export type SwitchStoryScenario =
    | "blockers"
    | "sessions"
    | "long-names"
    | "smart-idle"
    | "smart-agent";

  let {
    scenario = "blockers",
  }: {
    scenario?: SwitchStoryScenario;
  } = $props();
  let open = $state(true);
  const shown = untrack(() => scenario);

  const at = "2026-01-01T09:00:00Z";
  const head = "1234567890abcdef1234567890abcdef12345678";
  const target = "abcdef1234567890abcdef1234567890abcdef12";
  const long = shown === "long-names";
  const isSmart = shown === "smart-idle" || shown === "smart-agent";
  const current: Worktree = {
    ...worktree,
    kind: "linked",
    path: `${longPath}/current`,
    branch: long ? `feature/current-${longToken}` : "feature/current",
    head,
    dirty: isSmart,
    hasUntracked: isSmart,
    changes: isSmart
      ? { staged: 1, unstaged: 2, untracked: 1, conflicted: 0 }
      : worktree.changes,
    locked: false,
    lockReason: "",
    prunable: false,
    available: true,
    error: "",
    operation: null,
  };
  const other: Worktree = {
    ...current,
    id: "storybook-other",
    path: `${longPath}/other-${longToken}`,
    branch: long ? `feature/taken-${longToken}` : "feature/taken",
  };
  const repository: Repository = {
    id: "storybook-repository",
    name: `仓库 ${longToken}`,
    commonDir: `${longPath}/.git`,
    anchorPath: longPath,
    availability: "available",
    error: "",
    checkedAt: at,
    lastFetchedAt: at,
    initInstruction: "",
    worktrees: [current, other],
  };
  const chosen = long ? `feature/target-${longToken}` : "feature/target";
  const branch = (name: string, paths: string[] = []) => ({
    name,
    head: name === chosen ? target : head,
    worktreePaths: paths,
    subject: `提交 ${name}`,
    committedAt: at,
    upstream: null,
    ahead: null,
    behind: null,
    upstreamGone: false,
    mergedIntoDefault: false,
  });
  const branches: Branches = {
    branches: [
      branch(current.branch!, [current.path]),
      branch(other.branch!, [other.path]),
      branch(chosen),
    ],
    bases: [
      {
        name: `refs/remotes/origin/fix/${long ? longToken : "remote"}`,
        head: target,
        subject: `远程提交 ${longToken}`,
        kind: "remote",
        committedAt: at,
      },
    ],
    tags: [],
    defaultBase: head,
    defaultBaseSubject: "",
    defaultBranch: "main",
    defaultParent: longPath,
  };
  const activeAgentSessions = [
    { ...session, id: "abcdefga", status: "idle" as const },
    {
      ...session,
      id: "abcdefgb",
      title: `Claude Code · ${longToken}`,
      status: "running" as const,
      agent: {
        name: "claude",
        title: "Claude Code",
        icon: "claudecode",
        sessionId: "01a",
      },
    },
  ];
  const previews: Record<SwitchStoryScenario, Partial<SwitchPreview>> = {
    blockers: {
      blockers: [
        {
          code: "changes",
          message: `有未提交的修改（2 个未暂存），请先提交或贮藏：${longToken}`,
        },
        {
          code: "operation",
          message: "工作树正在变基，请先在终端完成或中止 rebase",
        },
      ],
      warnings: [
        { code: "untracked", message: "有未跟踪的文件；切换不会删除它们" },
        { code: "submodules", message: "仓库包含子模块，切换后不会自动更新" },
      ],
    },
    sessions: {
      confirmationId: "storybook-confirmation",
      expiresAt: "2026-01-01T12:05:00Z",
      sessions: activeAgentSessions,
      warnings: [
        {
          code: "sessions",
          message: "有 2 个运行中的会话；它们不会被关闭，会看到切换后的文件",
        },
        { code: "locked", message: `工作树已锁定：${longToken}` },
      ],
    },
    "long-names": {
      blockers: [
        {
          code: "occupied",
          message: `分支 ${chosen} 已被另一个工作树检出：${other.path}`,
          path: other.path,
        },
      ],
    },
    "smart-idle": {
      blockers: [
        {
          code: "changes",
          message: "有未提交的修改（已暂存 1、未暂存 2），直接切换会被阻止",
        },
      ],
      warnings: [{ code: "untracked", message: "有 1 个未跟踪文件" }],
    },
    "smart-agent": {
      sessions: activeAgentSessions,
      blockers: [
        {
          code: "changes",
          message: "有未提交的修改（已暂存 1、未暂存 2），直接切换会被阻止",
        },
      ],
      warnings: [
        {
          code: "sessions",
          message: "有 2 个运行中的会话；建议优先改为在新工作树打开",
        },
      ],
    },
  };
  const source: SwitchSource = {
    branches: async () => branches,
    commits: async () => ({
      commits: [
        { hash: head, subject: "当前提交", author: "dev", committedAt: at },
      ],
    }),
    preview: async (tree, next) => ({
      confirmationId: "",
      expiresAt: null,
      worktree: tree,
      target: {
        kind: next.kind,
        branch: next.kind === "local" ? next.branch : chosen,
        commit: target,
        subject: `目标提交 ${longToken}`,
        createsBranch: next.kind !== "local",
        upstream: null,
      },
      sessions: [],
      blockers: [],
      warnings: [],
      ...previews[shown],
    }),
    switch: async (tree) => ({ ...tree, branch: chosen }),
    ...(isSmart
      ? {
          smartPreview: async (
            tree,
            req,
          ): Promise<GitSmartCheckoutPreview> => ({
            confirmationId: "storybook-smart-confirm",
            expiresAt: "2026-01-01T12:05:00Z",
            repositoryId: repository.id,
            worktreeId: tree.id,
            worktree: tree,
            target: {
              kind: req.target.kind,
              branch: req.target.kind === "local" ? req.target.branch : chosen,
              commit: target,
              subject: `目标提交 ${longToken}`,
              createsBranch: req.target.kind !== "local",
              upstream: null,
            },
            includeUntracked: req.includeUntracked ?? true,
            stagedCount: 1,
            unstagedCount: 2,
            untrackedCount: 1,
            dirtyPaths: ["src/auth.ts", "src/session.ts", "notes.md"],
            hasActiveAgent: shown === "smart-agent",
            sessions: shown === "smart-agent" ? activeAgentSessions : [],
            blockers: [],
            warnings: [],
          }),
          smartCheckout: async (tree) => ({
            operationId: "op-smart-story",
            outcome: "completed",
            phase: "completed",
            stashOid: "9999aaaabbbbccccddddeeeeffff000011112222",
            stashRestored: true,
            stashDropped: true,
            stashPreserved: false,
            summary: `Smart Checkout 已完成：已携带修改切换至 ${chosen}`,
            inProgress: {
              active: false,
              worktreeId: tree.id,
              worktreePath: tree.path,
              headOid: target,
              headSubject: `目标提交 ${longToken}`,
              currentBranch: chosen,
              detached: false,
              conflictedCount: 0,
              conflictedFiles: [],
              canContinue: false,
              canAbort: false,
              continueBlockers: [],
            },
            worktree: { ...tree, branch: chosen },
          }),
        }
      : {}),
  };
</script>

<main class="mx-auto w-full max-w-5xl p-4" data-testid="switch-story">
  <Button onclick={() => (open = true)}>打开切换分支</Button>
  {#if open}
    <SwitchBranchDialog
      {repository}
      worktree={current}
      initialBranch={isSmart ? chosen : undefined}
      {source}
      onchange={() => {}}
      onnewworktree={() => {}}
      ongoto={() => {}}
      onopenconflict={() => {}}
      onclose={() => (open = false)}
    />
  {/if}
</main>
