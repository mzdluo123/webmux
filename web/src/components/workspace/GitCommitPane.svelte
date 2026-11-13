<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconCheck from "~icons/tabler/check";
  import IconMinus from "~icons/tabler/minus";
  import IconPlus from "~icons/tabler/plus";
  import IconRestore from "~icons/tabler/restore";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as Resizable from "$lib/components/ui/resizable";
  import { Textarea } from "$lib/components/ui/textarea";
  import { groupWorkingChanges } from "../../domains/workspace/git-workbench-state";
  import {
    syncElementScrollTop,
    type GitPaneScrollCallbacks,
  } from "../../domains/workspace/git-workbench-memory";
  import type {
    GitChangeGroup,
    GitWorkingChange,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import GitDiffViewer from "./GitDiffViewer.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    context,
    changes,
    activeChangeId,
    reviewedPaths,
    commitMessage,
    paneSizes = [40, 60],
    diffMode,
    listScrollTop = 0,
    diffScrollTop = 0,
    onPaneSizesChange,
    onResetLayout,
    onDiffModeChange,
    onListScrollChange,
    onDiffScrollChange,
    onSelectChange,
    onToggleReviewed,
    onStageChange,
    onUnstageChange,
    onStageHunk,
    onUnstageHunk,
    onCommitMessageChange,
    onOpenCommitPreview,
  }: {
    context: GitWorktreeContext;
    changes: readonly GitWorkingChange[];
    activeChangeId: string | null;
    reviewedPaths: readonly string[];
    commitMessage: string;
    paneSizes?: [number, number];
    onPaneSizesChange?: (sizes: [number, number]) => void;
    onSelectChange: (id: string) => void;
    onToggleReviewed: (path: string) => void;
    onStageChange: (changeId: string) => void;
    onUnstageChange: (changeId: string) => void;
    onStageHunk: (changeId: string, hunkId: string) => void;
    onUnstageHunk: (changeId: string, hunkId: string) => void;
    onCommitMessageChange: (msg: string) => void;
    onOpenCommitPreview: (followUpPush: boolean) => void;
  } & GitPaneScrollCallbacks = $props();

  const desktop = new MediaQuery("(min-width: 1280px)");
  let mobileStep = $state<"changes" | "diff">("changes");
  let paneResetKey = $state(0);
  let listScrollEl = $state<HTMLDivElement>();

  $effect(() => syncElementScrollTop(listScrollEl, listScrollTop));

  const grouped = $derived(groupWorkingChanges(changes));
  const activeChange = $derived(
    changes.find((c) => c.id === activeChangeId) ?? changes[0] ?? null,
  );
  const readonlyMode = $derived(
    context.headState === "readonly" || !context.worktreePath,
  );

  const sections: { key: GitChangeGroup; title: string }[] = [
    { key: "conflicted", title: "冲突 (Conflicts)" },
    { key: "staged", title: "已暂存 (Staged · Git index)" },
    { key: "unstaged", title: "未暂存修改 (Unstaged)" },
    { key: "untracked", title: "未跟踪文件 (Untracked)" },
  ];
</script>

