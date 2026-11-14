<script lang="ts">
  import IconArrowBackUp from "~icons/tabler/arrow-back-up";
  import IconGitCommit from "~icons/tabler/git-commit";
  import IconHistory from "~icons/tabler/history";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import type { GitBlameCommit, GitBlameResult } from "../../contracts/schemas";
  import type { GitBlameCrumb } from "../../hooks/workspace/git-blame-controller.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    data,
    loading = false,
    error = "",
    trail = [],
    onStepBack,
    onReblameParent,
    onInspectCommitInHistory,
    onJumpToCommit,
  }: {
    data: GitBlameResult | null;
    loading?: boolean;
    error?: string;
    trail?: readonly GitBlameCrumb[];
    onStepBack?: (index: number) => void;
    onReblameParent?: (previousOid: string, previousPath: string) => void;
    onInspectCommitInHistory?: (commitOid: string) => void;
    onJumpToCommit?: (commitOid: string) => void;
  } = $props();

  let activeCommitOid = $state<string | null>(null);

  const commitByOid = $derived.by(() => {
    const out: Record<string, GitBlameCommit> = {};
    for (const c of data?.commits ?? []) {
      out[c.oid] = c;
    }
    return out;
  });

  const selectedCommit = $derived.by(() => {
    if (!data) return null;
    if (activeCommitOid && commitByOid[activeCommitOid]) {
      return commitByOid[activeCommitOid] ?? null;
    }
    return data.commits[0] ?? null;
  });
</script>

<div
  class="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-background"
  data-testid="git-blame-view"
>
  {#if trail.length > 0 && onStepBack}
    <div
      class="flex flex-wrap items-center gap-1.5 border-b bg-background px-3 py-1.5 text-xs"
      data-testid="git-blame-breadcrumb"
    >
      <span class="text-muted-foreground">溯源轨迹：</span>
      {#each trail as crumb, idx (`${idx}:${crumb.rev}:${crumb.path}`)}
        <Button size="sm" variant="ghost" onclick={() => onStepBack(idx)}>
          <span class="font-mono">{crumb.label}</span>
        </Button>
        <span class="text-muted-foreground">/</span>
      {/each}
      {#if data}
        <Badge variant="info">
          <span class="font-mono">{data.shortOid} · {data.path}</span>
        </Badge>
      {/if}
    </div>
  {/if}

  {#if selectedCommit && data?.kind === "text"}
    <div
      class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-b bg-background px-3 py-2 text-xs"
      data-testid="git-blame-selected-commit-bar"
    >
      <div class="min-w-0 flex-1 basis-48 space-y-0.5">
        <div class="flex flex-wrap items-center gap-1.5">
          <Badge variant="info">
            <span class="font-mono">{selectedCommit.shortOid}</span>
          </Badge>
          <span class="font-medium text-foreground">
            {selectedCommit.authorName}
          </span>
          <span class="truncate text-muted-foreground">
            {selectedCommit.authorAt}
          </span>
        </div>
        <p class="truncate text-foreground">{selectedCommit.summary}</p>
      </div>

      <div class="flex flex-wrap items-center gap-1.5">
        {#if selectedCommit.previousOid && onReblameParent}
          <Button
            size="sm"
            variant="outline"
            onclick={() =>
              onReblameParent(
                selectedCommit.previousOid!,
                selectedCommit.previousPath || data.path,
              )}
            data-testid="git-blame-parent-btn"
          >
            <IconArrowBackUp aria-hidden="true" />
            在父版本继续 Blame
          </Button>
        {/if}
        {#if onInspectCommitInHistory}
          <Button
            size="sm"
            variant="outline"
            onclick={() => onInspectCommitInHistory(selectedCommit.oid)}
          >
            <IconHistory aria-hidden="true" />
            查看该提交改动
          </Button>
        {/if}
        {#if onJumpToCommit}
          <Button
            size="sm"
            variant="outline"
            onclick={() => onJumpToCommit(selectedCommit.oid)}
            data-testid="git-blame-jump-log-btn"
          >
            <IconGitCommit aria-hidden="true" />
            在历史图中定位
          </Button>
        {/if}
      </div>
    </div>
  {/if}

  <ScrollArea orientation="both" class="min-h-8 flex-1">
    {#if loading && !data}
      <p class="p-4 text-xs text-muted-foreground" role="status">
        正在按行溯源计算只读 Blame 注解…
      </p>
    {:else if error}
      <div class="p-3">
        <InlineNotice tone="danger">{error}</InlineNotice>
      </div>
    {:else if !data}
      <p class="p-4 text-xs text-muted-foreground">尚未加载 Blame 数据。</p>
    {:else if data.kind !== "text"}
      <div class="p-3">
        <InlineNotice tone="info">
          {data.degradeReason ||
            "该文件为二进制或体积过大，不直接展开逐行文本 Blame。"}
        </InlineNotice>
      </div>
    {:else}
      <div class="w-max min-w-full divide-y divide-border/40 font-mono text-xs">
        {#each data.lines as line, idx (line.lineNumber)}
          {@const prevLine = idx > 0 ? data.lines[idx - 1] : null}
          {@const groupStart =
            !prevLine || prevLine.commitOid !== line.commitOid}
          {@const meta = commitByOid[line.commitOid]}
          {@const isSelected = selectedCommit?.oid === line.commitOid}
          <div
            class={isSelected
              ? "flex min-w-0 items-stretch bg-accent/35"
              : "flex min-w-0 items-stretch hover:bg-muted/40"}
            data-blame-line={line.lineNumber}
          >
            <ListButton
              variant="provenance-gutter"
              title={meta
                ? `${meta.shortOid} · ${meta.authorName} · ${meta.summary}`
                : line.commitOid}
              onclick={() => (activeCommitOid = line.commitOid)}
              data-blame-gutter={line.shortOid}
            >
              {#if groupStart}
                <span class="shrink-0 font-semibold text-foreground">
                  {line.shortOid}
                </span>
                <span class="min-w-0 flex-1 truncate max-sm:hidden">
                  {meta?.authorName ?? ""}
                </span>
              {/if}
            </ListButton>
            <span
              class="w-10 shrink-0 select-none border-r px-1.5 py-0.5 text-right text-muted-foreground"
            >
              {line.lineNumber}
            </span>
            <pre
              class="min-w-0 flex-1 px-2.5 py-0.5 whitespace-pre text-foreground">{line.text}</pre>
          </div>
        {/each}
      </div>
    {/if}
  </ScrollArea>
</div>
