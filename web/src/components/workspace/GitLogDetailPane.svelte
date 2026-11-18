<script lang="ts">
  import { tick, untrack } from "svelte";
  import IconCheck from "~icons/tabler/check";
  import IconCopy from "~icons/tabler/copy";
  import IconGitCherryPick from "~icons/tabler/git-cherry-pick";
  import IconGitCommit from "~icons/tabler/git-commit";
  import IconRotate2 from "~icons/tabler/rotate-2";
  import IconTag from "~icons/tabler/tag";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";
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
  let detailViewport = $state<HTMLElement | null>(null);
  const comparing = $derived(Boolean(compareMode));
  let previousMode: boolean | undefined;
  let commitReadingPosition: { oid: string; top: number } | undefined;

  $effect.pre(() => {
    const viewport = detailViewport;
    const next = comparing;
    if (!viewport || next === previousMode) return;
    const changed = previousMode !== undefined;
    previousMode = next;

    const oid = untrack(() => detail?.oid);
    if (next && oid) commitReadingPosition = { oid, top: viewport.scrollTop };
    if (!changed) return;

    const top =
      !next && commitReadingPosition?.oid === oid
        ? (commitReadingPosition?.top ?? 0)
        : 0;
    if (!next) commitReadingPosition = undefined;
    void tick().then(() => {
      if (detailViewport === viewport && comparing === next)
        viewport.scrollTop = top;
    });
  });

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
  aria-label={m.s6_log_detail_aria()}
>
  <div
    class="flex shrink-0 items-center justify-between gap-2 border-b bg-background px-2.5 py-1.5"
  >
    <span
      class="flex min-h-(--control-sm) items-center text-xs font-semibold text-foreground"
    >
      {compareMode
        ? m.s6_log_detail_compare_heading()
        : m.s6_log_detail_commit_heading()}
    </span>
    {#if detail && !compareMode}
      <div class="flex items-center gap-1">
        <span class="font-mono text-xs text-muted-foreground">
          {detail.shortOid}
        </span>
        <Button
          size="icon-sm"
          variant="ghost"
          aria-label={m.s6_log_detail_copy_full_oid_aria()}
          title={m.s6_log_detail_copy_full_oid_tooltip({ oid: detail.oid })}
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

  <ScrollArea bind:viewportRef={detailViewport} class="min-h-8 flex-1">
    <div class="space-y-2.5 p-3 text-xs">
      {#if compareMode}
        <div class="space-y-2">
          <div class="rounded-md border bg-background p-2.5">
            <div class="flex items-center justify-between gap-2">
              <Badge variant="outline"
                >{m.s6_log_detail_compare_base_badge()}</Badge
              >
              <Button
                size="icon-sm"
                variant="ghost"
                aria-label={m.s6_log_detail_copy_base_oid_aria()}
                title={m.s6_log_detail_copy_base_oid_tooltip()}
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
              <Badge variant="info"
                >{m.s6_log_detail_compare_target_badge()}</Badge
              >
              <Button
                size="icon-sm"
                variant="ghost"
                aria-label={m.s6_log_detail_copy_target_oid_aria()}
                title={m.s6_log_detail_copy_target_oid_tooltip()}
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
            {m.s6_log_detail_compare_readonly_note()}
          </p>
        </div>
      {:else if loading && !detail}
        <p class="text-muted-foreground" role="status">
          {m.s6_log_detail_loading()}
        </p>
      {:else if !detail}
        <p class="text-muted-foreground">
          {m.s6_log_detail_empty()}
        </p>
      {:else}
        {#if detail.parents.length > 1}
          <div class="space-y-1 rounded-md border bg-background p-2">
            <span class="font-medium text-foreground"
              >{m.s6_log_detail_merge_parent_label()}</span
            >
            <SegmentedControl
              wrap
              value={resolvedParent.selectedParent}
              onValueChange={(val) => {
                if (val) onSelectParent(detail.oid, val);
              }}
              aria-label={m.s6_log_detail_merge_parent_aria()}
            >
              {#each detail.parents as parent, pIdx (parent.oid)}
                <SegmentedControlItem value={parent.oid}>
                  {m.s6_log_detail_parent_item({
                    index: String(pIdx + 1),
                    shortOid: parent.shortOid,
                  })}
                </SegmentedControlItem>
              {/each}
            </SegmentedControl>
          </div>
        {:else if detail.parents.length === 0}
          <div>
            <Badge variant="outline"
              >{m.s6_log_detail_root_commit_badge()}</Badge
            >
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
                  {m.s6_log_detail_cherry_pick_btn()}
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
                  {m.s6_log_detail_revert_btn()}
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
                  {m.s6_create_tag_ellipsis()}
                </Button>
              {/if}
            </div>
          {/if}
        </div>

        <div
          class="space-y-1 rounded-md border bg-background p-2.5 text-muted-foreground"
        >
          <p class="wrap-anywhere">
            <span class="font-medium text-foreground"
              >{m.s6_log_detail_author_label()}</span
            >{detail.authorName}
            &lt;{detail.authorEmail}&gt; · {detail.authorAt}
          </p>
          {#if detail.distinctCommitter}
            <p class="wrap-anywhere">
              <span class="font-medium text-foreground"
                >{m.s6_log_detail_committer_label()}</span
              >{detail.committerName}
              &lt;{detail.committerEmail}&gt; · {detail.committerAt}
            </p>
          {/if}
          <p class="font-mono break-all">
            <span class="font-sans font-medium text-foreground"
              >{m.s6_log_detail_oid_label()}</span
            >{detail.oid}
          </p>

          {#if detail.parents.length > 0}
            <div class="space-y-1 pt-1">
              <span class="font-medium text-foreground">
                {m.s6_log_detail_parents_label({
                  count: String(detail.parents.length),
                })}
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
  </ScrollArea>
</section>
