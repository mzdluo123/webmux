<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconArchive from "~icons/tabler/archive";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconDownload from "~icons/tabler/download";
  import IconPlayerPlay from "~icons/tabler/player-play";
  import IconPlus from "~icons/tabler/plus";
  import IconRefresh from "~icons/tabler/refresh";
  import IconTrash from "~icons/tabler/trash";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Empty from "$lib/components/ui/empty";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as Resizable from "$lib/components/ui/resizable";
  import type {
    GitStashDetail,
    GitStashEntry,
    GitStashFileChange,
  } from "../../contracts/schemas";
  import type {
    GitCommitFile,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import type { GitDiffPanelReader } from "../../hooks/workspace/git-log-diff.svelte";
  import { stashFileKey } from "../../hooks/workspace/git-stash-controller.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitLogDiffPanel from "./GitLogDiffPanel.svelte";
  import GitPanelToolbar from "./GitPanelToolbar.svelte";

  let {
    context,
    entries,
    selectedStashOid = null,
    detail = null,
    selectedFileKey = null,
    diffReader,
    fallbackFilesByKey = {},
    loadingList = false,
    loadingDetail = false,
    diffMode = "unified",
    onDiffModeChange,
    onRefresh,
    onSelectStash,
    onSelectFile,
    onOpenCreate,
    onOpenApply,
    onOpenDrop,
  }: {
    context: GitWorktreeContext;
    entries: readonly GitStashEntry[];
    selectedStashOid?: string | null;
    detail?: GitStashDetail | null;
    selectedFileKey?: string | null;
    diffReader?: GitDiffPanelReader;
    fallbackFilesByKey?: Record<string, GitCommitFile>;
    loadingList?: boolean;
    loadingDetail?: boolean;
    diffMode?: "unified" | "split";
    onDiffModeChange?: (mode: "unified" | "split") => void;
    onRefresh?: () => void;
    onSelectStash: (oid: string) => void;
    onSelectFile: (file: GitStashFileChange) => void;
    onOpenCreate?: () => void;
    onOpenApply?: (entry: GitStashEntry, mode: "apply" | "pop") => void;
    onOpenDrop?: (entry: GitStashEntry) => void;
  } = $props();

  const desktop = new MediaQuery("(min-width: 1024px)");
  let mobileStepOverride = $state<{
    oid: string;
    step: "detail" | "diff";
  } | null>(null);

  const readonlyMode = $derived(
    context.headState === "readonly" || !context.worktreePath,
  );
  const activeEntry = $derived(
    entries.find((e) => e.oid === selectedStashOid) ?? entries[0] ?? null,
  );
  const mobileStep = $derived<"list" | "detail" | "diff">(
    activeEntry && mobileStepOverride?.oid === activeEntry.oid
      ? mobileStepOverride.step
      : "list",
  );
  const activeDetail = $derived(
    detail && activeEntry && detail.entry.oid === activeEntry.oid
      ? detail
      : null,
  );
  const activeFile = $derived.by(() => {
    if (!activeDetail) return null;
    if (selectedFileKey) {
      const found = activeDetail.files.find(
        (f) => stashFileKey(f) === selectedFileKey,
      );
      if (found) return found;
    }
    return activeDetail.files[0] ?? null;
  });
  const activeFallbackFile = $derived.by<GitCommitFile | null>(() => {
    if (!activeFile) return null;
    const key = stashFileKey(activeFile);
    if (fallbackFilesByKey[key]) return fallbackFilesByKey[key]!;
    const path = activeFile.newPath || activeFile.oldPath;
    return {
      oldPath: activeFile.oldPath || path,
      newPath: path,
      status:
        activeFile.status === "A" ||
        activeFile.status === "D" ||
        activeFile.status === "R"
          ? activeFile.status
          : "M",
      binary: activeFile.binary,
      additions: 0,
      deletions: 0,
      hunks: [],
    };
  });
</script>

{#snippet stashListSection()}
  <section
    class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface"
    aria-label="仓库贮藏列表"
    data-testid="git-stash-list-pane"
  >
    <GitPanelToolbar>
      <div class="flex min-w-0 flex-1 items-center gap-1.5">
        <span class="truncate text-xs font-semibold text-foreground">
          贮藏列表 ({entries.length})
        </span>
      </div>
      <div class="flex shrink-0 items-center gap-1">
        {#if onOpenCreate}
          <Button
            size="xs"
            variant="git-primary"
            disabled={readonlyMode}
            title={readonlyMode
              ? "无工作树只读模式下无法创建贮藏，请先选择工作树"
              : "将当前工作树改动打包保存为新贮藏"}
            onclick={onOpenCreate}
            data-testid="git-stash-open-create-btn"
          >
            <IconPlus aria-hidden="true" />
            新建贮藏…
          </Button>
        {/if}
        {#if onRefresh}
          <Button
            size="icon-sm"
            variant="ghost"
            aria-label="刷新贮藏列表"
            title="刷新贮藏列表"
            disabled={loadingList}
            onclick={onRefresh}
          >
            <IconRefresh aria-hidden="true" />
          </Button>
        {/if}
      </div>
    </GitPanelToolbar>

    {#if readonlyMode}
      <div class="border-b border-git-border bg-git-panel p-2.5">
        <InlineNotice tone="info">
          当前处于无工作树只读模式，可检视所有贮藏详情与差异或丢弃旧条目；如需新建、应用或弹出贮藏，请切换到具体工作树。
        </InlineNotice>
      </div>
    {/if}

    <div class="min-h-0 flex-1 space-y-1 overflow-y-auto p-2">
      {#if loadingList && entries.length === 0}
        <p class="p-3 text-xs text-muted-foreground" role="status">
          正在读取仓库 refs/stash 条目…
        </p>
      {:else if entries.length === 0}
        <div class="p-3">
          <Empty.Root variant="outline">
            <Empty.Header>
              <Empty.Media variant="icon">
                <IconArchive aria-hidden="true" />
              </Empty.Media>
              <Empty.Title>暂无贮藏记录</Empty.Title>
              <Empty.Description>
                工作树有未提交改动时，可点击「新建贮藏…」或「提交」视图中的快捷按钮临时封存现场。
              </Empty.Description>
            </Empty.Header>
          </Empty.Root>
        </div>
      {:else}
        {#each entries as item (item.oid)}
          {@const selected = activeEntry?.oid === item.oid}
          <ListButton
            variant="git-row"
            aria-selected={selected}
            data-stash-row={item.stashRef}
            onclick={() => {
              onSelectStash(item.oid);
              if (!desktop.current) {
                mobileStepOverride = { oid: item.oid, step: "detail" };
              }
            }}
          >
            <div class="flex min-w-0 flex-1 flex-col gap-1">
              <div class="flex min-w-0 flex-wrap items-center gap-1.5">
                <Badge variant={selected ? "info" : "outline"}>
                  <span class="font-mono">{item.stashRef}</span>
                </Badge>
                <span class="font-mono text-xs text-muted-foreground">
                  {item.shortOid}
                </span>
                {#if item.branch}
                  <Badge variant="secondary" class="max-w-full">
                    <span class="truncate font-mono">{item.branch}</span>
                  </Badge>
                {/if}
                {#if item.hasUntracked}
                  <Badge variant="warning">含未跟踪 (S^3)</Badge>
                {/if}
              </div>
              <p class="truncate text-xs font-medium text-foreground">
                {item.summary}
              </p>
              <span class="text-xs text-muted-foreground">
                {item.createdAt}
              </span>
            </div>
          </ListButton>
        {/each}
      {/if}
    </div>
  </section>
{/snippet}

{#snippet stashDetailHeader(entry: GitStashEntry, isMobile: boolean)}
  <div
    class="space-y-2 border-b border-git-border bg-git-panel px-3 py-2 text-xs"
  >
    {#if isMobile}
      <div>
        <Button
          size="xs"
          variant="ghost"
          onclick={() => (mobileStepOverride = null)}
        >
          <IconArrowLeft aria-hidden="true" />
          返回贮藏列表
        </Button>
      </div>
    {/if}

    <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
      <div class="min-w-0 flex-1 space-y-0.5">
        <div class="flex flex-wrap items-center gap-1.5">
          <Badge variant="info">
            <span class="font-mono">{entry.stashRef}</span>
          </Badge>
          <span class="font-mono text-muted-foreground">{entry.shortOid}</span>
          {#if entry.baseOid}
            <span class="font-mono text-muted-foreground">
              基准 {entry.baseOid.slice(0, 7)}
            </span>
          {/if}
        </div>
        <h3 class="font-semibold wrap-anywhere text-foreground">
          {entry.summary}
        </h3>
      </div>

      <div class="flex flex-wrap items-center gap-1.5">
        {#if onOpenApply}
          <Button
            size="xs"
            variant="git-primary"
            wrap
            disabled={readonlyMode}
            title={readonlyMode
              ? "请先切换到目标工作树后再弹出贮藏"
              : "安全两步弹出：无冲突应用成功后才自动清理此贮藏"}
            onclick={() => onOpenApply(entry, "pop")}
            data-testid="git-stash-pop-btn"
          >
            <IconPlayerPlay aria-hidden="true" />
            安全弹出… (Pop)
          </Button>
          <Button
            size="xs"
            variant="outline"
            wrap
            disabled={readonlyMode}
            title={readonlyMode
              ? "请先切换到目标工作树后再应用贮藏"
              : "应用改动到当前工作树并保留原贮藏条目"}
            onclick={() => onOpenApply(entry, "apply")}
            data-testid="git-stash-apply-btn"
          >
            <IconDownload aria-hidden="true" />
            仅应用… (Apply)
          </Button>
        {/if}
        {#if onOpenDrop}
          <Button
            size="xs"
            variant="destructive-outline"
            wrap
            onclick={() => onOpenDrop(entry)}
            data-testid="git-stash-drop-btn"
          >
            <IconTrash aria-hidden="true" />
            丢弃… (Drop)
          </Button>
        {/if}
      </div>
    </div>
  </div>
{/snippet}

{#snippet stashFilesList(isMobile: boolean)}
  <div class="min-h-0 flex-1 overflow-y-auto p-2">
    {#if loadingDetail && !activeDetail}
      <p class="p-2.5 text-xs text-muted-foreground" role="status">
        正在读取贮藏文件清单（含未跟踪第 3 父节点）…
      </p>
    {:else if !activeDetail || activeDetail.files.length === 0}
      <p class="p-2.5 text-xs text-muted-foreground">
        此贮藏条目中没有可展示的文件变更。
      </p>
    {:else}
      <div
        class="mb-1.5 flex flex-wrap items-center gap-2 px-1.5 text-xs text-muted-foreground"
      >
        <span>已跟踪变更 {activeDetail.trackedCount} 项</span>
        {#if activeDetail.untrackedCount > 0}
          <span>· 未跟踪文件 {activeDetail.untrackedCount} 项</span>
        {/if}
      </div>
      <div class="space-y-0.5">
        {#each activeDetail.files as file (stashFileKey(file))}
          {@const path = file.newPath || file.oldPath}
          {@const labelPath =
            file.oldPath && file.oldPath !== path
              ? `${file.oldPath} → ${path}`
              : path}
          {@const isSelected =
            activeFile && stashFileKey(activeFile) === stashFileKey(file)}
          <ListButton
            variant="git-row"
            aria-selected={Boolean(isSelected)}
            data-stash-file-row={path}
            data-untracked={file.untracked}
            onclick={() => {
              onSelectFile(file);
              if (isMobile && activeEntry) {
                mobileStepOverride = { oid: activeEntry.oid, step: "diff" };
              }
            }}
          >
            <Badge
              variant={file.status === "D"
                ? "destructive"
                : file.status === "A" || file.untracked
                  ? "info"
                  : "outline"}
            >
              {file.untracked ? "U" : file.status}
            </Badge>
            <span
              class="min-w-0 flex-1 truncate font-mono text-xs"
              title={labelPath}
            >
              {labelPath}
            </span>
            {#if file.untracked}
              <Badge variant="warning">未跟踪 (S^3)</Badge>
            {/if}
            {#if file.binary}
              <Badge variant="secondary">二进制</Badge>
            {/if}
          </ListButton>
        {/each}
      </div>
    {/if}
  </div>
{/snippet}

{#snippet stashDiffPanel(entry: GitStashEntry, isMobile: boolean)}
  <GitLogDiffPanel
    logDiff={diffReader}
    fallbackFile={activeFallbackFile}
    fallbackLeftLabel={activeFile?.untracked
      ? "工作区未跟踪基准 (空)"
      : `基准 (${entry.baseOid?.slice(0, 7) ?? "HEAD"})`}
    fallbackRightLabel={`${entry.stashRef} (${entry.shortOid})`}
    backLabel={isMobile ? "返回贮藏文件列表" : undefined}
    presentation={isMobile ? "mobile" : "docked"}
    diffMode={isMobile ? "unified" : diffMode}
    onDiffModeChange={isMobile ? undefined : onDiffModeChange}
    onBack={isMobile
      ? () => (mobileStepOverride = { oid: entry.oid, step: "detail" })
      : undefined}
  />
{/snippet}

<div
  class="flex h-full min-h-0 min-w-0 flex-1 flex-col bg-git-surface"
  data-testid="git-stash-pane"
>
  {#if desktop.current}
    <Resizable.PaneGroup
      direction="horizontal"
      class="h-full min-h-0 min-w-0 flex-1"
    >
      <Resizable.Pane defaultSize={36} minSize={24} class="git-pane">
        {@render stashListSection()}
      </Resizable.Pane>
      <Resizable.Handle withHandle class="git-divider" />
      <Resizable.Pane defaultSize={64} minSize={36} class="git-pane">
        {#if !activeEntry}
          <div
            class="flex h-full items-center justify-center px-6 py-8 text-center text-xs text-muted-foreground"
          >
            选择左侧贮藏条目以检视完整已跟踪与未跟踪文件差异，或执行安全弹出。
          </div>
        {:else}
          <div class="flex h-full min-h-0 min-w-0 flex-col">
            {@render stashDetailHeader(activeEntry, false)}
            <Resizable.PaneGroup
              direction="vertical"
              class="min-h-0 min-w-0 flex-1"
            >
              <Resizable.Pane defaultSize={38} minSize={20} class="git-pane">
                {@render stashFilesList(false)}
              </Resizable.Pane>
              <Resizable.Handle withHandle class="git-divider" />
              <Resizable.Pane defaultSize={62} minSize={28} class="git-pane">
                {@render stashDiffPanel(activeEntry, false)}
              </Resizable.Pane>
            </Resizable.PaneGroup>
          </div>
        {/if}
      </Resizable.Pane>
    </Resizable.PaneGroup>
  {:else if mobileStep === "list" || !activeEntry}
    {@render stashListSection()}
  {:else if mobileStep === "detail"}
    <div class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface">
      {@render stashDetailHeader(activeEntry, true)}
      {@render stashFilesList(true)}
    </div>
  {:else}
    {@render stashDiffPanel(activeEntry, true)}
  {/if}
</div>
