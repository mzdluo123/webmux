import { z } from "zod";

export const GitWorkbenchModeSchema = z.enum(["log", "commit", "branches"]);
export type GitWorkbenchMode = z.infer<typeof GitWorkbenchModeSchema>;

export const GitHistoryScopeSchema = z.enum([
  "all",
  "branch",
  "to-push",
  "incoming",
]);
export type GitHistoryScope = z.infer<typeof GitHistoryScopeSchema>;

const GitRefBadgeSchema = z.object({
  name: z.string(),
  kind: z.enum(["head", "local", "remote", "tag"]),
  worktreeId: z.string().optional(),
});

const GitHistoryEdgeSchema = z.object({
  fromLane: z.number().int().nonnegative(),
  toLane: z.number().int().nonnegative(),
  targetOid: z.string(),
  kind: z.enum(["direct", "indirect", "boundary"]),
});
export type GitHistoryEdge = z.infer<typeof GitHistoryEdgeSchema>;

const GitCommitMetadataFields = {
  authorName: z.string(),
  authorEmail: z.string(),
  authorAt: z.string(),
  committerName: z.string(),
  committerEmail: z.string(),
  committerAt: z.string(),
};

export const GitHistoryRowSchema = z.object({
  oid: z.string(),
  shortOid: z.string(),
  parents: z.array(z.string()),
  lane: z.number().int().nonnegative(),
  activeLanes: z.array(z.number().int().nonnegative()),
  edges: z.array(GitHistoryEdgeSchema),
  directParentsMissing: z.boolean(),
  subject: z.string(),
  ...GitCommitMetadataFields,
  refs: z.array(GitRefBadgeSchema),
  changedPaths: z.array(z.string()).default([]),
  branches: z.array(z.string()).default([]),
});
export type GitHistoryRow = z.infer<typeof GitHistoryRowSchema>;

const GitDiffLineSchema = z.object({
  kind: z.enum(["context", "delete", "add"]),
  oldLine: z.number().int().positive().optional(),
  newLine: z.number().int().positive().optional(),
  text: z.string(),
  lineEnding: z.enum(["lf", "crlf", "none"]).default("lf"),
});
export type GitDiffLine = z.infer<typeof GitDiffLineSchema>;

const GitStagingHunkSchema = z.object({
  id: z.string(),
  header: z.string(),
  oldStart: z.number().int().nonnegative(),
  oldLines: z.number().int().nonnegative(),
  newStart: z.number().int().nonnegative(),
  newLines: z.number().int().nonnegative(),
  lines: z.array(GitDiffLineSchema),
});
export type GitStagingHunk = z.infer<typeof GitStagingHunkSchema>;

const GitCommitFileSchema = z.object({
  oldPath: z.string().default(""),
  newPath: z.string(),
  status: z.enum(["A", "M", "D", "R", "C", "T"]),
  binary: z.boolean().default(false),
  additions: z.number().int().nonnegative().default(0),
  deletions: z.number().int().nonnegative().default(0),
  hunks: z.array(GitStagingHunkSchema).default([]),
});
export type GitCommitFile = z.infer<typeof GitCommitFileSchema>;

const GitCommitParentSchema = z.object({
  oid: z.string(),
  shortOid: z.string(),
  subject: z.string(),
});

export const GitCommitDetailSchema = z.object({
  oid: z.string(),
  shortOid: z.string(),
  subject: z.string(),
  body: z.string().default(""),
  ...GitCommitMetadataFields,
  distinctCommitter: z.boolean(),
  parents: z.array(GitCommitParentSchema),
  selectedParent: z.string(),
  refs: z.array(GitRefBadgeSchema),
  filesByParent: z.record(z.string(), z.array(GitCommitFileSchema)),
});
export type GitCommitDetail = z.infer<typeof GitCommitDetailSchema>;

const GitChangeGroupSchema = z.enum([
  "conflicted",
  "unstaged",
  "staged",
  "untracked",
]);
export type GitChangeGroup = z.infer<typeof GitChangeGroupSchema>;

export const GitWorkingChangeSchema = z.object({
  id: z.string(),
  path: z.string(),
  oldPath: z.string().optional(),
  group: GitChangeGroupSchema,
  status: z.enum(["M", "A", "D", "R", "U", "?"]),
  binary: z.boolean().default(false),
  hunkStageAvailable: z.boolean(),
  degradeReason: z.string().optional(),
  preStagedFromCli: z.boolean().default(false),
  hunks: z.array(GitStagingHunkSchema).default([]),
});
export type GitWorkingChange = z.infer<typeof GitWorkingChangeSchema>;

const GitBranchOccupancySchema = z.object({
  worktreeId: z.string(),
  worktreePath: z.string(),
  activeSessions: z.number().int().nonnegative(),
});

