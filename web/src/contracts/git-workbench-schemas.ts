import type {
  GitAbortOperationPreview,
  GitConflictFileDetail,
  GitDivergencePreview,
  GitInProgressOperation,
} from "./git-conflict-schemas";
import { z } from "zod";
import {
  GitCommitParentSchema,
  GitHistoryItemSchema,
  GitHistoryScopeSchema,
  GitRefBadgeSchema,
  GitStagingHunkLineSchema,
  GitStagingHunkSchema as GitWireStagingHunkSchema,
  type GitHistoryEdge,
  type GitHistoryScope,
  type GitHistoryStatus,
  type GitOperation,
  type GitOperationKind,
  type GitOperationStatus,
  type GitTaskBaseline,
} from "./git-api-schemas";

import { PushCheckSchema, type Remote } from "./workspace-schemas";
import type {
  GitStashApplyPreview,
  GitStashCreatePreview,
  GitStashDetail,
  GitStashDropPreview,
  GitStashEntry,
} from "./git-stash-schemas";
import type { GitBlameResult, GitFileHistoryRow } from "./git-v2-schemas";

export {
  GitHistoryScopeSchema,
  type GitHistoryEdge,
  type GitHistoryScope,
  type GitHistoryStatus,
  type GitOperation,
  type GitOperationKind,
  type GitOperationStatus,
  type GitTaskBaseline,
};

export const GitWorkbenchModeSchema = z.enum([
  "log",
  "commit",
  "branches",
  "stash",
]);
export type GitWorkbenchMode = z.infer<typeof GitWorkbenchModeSchema>;

export const GitReviewScopeSchema = z.enum(["uncommitted", "task"]);
export type GitReviewScope = z.infer<typeof GitReviewScopeSchema>;

const GitCommitMetadataFields = {
  authorName: z.string(),
  authorEmail: z.string(),
  authorAt: z.string(),
  committerName: z.string(),
  committerEmail: z.string(),
  committerAt: z.string(),
};

export const GitHistoryRowSchema = GitHistoryItemSchema.extend({
  committerEmail: z.string(),
  changedPaths: z.array(z.string()).default([]),
  branches: z.array(z.string()).default([]),
});
export type GitHistoryRow = z.infer<typeof GitHistoryRowSchema>;

const GitDiffLineSchema = GitStagingHunkLineSchema.extend({
  lineEnding: GitStagingHunkLineSchema.shape.lineEnding.default("lf"),
});

const GitStagingHunkSchema = GitWireStagingHunkSchema.extend({
  lines: z.array(GitDiffLineSchema),
});

const GitCommitFileSchema = z.object({
  changeId: z.string().optional(),
  oldPath: z.string().default(""),
  newPath: z.string(),
  status: z.enum(["A", "M", "D", "R", "C", "T"]),
  binary: z.boolean().default(false),
  additions: z.number().int().nonnegative().default(0),
  deletions: z.number().int().nonnegative().default(0),
  hunks: z.array(GitStagingHunkSchema).default([]),
});
export type GitCommitFile = z.infer<typeof GitCommitFileSchema>;

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
  identityKey: z.string().default(""),
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
  kind: z.enum(["local", "remote", "tag"]),
  head: z.string(),
  shortHead: z.string(),
  subject: z.string(),
  committedAt: z.string(),
  favorite: z.boolean().default(false),
  upstream: z.string().optional(),
  ahead: z.number().int().nonnegative().optional(),
  behind: z.number().int().nonnegative().optional(),
  upstreamGone: z.boolean().default(false),
  mergedIntoDefault: z.boolean().default(false),
  occupiedBy: GitBranchOccupancySchema.optional(),
  tagKind: z.enum(["lightweight", "annotated"]).optional(),
  tagOid: z.string().optional(),
  tagMessage: z.string().optional(),
  taggerName: z.string().optional(),
  taggerEmail: z.string().optional(),
  taggedAt: z.string().optional(),
  tagSigned: z.boolean().optional(),
});
export type GitBranchItem = z.infer<typeof GitBranchItemSchema>;

const GitIssueSchema = z.object({
  code: z.string(),
  message: z.string(),
});
export type GitIssue = z.infer<typeof GitIssueSchema>;

export const GitCommitOutcomeSchema = z.object({
  operationId: z.string().optional(),
  status: z.enum(["matched", "needs_review", "uncertain", "busy_locked"]),
  commitOid: z.string().optional(),
  expectedTreeOid: z.string().optional(),
  actualTreeOid: z.string().optional(),
  summary: z.string(),
  reasons: z.array(z.string()).default([]),
});
export type GitCommitOutcome = z.infer<typeof GitCommitOutcomeSchema>;

export type GitActiveOperationPhase =
  | "idle"
  | "validating"
  | "running"
  | "reconciling";

export interface GitActiveOperationState {
  phase: GitActiveOperationPhase;
  kind?: GitOperationKind;
  operationId?: string;
  clientRequestId?: string;
  summary: string;
}

