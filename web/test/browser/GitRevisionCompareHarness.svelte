<script lang="ts">
  import { untrack } from "svelte";
  import GitRevisionCompareView from "../../src/components/workspace/GitRevisionCompareView.svelte";
  import type { GitBranchItem } from "../../src/contracts/git-workbench-schemas";
  import type { GitDiffPanelReader } from "../../src/hooks/workspace/git-log-diff.svelte";

  let {
    refs = [],
    initialBaseOid = "2222222222222222222222222222222222222222",
    initialTargetKind = "worktree",
    initialTargetOid = "3333333333333333333333333333333333333333",
    initialPhase = "loading",
    hasWorktree = true,
  }: {
    refs?: readonly GitBranchItem[];
    initialBaseOid?: string;
    initialTargetKind?: "worktree" | "rev";
    initialTargetOid?: string;
    initialPhase?: "idle" | "loading" | "error";
    hasWorktree?: boolean;
  } = $props();

  let baseOid = $state(untrack(() => initialBaseOid));
  let targetKind = $state<"worktree" | "rev">(untrack(() => initialTargetKind));
  let targetOid = $state(untrack(() => initialTargetOid));
  let phase = $state<"idle" | "loading" | "error">(untrack(() => initialPhase));

  const diffReader: GitDiffPanelReader = {
    get state() {
      return {
        phase,
        leftLabel: baseOid.slice(0, 7),
        rightLabel:
          targetKind === "worktree" ? "worktree" : targetOid.slice(0, 7),
        baseOid,
        targetOid: targetKind === "worktree" ? "WORKTREE" : targetOid,
        path: "internal/workspace/history.go",
        oldPath: "internal/workspace/history.go",
        status: "M" as const,
        snapshotId: "snap-1",
        fileDiff: null,
        rawDiff: null,
        reason: phase === "error" ? "临时读取超时" : "",
        downloadingSide: null,
      };
    },
    downloadSnapshotSide: async () => {},
    retryActiveDiff: () => {
      phase = "loading";
    },
  };
</script>

<div class="git-workbench flex h-[560px] w-full flex-col bg-background">
  <GitRevisionCompareView
    {refs}
    {baseOid}
    {targetKind}
    {targetOid}
    {hasWorktree}
    {diffReader}
    onSelectBase={(oid) => {
      baseOid = oid;
    }}
    onTargetKindChange={(kind) => {
      targetKind = kind;
    }}
    onTargetOidChange={(oid) => {
      targetOid = oid;
    }}
  />
</div>