export const GitBranchItemSchema = z.object({
  name: z.string(),
  kind: z.enum(["local", "remote"]),
  head: z.string(),
  shortHead: z.string(),
  subject: z.string(),
  committedAt: z.string(),
  favorite: z.boolean().default(false),
  upstream: z.string().optional(),
  ahead: z.number().int().nonnegative().optional(),
  behind: z.number().int().nonnegative().optional(),
  upstreamGone: z.boolean().default(false),
  occupiedBy: GitBranchOccupancySchema.optional(),
});
export type GitBranchItem = z.infer<typeof GitBranchItemSchema>;

const GitIssueSchema = z.object({
  code: z.string(),
  message: z.string(),
});
export type GitIssue = z.infer<typeof GitIssueSchema>;

export const GitCommitOutcomeSchema = z.object({
  status: z.enum(["matched", "needs_review", "uncertain", "busy_locked"]),
  commitOid: z.string().optional(),
  expectedTreeOid: z.string().optional(),
  actualTreeOid: z.string().optional(),
  summary: z.string(),
  reasons: z.array(z.string()).default([]),
});
export type GitCommitOutcome = z.infer<typeof GitCommitOutcomeSchema>;

export const GitCommitPreviewModelSchema = z.object({
  repositoryName: z.string(),
  worktreePath: z.string(),
  headState: z.enum(["branch", "detached", "unborn"]),
  branch: z.string(),
  headOid: z.string(),
  expectedTreeOid: z.string(),
  authorName: z.string(),
  authorEmail: z.string(),
  signEnabled: z.boolean(),
  executableHooks: z.array(z.string()),
  stagedFiles: z.array(GitWorkingChangeSchema),
  unstagedCount: z.number().int().nonnegative(),
  untrackedCount: z.number().int().nonnegative(),
  conflictedCount: z.number().int().nonnegative(),
  followUpPush: z.boolean(),
  blockers: z.array(GitIssueSchema),
  warnings: z.array(GitIssueSchema),
});
export type GitCommitPreviewModel = z.infer<typeof GitCommitPreviewModelSchema>;

export const GitPushPreviewModelSchema = z.object({
  repositoryName: z.string(),
  sourceBranch: z.string(),
  remote: z.string(),
  targetBranch: z.string(),
  pushUrl: z.string(),
  publish: z.boolean(),
  commits: z.array(GitCommitDetailSchema),
  blockers: z.array(GitIssueSchema),
  warnings: z.array(GitIssueSchema),
});
export type GitPushPreviewModel = z.infer<typeof GitPushPreviewModelSchema>;

export const GitWorktreeContextSchema = z.object({
  repositoryId: z.string(),
  repositoryName: z.string(),
  worktreeId: z.string().nullable(),
  worktreePath: z.string().nullable(),
  worktreeKind: z.enum(["main", "linked", "bare", "none"]),
  checkedOutBranch: z.string().nullable(),
  headOid: z.string().nullable(),
  headState: z.enum(["branch", "detached", "unborn", "readonly"]),
  ahead: z.number().int().nonnegative().default(0),
  behind: z.number().int().nonnegative().default(0),
  upstream: z.string().nullable().default(null),
  authorName: z.string().default("Workspace Dev"),
  authorEmail: z.string().default("dev@example.invalid"),
  signEnabled: z.boolean().default(false),
  executableHooks: z.array(z.string()).default([]),
  operationInProgress: z.string().nullable().default(null),
});
export type GitWorktreeContext = z.infer<typeof GitWorktreeContextSchema>;

export interface GitWorkbenchViewState {
  mode: GitWorkbenchMode;
  context: GitWorktreeContext;
  historyState: "ready" | "empty" | "unborn" | "no_upstream" | "no_worktree";
  historyReason: string;
  scope: GitHistoryScope;
  branchFilter: string | null;
  searchQuery: string;
  pathFilter: string;
  commits: GitHistoryRow[];
  commitDetails: Record<string, GitCommitDetail>;
  selectedCommitOid: string | null;
  selectedParentByCommit: Record<string, string>;
  selectedCommitFilePath: string | null;
  workingChanges: GitWorkingChange[];
  activeChangeId: string | null;
  reviewedPaths: string[];
  commitMessage: string;
  branches: GitBranchItem[];
  commitPreview: GitCommitPreviewModel | null;
  lastCommitOutcome: GitCommitOutcome | null;
  pushPreview: GitPushPreviewModel | null;
  statusBanner: string;
}

export interface GroupedWorkingChanges {
  conflicted: GitWorkingChange[];
  unstaged: GitWorkingChange[];
  staged: GitWorkingChange[];
  untracked: GitWorkingChange[];
}