{#snippet changeRow(item: GitWorkingChange)}
  {@const isReviewed = reviewedPaths.includes(item.path)}
  {@const isReading = activeChange?.id === item.id}
  <div
    class="flex min-w-0 items-center gap-1 rounded-sm"
    data-change-id={item.id}
    data-reviewed={isReviewed}
    data-group={item.group}
  >
    <Button
      size="icon-xs"
      variant="ghost"
      aria-pressed={isReviewed}
      aria-label={`标记已审阅 ${item.path}`}
      title={isReviewed ? `已审阅 ${item.path}` : `标记已审阅 ${item.path}`}
      onclick={() => onToggleReviewed(item.path)}
    >
      <IconCheck aria-hidden="true" />
    </Button>

    <div class="min-w-0 flex-1">
      <ListButton
        variant="git-row"
        aria-selected={isReading}
        onclick={() => {
          onSelectChange(item.id);
          if (!desktop.current) mobileStep = "diff";
        }}
      >
        <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
          <Badge
            variant={item.group === "conflicted"
              ? "destructive"
              : item.group === "staged"
                ? "info"
                : "outline"}
          >
            {item.status}
          </Badge>
          <span
            class="min-w-[6ch] max-w-full flex-1 truncate font-mono"
            title={item.path}
          >
            {item.path}
          </span>
          {#if isReviewed}
            <Badge variant="secondary" class="max-w-full">
              <span class="truncate">已审阅</span>
            </Badge>
          {/if}
          {#if item.preStagedFromCli}
            <Badge variant="warning" class="max-w-full">
              <span class="truncate">CLI 预暂存</span>
            </Badge>
          {/if}
        </div>
      </ListButton>
    </div>

    {#if !readonlyMode}
      {#if item.group === "staged"}
        <Button
          size="icon-xs"
          variant="ghost"
          aria-label={`取消暂存文件 ${item.path}`}
          title={`取消暂存文件 ${item.path}`}
          onclick={() => onUnstageChange(item.id)}
        >
          <IconMinus aria-hidden="true" />
        </Button>
      {:else if item.group === "unstaged" || item.group === "untracked"}
        <Button
          size="icon-xs"
          variant="ghost"
          aria-label={`暂存文件 ${item.path}`}
          title={`暂存文件 ${item.path}`}
          onclick={() => onStageChange(item.id)}
        >
          <IconPlus aria-hidden="true" />
        </Button>
      {/if}
    {/if}
  </div>
{/snippet}

{#snippet changesAndComposerPane()}
  <section
    class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface"
    aria-label="工作树变更与提交"
  >
    <div
      class="flex min-w-0 items-center justify-between gap-2 border-b border-git-border bg-git-panel px-3 py-1.5"
    >
      <span class="min-w-0 truncate text-xs font-semibold text-foreground">
        变更树（阅读高亮 / 已审阅 / 真实暂存三态独立）
      </span>
      {#if desktop.current}
        <Button
          size="icon-sm"
          variant="ghost"
          aria-label="重置提交分栏比例"
          title="重置提交分栏比例"
          onclick={() => {
            onResetLayout?.();
            paneResetKey++;
          }}
        >
          <IconRestore aria-hidden="true" />
        </Button>
      {/if}
    </div>

    {#if readonlyMode}
      <div class="p-3">
        <InlineNotice tone="info">
          当前处于无工作树只读仓库上下文，无法暂存或创建提交。请切换到具体工作树。
        </InlineNotice>
      </div>
    {/if}

    <div
      bind:this={listScrollEl}
      class="min-h-0 flex-1 space-y-3 overflow-y-auto p-2"
      onscroll={(e) => onListScrollChange?.(e.currentTarget.scrollTop)}
    >
      {#each sections as sec (sec.key)}
        {@const list = grouped[sec.key]}
        {#if list.length > 0 || sec.key === "staged" || sec.key === "unstaged"}
          <div class="space-y-1">
            <div class="flex items-center justify-between px-1">
              <span class="text-xs font-semibold text-muted-foreground">
                {sec.title}
              </span>
              <Badge variant="outline">{list.length}</Badge>
            </div>
            {#if list.length === 0}
              <p class="px-2 py-1 text-xs text-muted-foreground">暂无条目</p>
            {:else}
              <div class="space-y-0.5">
                {#each list as entry (entry.id)}
                  {@render changeRow(entry)}
                {/each}
              </div>
            {/if}
          </div>
        {/if}
      {/each}
    </div>

    <div class="space-y-2 border-t border-git-border bg-git-panel p-3">
      <div
        class="flex min-w-0 flex-wrap items-center justify-between gap-1 text-xs text-muted-foreground"
      >
        <span class="truncate">
          作者：{context.authorName || "(未配置)"} &lt;{context.authorEmail ||
            "未配置"}&gt;
        </span>
        {#if context.signEnabled}
          <Badge variant="info" class="max-w-full">
            <span class="truncate">GPG/SSH 签名开启</span>
          </Badge>
        {/if}
      </div>

      <Textarea
        value={commitMessage}
        placeholder="输入提交标题与说明（提交内容严格来自真实 Git index 暂存区）…"
        aria-label="提交说明"
        disabled={readonlyMode}
        oninput={(e) => onCommitMessageChange(e.currentTarget.value)}
      />

      <div class="flex flex-wrap items-center justify-end gap-2">
        <Button
          variant="outline"
          disabled={readonlyMode || !commitMessage.trim()}
          onclick={() => onOpenCommitPreview(true)}
        >
          提交并推送…
        </Button>
        <Button
          variant="git-primary"
          disabled={readonlyMode || !commitMessage.trim()}
          onclick={() => onOpenCommitPreview(false)}
        >
          提交预览…
        </Button>
      </div>
    </div>
  </section>
{/snippet}

{#snippet diffPane()}
  <section
    class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface"
    aria-label="工作树文件差异与代码块暂存"
  >
    {#if !desktop.current}
      <div class="border-b border-git-border bg-git-panel p-2">
        <Button
          size="xs"
          variant="ghost"
          onclick={() => (mobileStep = "changes")}
        >
          <IconArrowLeft aria-hidden="true" />
          返回变更列表
        </Button>
      </div>
    {/if}

    <div class="min-h-0 flex-1">
      {#if !activeChange}
        <p class="p-4 text-xs text-muted-foreground">
          选择左侧任意变更项查看差异或逐块暂存。
        </p>
      {:else}
        <GitDiffViewer
          path={activeChange.path}
          oldPath={activeChange.oldPath}
          status={activeChange.status}
          binary={activeChange.binary}
          degradeReason={activeChange.degradeReason}
          hunks={activeChange.hunks}
          stageMode={readonlyMode
            ? "none"
            : activeChange.group === "staged"
              ? "unstage"
              : activeChange.group === "unstaged"
                ? "stage"
                : "none"}
          hunkStageAvailable={activeChange.hunkStageAvailable}
          preferredViewMode={diffMode}
          scrollTop={diffScrollTop}
          onViewModeChange={onDiffModeChange}
          onScrollChange={onDiffScrollChange}
          onStageHunk={(hunkId) => onStageHunk(activeChange.id, hunkId)}
          onUnstageHunk={(hunkId) => onUnstageHunk(activeChange.id, hunkId)}
        />
      {/if}
    </div>
  </section>
{/snippet}

{#if desktop.current}
  {#key paneResetKey}
    <Resizable.PaneGroup
      direction="horizontal"
      class="h-full min-h-0 min-w-0"
      onLayoutChange={(sizes) => {
        if (sizes.length === 2) {
          onPaneSizesChange?.([sizes[0]!, sizes[1]!]);
        }
      }}
    >
      <Resizable.Pane defaultSize={paneSizes[0]} minSize={26} class="git-pane">
        {@render changesAndComposerPane()}
      </Resizable.Pane>
      <Resizable.Handle withHandle class="git-divider" />
      <Resizable.Pane defaultSize={paneSizes[1]} minSize={34} class="git-pane">
        {@render diffPane()}
      </Resizable.Pane>
    </Resizable.PaneGroup>
  {/key}
{:else if mobileStep === "changes"}
  {@render changesAndComposerPane()}
{:else}
  {@render diffPane()}
{/if}
