<script lang="ts">
  import IconCheck from "~icons/tabler/check";
  import IconCopy from "~icons/tabler/copy";
  import IconGitCherryPick from "~icons/tabler/git-cherry-pick";
  import IconGitCommit from "~icons/tabler/git-commit";
  import IconRotate2 from "~icons/tabler/rotate-2";
  import IconTag from "~icons/tabler/tag";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import { resolveCommitParentFiles } from "../../domains/workspace/git-workbench-state";
  import type {
    GitCommitDetail,
    GitCompareViewState,
    GitHistoryRow,
  } from "../../contracts/git-workbench-schemas";

  let {
    detail,
    selectedParentOverride,
    compareMode = null,
    commits = [],
    loading = false,
    onSelectParent,
    onJumpToCommit,
    onCherryPickCommit,
    onRevertCommit,
    onCreateTagCommit,
  }: {
    detail: GitCommitDetail | null;
    selectedParentOverride?: string;
    compareMode?: GitCompareViewState | null;
    commits?: readonly GitHistoryRow[];
    loading?: boolean;
    onSelectParent: (commitOid: string, parentOid: string) => void;
    onJumpToCommit?: (oid: string) => void;
    onCherryPickCommit?: (oid: string) => void;
    onRevertCommit?: (oid: string) => void;
    onCreateTagCommit?: (oid: string, subject: string) => void;
  } = $props();

  let copiedOid = $state<string | null>(null);

  const resolvedParent = $derived(
    detail
      ? resolveCommitParentFiles(detail, selectedParentOverride)
      : { selectedParent: "", files: [] },
  );

  const compareBaseRow = $derived(
    compareMode
      ? commits.find((item) => item.oid === compareMode.baseOid)
      : undefined,
  );
  const compareTargetRow = $derived(
    compareMode
      ? commits.find((item) => item.oid === compareMode.targetOid)
      : undefined,
  );

  function copyOid(oid: string) {
    if (typeof navigator !== "undefined" && navigator.clipboard) {
      void navigator.clipboard.writeText(oid);
    }
    copiedOid = oid;
    setTimeout(() => {
      if (copiedOid === oid) copiedOid = null;
    }, 1500);
  }
</script>

<section
  class="flex h-full min-h-0 min-w-0 flex-col bg-background"
  aria-label="提交详情元数据"
