<script lang="ts">
  import * as Tabs from "$lib/components/ui/tabs";
  import type {
    GitBlameResult,
    GitFileHistoryRow,
  } from "../../contracts/schemas";
  import type {
    GitFileInspectorController,
    GitFileInspectorTab,
  } from "../../hooks/workspace/git-file-inspector-controller.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import GitBlameView from "./GitBlameView.svelte";
  import GitFileHistoryView from "./GitFileHistoryView.svelte";
  import GitRevisionCompareView from "./GitRevisionCompareView.svelte";

  let {
    inspector,
    hasWorktree = false,
    fixtureHistoryRows,
    fixtureBlame,
    onJumpToCommit,
    onClose,
  }: {
    inspector?: GitFileInspectorController;
    hasWorktree?: boolean;
    fixtureHistoryRows?: readonly GitFileHistoryRow[];
    fixtureBlame?: GitBlameResult | null;
    onJumpToCommit?: (oid: string) => void;
    onClose?: () => void;
  } = $props();

  let localTab = $state<GitFileInspectorTab>("history");
  let localSelectedOid = $state<string | null>(null);

  const activeTab = $derived(inspector ? inspector.tab : localTab);
  const activePath = $derived(
    inspector?.filePath ||
      fixtureBlame?.path ||
      fixtureHistoryRows?.[0]?.path ||
      "",
  );
  const rows = $derived(
    inspector ? inspector.historyRows : (fixtureHistoryRows ?? []),
  );
  const blameData = $derived(
    inspector ? inspector.blame.data : (fixtureBlame ?? null),
  );

  function setTab(next: string) {
    if (next === "history" || next === "blame" || next === "compare") {
      if (inspector) inspector.switchTab(next);
      else localTab = next;
    }
  }

  function handleClose() {
    if (inspector) inspector.close();
    onClose?.();
  }
</script>

<ResponsiveDialog
  title={`单文件溯源检视 · ${activePath}`}
  description="只读检视文件重命名追踪历史 (--follow)、逐行 Blame 溯源，或与任意分支/标签/提交对比。"
  drawerDescription="文件历史、Blame 溯源与版本比较。"
  truncateDrawerTitle
  class="sm:max-w-5xl"
  onclose={handleClose}
>
  <div
    class="flex h-[70vh] max-h-[640px] min-h-[360px] min-w-0 flex-col overflow-hidden rounded-lg border border-git-border bg-git-surface"
    data-testid="git-file-inspector-dialog"
  >
    <Tabs.Root
      value={activeTab}
      onValueChange={setTab}
      class="flex min-h-0 min-w-0 flex-1 flex-col"
    >
      <div class="border-b border-git-border bg-git-panel px-2.5">
        <Tabs.List
          variant="line"
          class="max-w-full flex-wrap group-data-horizontal/tabs:h-auto"
          aria-label="单文件溯源模式"
        >
          <Tabs.Trigger value="history" data-testid="git-inspector-tab-history">
            文件历史 (--follow)
          </Tabs.Trigger>
          <Tabs.Trigger value="blame" data-testid="git-inspector-tab-blame">
            逐行溯源 (Blame)
          </Tabs.Trigger>
          <Tabs.Trigger value="compare" data-testid="git-inspector-tab-compare">
            与版本比较
          </Tabs.Trigger>
        </Tabs.List>
      </div>

      <Tabs.Content
        value="history"
        class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden p-0"
      >
        <GitFileHistoryView
          {rows}
          selectedOid={inspector
            ? inspector.selectedHistoryOid
            : localSelectedOid}
          nextCursor={inspector?.historyNextCursor ?? ""}
          loading={inspector?.historyLoading ?? false}
          loadingMore={inspector?.historyLoadingMore ?? false}
          error={inspector?.historyError ?? ""}
          diffReader={inspector?.historyReader}
          onSelectRow={(row) => {
            if (inspector) inspector.selectHistoryRow(row);
            else localSelectedOid = row.oid;
          }}
          onLoadMore={inspector
            ? () => void inspector.loadMoreHistory()
            : undefined}
          onOpenBlameAt={inspector
            ? (oid, path) => {
                inspector.switchTab("blame");
                void inspector.blame.load(oid, path, false);
              }
            : undefined}
          onJumpToCommit={onJumpToCommit
            ? (oid) => {
                handleClose();
                onJumpToCommit(oid);
              }
            : undefined}
        />
      </Tabs.Content>

      <Tabs.Content
        value="blame"
        class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden p-0"
      >
        <GitBlameView
          data={blameData}
          loading={inspector?.blame.loading ?? false}
          error={inspector?.blame.error ?? ""}
          trail={inspector?.blame.trail ?? []}
          onStepBack={inspector
            ? (idx) => inspector.blame.stepBack(idx)
            : undefined}
          onReblameParent={inspector
            ? (prevOid, prevPath) =>
                void inspector.blame.load(prevOid, prevPath, true)
            : undefined}
          onInspectCommitInHistory={(commitOid) => {
            if (inspector) {
              inspector.switchTab("history");
              const found = inspector.historyRows.find(
                (r) => r.oid === commitOid,
              );
              if (found) inspector.selectHistoryRow(found);
            } else {
              localSelectedOid = commitOid;
              localTab = "history";
            }
          }}
          onJumpToCommit={onJumpToCommit
            ? (oid) => {
                handleClose();
                onJumpToCommit(oid);
              }
            : undefined}
        />
      </Tabs.Content>

      <Tabs.Content
        value="compare"
        class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden p-0"
      >
        <GitRevisionCompareView
          refs={inspector?.availableRefs ?? []}
          baseOid={inspector?.compareBaseOid ?? ""}
          targetKind={inspector?.compareTargetKind ?? "worktree"}
          targetOid={inspector?.compareTargetOid ?? ""}
          {hasWorktree}
          diffReader={inspector?.compareReader}
          onSelectBase={(oid, label) => inspector?.setCompareBase(oid, label)}
          onTargetKindChange={(kind) => inspector?.setCompareTargetKind(kind)}
          onTargetOidChange={(oid) => inspector?.setCompareTargetOid(oid)}
        />
      </Tabs.Content>
    </Tabs.Root>
  </div>
</ResponsiveDialog>
