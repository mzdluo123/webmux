<script lang="ts">
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import { resolveCommitParentFiles } from "../../domains/workspace/git-workbench-state";
  import type { GitCommitDetail } from "../../contracts/git-workbench-schemas";
  import GitDiffViewer from "./GitDiffViewer.svelte";

  let {
    detail,
    selectedParentOverride,
    selectedCommitFilePath,
    desktop = true,
    diffMode,
    diffScrollTop = 0,
    onDiffModeChange,
    onDiffScrollChange,
    onBackToList,
    onSelectParent,
    onSelectCommitFile,
    onOpenMobileDiff,
  }: {
    detail: GitCommitDetail | null;
    selectedParentOverride?: string;
    selectedCommitFilePath: string | null;
    desktop?: boolean;
    diffMode?: "unified" | "split";
    diffScrollTop?: number;
    onDiffModeChange?: (mode: "unified" | "split") => void;
    onDiffScrollChange?: (top: number) => void;
    onBackToList?: () => void;
    onSelectParent: (commitOid: string, parentOid: string) => void;
    onSelectCommitFile: (path: string) => void;
    onOpenMobileDiff?: () => void;
  } = $props();

  let showDiffPreview = $state(false);

  const resolvedParent = $derived(
    detail
      ? resolveCommitParentFiles(detail, selectedParentOverride)
      : { selectedParent: "", files: [] },
  );
  const activeFile = $derived(
    resolvedParent.files.find((f) => f.newPath === selectedCommitFilePath) ??
      resolvedParent.files[0] ??
      null,
  );
</script>

<section
  class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface"
  aria-label="提交详情与变更文件"
>
  {#if !detail}
    <p class="p-4 text-xs text-muted-foreground">
      选择一条提交记录以查看变更文件与元数据。
    </p>
  {:else}
    <div class="space-y-2 border-b border-git-border bg-git-panel p-3">
      {#if !desktop && onBackToList}
        <Button size="xs" variant="ghost" onclick={onBackToList}>
          <IconArrowLeft aria-hidden="true" />
          返回提交列表
        </Button>
      {/if}

      <div class="space-y-1">
        <h3 class="text-sm font-semibold wrap-anywhere text-foreground">
          {detail.subject}
        </h3>
        {#if detail.body}
          <p
            class="text-xs whitespace-pre-wrap wrap-anywhere text-muted-foreground"
          >
            {detail.body}
          </p>
        {/if}
      </div>

      <div class="grid min-w-0 grid-cols-1 gap-1 text-xs text-muted-foreground">
        <p class="wrap-anywhere">
          作者：{detail.authorName} &lt;{detail.authorEmail}&gt; · {detail.authorAt}
        </p>
        {#if detail.distinctCommitter}
          <p class="wrap-anywhere">
            提交者：{detail.committerName} &lt;{detail.committerEmail}&gt; · {detail.committerAt}
          </p>
        {/if}
        <p class="font-mono break-all">OID：{detail.oid}</p>
      </div>

      {#if detail.parents.length > 1}
        <div class="space-y-1 pt-1">
          <span class="text-xs font-medium text-foreground">对比父提交：</span>
          <SegmentedControl.Root
            wrap
            value={resolvedParent.selectedParent}
            onValueChange={(val) => {
              if (val) onSelectParent(detail.oid, val);
            }}
            aria-label="合并提交父节点切换"
          >
            {#each detail.parents as parent, pIdx (parent.oid)}
              <SegmentedControl.Item value={parent.oid}>
                父节点 #{pIdx + 1} ({parent.shortOid})
              </SegmentedControl.Item>
            {/each}
          </SegmentedControl.Root>
        </div>
      {:else if detail.parents.length === 0}
        <Badge variant="outline">根提交（对比空树）</Badge>
      {/if}
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto p-1.5">
      <div class="mb-1 flex items-center justify-between px-1.5">
        <span class="text-xs font-medium text-muted-foreground">
          变更文件 ({resolvedParent.files.length})
        </span>
        {#if desktop && activeFile}
          <Button
            size="xs"
            variant="ghost"
            aria-pressed={showDiffPreview}
            onclick={() => (showDiffPreview = !showDiffPreview)}
          >
            {showDiffPreview ? "收起差异预览" : "展开差异预览"}
          </Button>
        {/if}
      </div>
      {#each resolvedParent.files as file (file.newPath)}
        <ListButton
          variant="git-row"
          aria-selected={activeFile?.newPath === file.newPath}
          onclick={() => {
            onSelectCommitFile(file.newPath);
            if (desktop) showDiffPreview = true;
            else onOpenMobileDiff?.();
          }}
        >
          <Badge variant="outline">{file.status}</Badge>
          <span class="min-w-0 flex-1 truncate font-mono" title={file.newPath}>
            {file.oldPath && file.oldPath !== file.newPath
              ? `${file.oldPath} → ${file.newPath}`
              : file.newPath}
          </span>
          <span class="shrink-0 font-mono text-xs text-success">
            +{file.additions}
          </span>
          <span class="shrink-0 font-mono text-xs text-destructive">
            -{file.deletions}
          </span>
        </ListButton>
      {/each}
    </div>

    {#if desktop && showDiffPreview && activeFile}
      <div class="h-64 min-h-0 shrink-0 border-t border-git-border">
        <GitDiffViewer
          file={activeFile}
          preferredViewMode={diffMode}
          scrollTop={diffScrollTop}
          onViewModeChange={onDiffModeChange}
          onScrollChange={onDiffScrollChange}
        />
      </div>
    {/if}
  {/if}
</section>
