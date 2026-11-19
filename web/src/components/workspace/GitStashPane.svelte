<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
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
  import { ScrollArea } from "$lib/components/ui/scroll-area";
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
  import { dockGroup, dockSplit } from "$lib/dock-layout";
  import { stashFileKey } from "../../hooks/workspace/git-stash-controller.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import DockPanel from "../shared/DockPanel.svelte";
  import DockWorkspace from "../shared/DockWorkspace.svelte";
  import GitLogDiffPanel from "./GitLogDiffPanel.svelte";
  import GitPanelToolbar from "./GitPanelToolbar.svelte";

  let {
    context,
    storageKey = "git.stash",
    entries,
    selectedStashOid = null,
    detail = null,
    selectedFileKey = null,
    diffReader,
    fallbackFilesByKey = {},
    loadingList = false,
    loadingDetail = false,
    onRefresh,
    onSelectStash,
    onSelectFile,
    onOpenCreate,
    onOpenApply,
    onOpenDrop,
  }: {
    context: GitWorktreeContext;
    storageKey?: string;
    entries: readonly GitStashEntry[];
    selectedStashOid?: string | null;
    detail?: GitStashDetail | null;
    selectedFileKey?: string | null;
    diffReader?: GitDiffPanelReader;
    fallbackFilesByKey?: Record<string, GitCommitFile>;
    loadingList?: boolean;
    loadingDetail?: boolean;
    onRefresh?: () => void;
    onSelectStash: (oid: string) => void;
    onSelectFile: (file: GitStashFileChange) => void;
    onOpenCreate?: () => void;
    onOpenApply?: (entry: GitStashEntry, mode: "apply" | "pop") => void;
    onOpenDrop?: (entry: GitStashEntry) => void;
  } = $props();

  const desktop = new MediaQuery("(min-width: 1024px)");
  const initial = dockSplit(
    "horizontal",
    dockGroup("list"),
    dockSplit("vertical", dockGroup("detail"), dockGroup("diff"), 0.38),
    0.36,
  );
  let dock = $state<ReturnType<typeof DockWorkspace>>();
  let mobilePanel = $state("list");
  let mobileEntry = $state<string>();

  const readonlyMode = $derived(
    context.headState === "readonly" || !context.worktreePath,
  );
  const activeEntry = $derived(
    entries.find((e) => e.oid === selectedStashOid) ?? entries[0] ?? null,
  );

  $effect(() => {
    if (mobileEntry && mobileEntry !== activeEntry?.oid) {
      mobileEntry = undefined;
      if (desktop.current) mobilePanel = "list";
      else dock?.show("list");
    }
  });

  function rememberMobileEntry(oid: string) {
    if (!desktop.current) mobileEntry = oid;
  }

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
    class="flex h-full min-h-0 min-w-0 flex-col bg-background"
    data-testid="git-stash-list-pane"
  >
    <GitPanelToolbar>
      <div class="flex min-w-0 flex-1 items-center gap-1.5">
        <span class="truncate text-xs font-semibold text-foreground">
          {m.s7_stash_list_heading({ count: entries.length })}
        </span>
      </div>
      <div class="flex shrink-0 items-center gap-1">
        {#if onOpenCreate}
          <Button
            size="sm"
            variant="default"
            disabled={readonlyMode}
            title={readonlyMode
              ? m.s7_stash_new_disabled_readonly()
              : m.s7_stash_new_tooltip()}
            onclick={onOpenCreate}
            data-testid="git-stash-open-create-btn"
          >
            <IconPlus aria-hidden="true" />
            {m.s7_stash_new_btn()}
          </Button>
        {/if}
        {#if onRefresh}
          <Button
            size="icon-sm"
            variant="ghost"
            aria-label={m.s7_stash_refresh_aria()}
            title={m.s7_stash_refresh_aria()}
            disabled={loadingList}
            onclick={onRefresh}
          >
            <IconRefresh aria-hidden="true" />
          </Button>
        {/if}
      </div>
    </GitPanelToolbar>

    {#if readonlyMode}
      <div class="border-b bg-background p-2.5">
        <InlineNotice tone="info">
          {m.s7_stash_readonly_notice()}
        </InlineNotice>
      </div>
    {/if}

    <ScrollArea class="min-h-8 flex-1">
      <div class="space-y-1 p-2">
        {#if loadingList && entries.length === 0}
          <p class="p-3 text-xs text-muted-foreground" role="status">
            {m.s7_stash_loading_list()}
          </p>
        {:else if entries.length === 0}
          <div class="p-3">
            <Empty.Root variant="outline">
              <Empty.Header>
                <Empty.Media variant="icon">
                  <IconArchive aria-hidden="true" />
                </Empty.Media>
                <Empty.Title>{m.s7_stash_empty_title()}</Empty.Title>
                <Empty.Description>
                  {m.s7_stash_empty_desc()}
                </Empty.Description>
              </Empty.Header>
            </Empty.Root>
          </div>
        {:else}
          {#each entries as item (item.oid)}
            {@const selected = activeEntry?.oid === item.oid}
            <ListButton
              variant="compact-row"
              aria-selected={selected}
              data-stash-row={item.stashRef}
              onclick={() => {
                rememberMobileEntry(item.oid);
                onSelectStash(item.oid);
                if (!desktop.current) dock?.show("detail");
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
                    <Badge variant="warning"
                      >{m.s7_stash_has_untracked_badge()}</Badge
                    >
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
    </ScrollArea>
  </section>
{/snippet}

{#snippet stashDetailHeader(entry: GitStashEntry, isMobile: boolean)}
  <div class="space-y-2 border-b bg-background px-3 py-2 text-xs">
    {#if isMobile}
      <div>
        <Button size="sm" variant="ghost" onclick={() => dock?.show("list")}>
          <IconArrowLeft aria-hidden="true" />
          {m.s7_stash_back_to_list()}
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
              {m.s7_stash_base_short({ oid: entry.baseOid.slice(0, 7) })}
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
            size="sm"
            variant="default"
            wrap
            disabled={readonlyMode}
            title={readonlyMode
              ? m.s7_stash_pop_disabled_readonly()
              : m.s7_stash_pop_tooltip()}
            onclick={() => {
              rememberMobileEntry(entry.oid);
              onOpenApply(entry, "pop");
            }}
            data-testid="git-stash-pop-btn"
          >
            <IconPlayerPlay aria-hidden="true" />
            {m.s7_stash_pop_btn()}
          </Button>
          <Button
            size="sm"
            variant="outline"
            wrap
            disabled={readonlyMode}
            title={readonlyMode
              ? m.s7_stash_apply_disabled_readonly()
              : m.s7_stash_apply_tooltip()}
            onclick={() => onOpenApply(entry, "apply")}
            data-testid="git-stash-apply-btn"
          >
            <IconDownload aria-hidden="true" />
            {m.s7_stash_apply_btn()}
          </Button>
        {/if}
        {#if onOpenDrop}
          <Button
            size="sm"
            variant="destructive-outline"
            wrap
            onclick={() => {
              rememberMobileEntry(entry.oid);
              onOpenDrop(entry);
            }}
            data-testid="git-stash-drop-btn"
          >
            <IconTrash aria-hidden="true" />
            {m.s7_stash_drop_btn()}
          </Button>
        {/if}
      </div>
    </div>
  </div>
{/snippet}

{#snippet stashFilesList()}
  <ScrollArea class="min-h-8 flex-1">
    <div class="p-2">
      {#if loadingDetail && !activeDetail}
        <p class="p-2.5 text-xs text-muted-foreground" role="status">
          {m.s7_stash_loading_detail()}
        </p>
      {:else if !activeDetail || activeDetail.files.length === 0}
        <p class="p-2.5 text-xs text-muted-foreground">
          {m.s7_stash_no_files()}
        </p>
      {:else}
        <div
          class="mb-1.5 flex flex-wrap items-center gap-2 px-1.5 text-xs text-muted-foreground"
        >
          <span
            >{m.s7_stash_tracked_count({
              count: activeDetail.trackedCount,
            })}</span
          >
          {#if activeDetail.untrackedCount > 0}
            <span
              >{m.s7_stash_untracked_count({
                count: activeDetail.untrackedCount,
              })}</span
            >
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
              variant="compact-row"
              aria-selected={Boolean(isSelected)}
              data-stash-file-row={path}
              data-untracked={file.untracked}
              onclick={() => {
                onSelectFile(file);
                if (!desktop.current) dock?.show("diff");
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
                <Badge variant="warning">{m.s7_stash_untracked_badge()}</Badge>
              {/if}
              {#if file.binary}
                <Badge variant="secondary">{m.s7_binary_badge()}</Badge>
              {/if}
            </ListButton>
          {/each}
        </div>
      {/if}
    </div>
  </ScrollArea>
{/snippet}

{#snippet stashDiffPanel(entry: GitStashEntry, isMobile: boolean)}
  <GitLogDiffPanel
    logDiff={diffReader}
    fallbackFile={activeFallbackFile}
    fallbackLeftLabel={activeFile?.untracked
      ? m.s7_stash_untracked_base_label()
      : m.s7_stash_fallback_base_label({
          base: entry.baseOid?.slice(0, 7) ?? "HEAD",
        })}
    fallbackRightLabel={`${entry.stashRef} (${entry.shortOid})`}
    backLabel={isMobile ? m.s7_stash_back_to_files() : undefined}
    presentation={isMobile ? "mobile" : "docked"}
    onBack={isMobile ? () => dock?.show("detail") : undefined}
  />
{/snippet}

<div
  class="flex h-full min-h-0 min-w-0 flex-1 flex-col bg-background"
  data-testid="git-stash-pane"
>
  <DockWorkspace
    {initial}
    {storageKey}
    compact={!desktop.current}
    bind:mobilePanel
    bind:this={dock}
  >
    <DockPanel id="list" title={m.s7_stash_list_aria()}>
      {@render stashListSection()}
    </DockPanel>

    <DockPanel id="detail" title={m.dock_git_stash_details()}>
      {#if activeEntry}
        <div class="flex h-full min-h-0 min-w-0 flex-col bg-background">
          {@render stashDetailHeader(activeEntry, !desktop.current)}
          {@render stashFilesList()}
        </div>
      {:else}
        {@render noStashSelected()}
      {/if}
    </DockPanel>

    <DockPanel id="diff" title={m.s6_log_diff_preview_placeholder()}>
      {#if activeEntry}
        {@render stashDiffPanel(activeEntry, !desktop.current)}
      {:else}
        {@render noStashSelected()}
      {/if}
    </DockPanel>
  </DockWorkspace>
</div>

{#snippet noStashSelected()}
  <div
    class="flex h-full items-center justify-center px-6 py-8 text-center text-xs text-muted-foreground"
  >
    {m.s7_stash_select_prompt()}
  </div>
{/snippet}
