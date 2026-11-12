import type {
  GitBranchItem,
  GitHistoryRow,
  GitHistoryScope,
  GitWorkbenchViewState,
  GitWorktreeContext,
} from "../../contracts/git-workbench-schemas";
import type { GitSharedBranchActionKind } from "../../domains/workspace/git-branches-model";
import type { createReworkDraftItem } from "../../domains/workspace/git-rework-model";
import type { GitPaneScrollCallbacks } from "../../domains/workspace/git-workbench-memory";
import type { GitLogDiffController } from "../../hooks/workspace/git-log-diff.svelte";

export type GitLogFilterFields = Pick<
  GitWorkbenchViewState,
  | "scope"
  | "branchFilter"
  | "searchQuery"
  | "authorFilter"
  | "sinceFilter"
  | "untilFilter"
  | "pathFilter"
  | "refsChanged"
  | "selectionNotice"
  | "compareBaseMarkOid"
>;

export interface GitLogFilterCallbacks {
  onRefreshHistory?: () => void;
  onScopeChange: (scope: GitHistoryScope) => void;
  onSearchChange: (q: string) => void;
  onAuthorChange?: (author: string) => void;
  onSinceChange?: (since: string) => void;
  onUntilChange?: (until: string) => void;
  onPathFilterChange: (path: string) => void;
  onClearFilters?: () => void;
  onJumpToHead?: () => void;
  onDismissSelectionNotice?: () => void;
}

export interface GitLogPaneProps
  extends
    GitPaneScrollCallbacks,
    GitLogFilterFields,
    GitLogFilterCallbacks,
    Pick<
      GitWorkbenchViewState,
      | "historyState"
      | "nextCursor"
      | "historyLoadingMore"
      | "commitDetails"
      | "selectedCommitOid"
      | "selectedParentByCommit"
      | "selectedCommitFilePath"
      | "compareMode"
    > {
  context?: GitWorktreeContext;
  historyReason?: string;
  taskBaseline?: GitWorkbenchViewState["taskBaseline"];
  liveFiltered?: boolean;
  detailLoading?: boolean;
  commits: readonly GitHistoryRow[];
  branches: readonly GitBranchItem[];
  checkedOutBranch: string | null;
  logDiff?: GitLogDiffController;
  preferredShowBranches?: boolean;
  branchTreeWidth?: number;
  diffHeight?: number;
  diffCollapsed?: boolean;
  showAuthorColumn?: boolean;
  twoPaneSizes?: [number, number];
  onShowBranchesChange?: (open: boolean) => void;
  onBranchTreeWidthChange?: (width: number) => void;
  onDiffHeightChange?: (height: number) => void;
  onDiffCollapsedChange?: (collapsed: boolean) => void;
  onTwoPaneSizesChange?: (sizes: [number, number]) => void;
  onBranchFilterChange: (branch: string | null) => void;
  onBranchAction?: (
    kind: GitSharedBranchActionKind,
    branch: GitBranchItem,
  ) => void;
  onLoadMoreHistory?: () => void;
  onToggleFavorite?: (branch: string) => void;
  onSelectCommit: (oid: string) => void;
  onSelectParent: (commitOid: string, parentOid: string) => void;
  onSelectCommitFile: (path: string) => void;
  onMarkCompareBase?: (oid: string | null) => void;
  onStartCompare?: (baseOid: string, targetOid: string) => void;
  onLoadMoreCompareFiles?: () => void;
  onSwapCompareDirection?: () => void;
  onExitCompare?: () => void;
  onSetTaskBaseline?: (baseOid?: string) => void;
  onCherryPickCommit?: (oid: string) => void;
  onRevertCommit?: (oid: string) => void;
  onAddReworkItem?: (
    input: Parameters<typeof createReworkDraftItem>[0],
    options?: { openSheet?: boolean },
  ) => void;
  onRequestCheckout: (branch: string) => void;
  onOpenOccupiedWorktree: (worktreeId: string, path: string) => void;
}