>
  <div
    class="flex items-center justify-between gap-2 border-b bg-background px-2.5 py-1.5"
  >
    <span class="text-xs font-semibold text-foreground">
      {compareMode ? "两版本比较详情" : "提交详情"}
    </span>
    {#if detail && !compareMode}
      <div class="flex items-center gap-1">
        <span class="font-mono text-xs text-muted-foreground">
          {detail.shortOid}
        </span>
        <Button
          size="icon-sm"
          variant="ghost"
          aria-label="复制完整提交 OID"
          title={`复制完整 OID：${detail.oid}`}
          onclick={() => copyOid(detail.oid)}
        >
          {#if copiedOid === detail.oid}
            <IconCheck class="text-success" aria-hidden="true" />
          {:else}
            <IconCopy aria-hidden="true" />
          {/if}
        </Button>
      </div>
    {/if}
  </div>

  <div class="min-h-0 flex-1 space-y-2.5 overflow-y-auto p-3 text-xs">
    {#if compareMode}
      <div class="space-y-2">
        <div class="rounded-md border bg-background p-2.5">
          <div class="flex items-center justify-between gap-2">
            <Badge variant="outline">比较起点 (Base)</Badge>
            <Button
              size="icon-sm"
              variant="ghost"
              aria-label="复制起点 OID"
              title="复制起点完整 OID"
              onclick={() => copyOid(compareMode.baseOid)}
            >
              {#if copiedOid === compareMode.baseOid}
                <IconCheck class="text-success" aria-hidden="true" />
              {:else}
                <IconCopy aria-hidden="true" />
              {/if}
            </Button>
          </div>
          <p class="mt-1 font-mono break-all text-foreground">
            {compareMode.baseOid}
          </p>
          {#if compareBaseRow}
            <p class="mt-1 wrap-anywhere text-muted-foreground">
              {compareBaseRow.subject}
            </p>
          {/if}
        </div>

        <div class="rounded-md border bg-background p-2.5">
          <div class="flex items-center justify-between gap-2">
            <Badge variant="info">比较终点 (Target)</Badge>
            <Button
              size="icon-sm"
              variant="ghost"
              aria-label="复制终点 OID"
              title="复制终点完整 OID"
              onclick={() => copyOid(compareMode.targetOid)}
            >
              {#if copiedOid === compareMode.targetOid}
                <IconCheck class="text-success" aria-hidden="true" />
              {:else}
                <IconCopy aria-hidden="true" />
              {/if}
            </Button>
          </div>
          <p class="mt-1 font-mono break-all text-foreground">
            {compareMode.targetOid}
          </p>
          {#if compareTargetRow}
            <p class="mt-1 wrap-anywhere text-muted-foreground">
              {compareTargetRow.subject}
            </p>
          {/if}
        </div>

        <p class="leading-relaxed text-muted-foreground">
          只读对比：直接按完整提交 OID
          读取冻结历史对象树，不执行检出，也不修改工作树或暂存索引。
        </p>
      </div>
    {:else if loading && !detail}
      <p class="text-muted-foreground" role="status">正在加载提交详情…</p>
    {:else if !detail}
      <p class="text-muted-foreground">
        选择一条提交记录以查看完整说明、作者、父节点与引用信息。
      </p>
    {:else}
      {#if detail.parents.length > 1}
        <div class="space-y-1 rounded-md border bg-background p-2">
          <span class="font-medium text-foreground">合并提交对比父节点：</span>
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
        <div>
          <Badge variant="outline">根提交（对比空树）</Badge>
        </div>
      {/if}

      {#if detail.refs.length > 0}
        <div class="flex flex-wrap items-center gap-1.5">
          {#each detail.refs as refBadge (`${refBadge.kind}:${refBadge.name}`)}
            <Badge
              variant={refBadge.kind === "head"
                ? "info"
                : refBadge.kind === "remote"
                  ? "secondary"
                  : "outline"}
            >
              {#if refBadge.kind === "tag"}
                <IconTag class="size-3 shrink-0" aria-hidden="true" />
              {/if}
              <span>{refBadge.name}</span>
            </Badge>
          {/each}
        </div>
      {/if}

      <div class="space-y-1.5">
        <h3 class="text-sm font-semibold wrap-anywhere text-foreground">
          {detail.subject}
        </h3>
        {#if detail.body}
          <p
            class="leading-relaxed whitespace-pre-wrap wrap-anywhere text-foreground/90"
          >
            {detail.body}
          </p>
        {/if}
        {#if onCherryPickCommit || onRevertCommit || onCreateTagCommit}
          <div class="flex flex-wrap items-center gap-1.5 pt-1">
            {#if onCherryPickCommit}
              <Button
                size="sm"
                variant="outline"
                wrap
                onclick={() => onCherryPickCommit(detail.oid)}
                data-testid="git-log-detail-cherry-pick-btn"
              >
                <IconGitCherryPick aria-hidden="true" />
                遴选此提交… (Cherry-pick)
              </Button>
            {/if}
            {#if onRevertCommit}
              <Button
                size="sm"
                variant="outline"
                wrap
                onclick={() => onRevertCommit(detail.oid)}
                data-testid="git-log-detail-revert-btn"
              >
                <IconRotate2 aria-hidden="true" />
                还原此提交… (Revert)
              </Button>
            {/if}
            {#if onCreateTagCommit}
              <Button
                size="sm"
                variant="outline"
                wrap
                onclick={() => onCreateTagCommit(detail.oid, detail.subject)}
                data-testid="git-log-detail-create-tag-btn"
              >
                <IconTag aria-hidden="true" />
                新建标签…
              </Button>
            {/if}
          </div>
        {/if}
      </div>

      <div
        class="space-y-1 rounded-md border bg-background p-2.5 text-muted-foreground"
      >
        <p class="wrap-anywhere">
          <span class="font-medium text-foreground">作者：</span
          >{detail.authorName}
          &lt;{detail.authorEmail}&gt; · {detail.authorAt}
        </p>
        {#if detail.distinctCommitter}
          <p class="wrap-anywhere">
            <span class="font-medium text-foreground">提交者：</span
            >{detail.committerName}
            &lt;{detail.committerEmail}&gt; · {detail.committerAt}
          </p>
        {/if}
        <p class="font-mono break-all">
          <span class="font-sans font-medium text-foreground">OID：</span
          >{detail.oid}
        </p>

        {#if detail.parents.length > 0}
          <div class="space-y-1 pt-1">
            <span class="font-medium text-foreground">
              父提交 ({detail.parents.length})：
            </span>
            <div class="flex flex-wrap gap-1.5">
              {#each detail.parents as parent (parent.oid)}
                {#if onJumpToCommit}
                  <Button
                    size="sm"
                    variant="outline"
                    class="max-w-full"
                    title={parent.subject
                      ? `${parent.oid} · ${parent.subject}`
                      : parent.oid}
                    onclick={() => onJumpToCommit(parent.oid)}
                  >
                    <IconGitCommit aria-hidden="true" />
                    <span class="font-mono">{parent.shortOid}</span>
                    {#if parent.subject}
                      <span class="max-w-36 truncate text-muted-foreground">
                        {parent.subject}
                      </span>
                    {/if}
                  </Button>
                {:else}
                  <Badge variant="outline">
                    <span class="font-mono">{parent.shortOid}</span>
                  </Badge>
                {/if}
              {/each}
            </div>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</section>