export const GitCommitPreviewModelSchema = z.object({
  repositoryName: z.string(),
  worktreePath: z.string(),
  headState: z.enum(["branch", "detached", "unborn"]),
  branch: z.string(),
  headOid: z.string(),
  headMessage: z.string().default(""),
  headParents: z.array(z.string()).default([]),
  headPush: PushCheckSchema.optional(),
  amend: z.boolean().default(false),
  amendPushed: z.boolean().default(false),
  authorOverride: z.string().default(""),
  authorOverridden: z.boolean().default(false),
  envAuthorName: z.string().default(""),
  envAuthorEmail: z.string().default(""),
  signoff: z.boolean().default(false),
  signoffTrailer: z.string().default(""),
  expectedTreeOid: z.string(),
  authorName: z.string(),
  authorEmail: z.string(),
  committerName: z.string().default(""),
  committerEmail: z.string().default(""),
  signEnabled: z.boolean(),
  signFormat: z.string().optional(),
  operation: z.string().optional(),
  executableHooks: z.array(z.string()),
  stagedFiles: z.array(GitWorkingChangeSchema),
  unstagedCount: z.number().int().nonnegative(),
  untrackedCount: z.number().int().nonnegative(),
  conflictedCount: z.number().int().nonnegative(),
  followUpPush: z.boolean(),
  blockers: z.array(GitIssueSchema),
  warnings: z.array(GitIssueSchema),
  stale: z.boolean().default(false),
  dialogError: z.string().default(""),
});
export type GitCommitPreviewModel = z.infer<typeof GitCommitPreviewModelSchema>;

const GitPrecedingCommitSchema = z.object({
  commitOid: z.string(),
  shortOid: z.string(),
  subject: z.string(),
});
export type GitPrecedingCommit = z.infer<typeof GitPrecedingCommitSchema>;

export const GitPushPreviewModelSchema = z.object({
  repositoryName: z.string(),
  sourceBranch: z.string(),
  remote: z.string(),
  targetBranch: z.string(),
  pushUrl: z.string(),
  publish: z.boolean(),
  totalCommits: z.number().int().nonnegative().optional(),
  precedingCommit: GitPrecedingCommitSchema.optional(),
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
  upstreamGone: z.boolean().default(false),
  authorName: z.string().default(""),
  authorEmail: z.string().default(""),
  committerName: z.string().default(""),
  committerEmail: z.string().default(""),
  signEnabled: z.boolean().default(false),
  signFormat: z.string().default(""),
  executableHooks: z.array(z.string()).default([]),
  operationInProgress: z.string().nullable().default(null),
});
export type GitWorktreeContext = z.infer<typeof GitWorktreeContextSchema>;

export interface GitSelectionNotice {
  message: string;
  canJumpHead: boolean;
}

export interface GitCompareViewState {
  baseOid: string;
  targetOid: string;
  files: GitCommitFile[];
  nextCursor: string;
  loading: boolean;
  loadingMore: boolean;
  error: string;
}

export interface GitTaskReviewState {
  status: "idle" | "loading" | "ready" | "no_task_baseline" | "error";
  reason: string;
  baseline: GitTaskBaseline | null;
  headOid: string;
  headUnborn: boolean;
  commitCount: number;
  uncommittedCount: number;
  files: GitCommitFile[];
  selectedPath: string | null;
  searchQuery: string;
  nextCursor: string;
  loadingMore: boolean;
  mutating: boolean;
}

export interface GitWorkbenchViewState {
  mode: GitWorkbenchMode;
  context: GitWorktreeContext;
  historyState: GitHistoryStatus;
  historyReason: string;
  historyUpstream?: string;
  taskBaseline?: GitTaskBaseline | null;
  lastFetchedAt?: string;
  nextCursor?: string;
  refsChanged?: boolean;
  historyLoadingMore?: boolean;
  scope: GitHistoryScope;
  reviewScope?: GitReviewScope;
  taskReview?: GitTaskReviewState;
  branchFilter: string | null;
  searchQuery: string;
  authorFilter?: string;
  sinceFilter?: string;
  untilFilter?: string;
  pathFilter: string;
  commits: GitHistoryRow[];
  commitDetails: Record<string, GitCommitDetail>;
  selectedCommitOid: string | null;
  selectedParentByCommit: Record<string, string>;
  selectedCommitFilePath: string | null;
  selectionNotice?: GitSelectionNotice | null;
  compareBaseMarkOid?: string | null;
  compareMode?: GitCompareViewState | null;
  workingChanges: GitWorkingChange[];
  activeChangeId: string | null;
  reviewedPaths: string[];
  commitMessage: string;
  branches: GitBranchItem[];
  commitPreview: GitCommitPreviewModel | null;
  lastCommitOutcome: GitCommitOutcome | null;
  pushPreview: GitPushPreviewModel | null;
  operations?: GitOperation[];
  activeOperation?: GitActiveOperationState | null;
  inProgress?: GitInProgressOperation | null;
  divergencePreview?: GitDivergencePreview | null;
  abortPreview?: GitAbortOperationPreview | null;
  conflictDetail?: GitConflictFileDetail | null;
  conflictOpen?: boolean;
  remotes?: Remote[];
  stashes?: GitStashEntry[];
  selectedStashOid?: string | null;
  selectedStashDetail?: GitStashDetail | null;
  stashCreatePreview?: GitStashCreatePreview | null;
  stashApplyPreview?: GitStashApplyPreview | null;
  stashDropPreview?: GitStashDropPreview | null;
  commitAmend?: boolean;
  commitAmendPushed?: boolean;
  commitAuthorOverride?: string;
  commitSignoff?: boolean;
  commitSignoffTrailer?: string;
  amendDraftConflict?: {
    headMessage: string;
    currentDraft: string;
  } | null;
  fileInspectorOpen?: boolean;
  fixtureHistoryRows?: GitFileHistoryRow[];
  fixtureBlame?: GitBlameResult | null;
  statusBanner: string;
}

export interface GroupedWorkingChanges {
  conflicted: GitWorkingChange[];
  unstaged: GitWorkingChange[];
  staged: GitWorkingChange[];
  untracked: GitWorkingChange[];
}
