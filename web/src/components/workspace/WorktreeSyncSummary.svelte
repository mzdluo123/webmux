<script lang="ts">
  import type { Snippet } from "svelte";
  import IconCopy from "~icons/tabler/copy";
  import { m } from "$lib/paraglide/messages.js";
  import { locale } from "$lib/locale";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { copyText } from "../../platform/clipboard";
  import { relativeTime } from "../../domains/workspace/commit-choices";
  import type { Worktree } from "../../contracts/schemas";
  import {
    changeCounts,
    fetchedText,
    operationLabels,
    worktreeUpstream,
  } from "../../domains/workspace/worktree-sync";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import UpstreamStatus from "./UpstreamStatus.svelte";

  // A worktree's Git sync: its upstream and the commits ahead or behind it,
  // the change counts, the last commit and an operation in progress.
  // `indicator` is the home title's: only what needs attention, laid out by
  // the parent's flex row. `compact` is one wrapping line of a list row;
  // `full` the details' three sections.
  let {
    worktree,
    variant = "compact",
    lastFetchedAt,
    actions,
  }: {
    worktree: Worktree;
    variant?: "indicator" | "compact" | "full";
    /** The repository's last fetch; shown beside the upstream in `full`. */
    lastFetchedAt?: string | null;
    /** The branch's sync actions, at the end of `full`'s branch section. */
    actions?: Snippet;
  } = $props();
  const upstream = $derived(worktreeUpstream(worktree));
  const operation = $derived(
    worktree.operation ? operationLabels[worktree.operation] : "",
  );
  const commit = $derived(worktree.lastCommit);
  // A readable worktree without commits says so instead of "未知".
  const empty = $derived(
    worktree.available && worktree.changes !== null && worktree.head === null,
  );
  const changed = $derived(
    worktree.changes
      ? changeCounts(worktree.changes).filter((item) => item.count > 0)
      : [],
  );
</script>

{#snippet operationBadge()}
  {#if operation}<Badge variant="warning">{operation}</Badge>{/if}
{/snippet}

{#if variant === "indicator"}
  {@render operationBadge()}
  {#if upstream}<UpstreamStatus state={upstream} indicator />{/if}
{:else if worktree.kind === "bare"}
  <!-- A bare repository has no working tree to sync. -->
{:else if variant === "compact"}
  <div
    class="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground"
  >
    {#if operation || upstream}
      <span class="flex min-w-0 flex-wrap items-center gap-1.5">
        {@render operationBadge()}
        {#if upstream}<UpstreamStatus state={upstream} />{/if}
      </span>
    {/if}
    {#if changed.length}
      <span class="tabular-nums"
        >{changed
          .map((item) => `${item.label} ${item.count}`)
          .join(" · ")}</span
      >
    {/if}
    {#if commit}
      <span class="min-w-0 break-all" title={commit.hash}
        >{commit.subject || m.s8_untitled_commit()} · {relativeTime(
          commit.committedAt,
        )}</span
      >
    {:else if empty}<span>{m.s8_no_commits_yet()}</span>{/if}
  </div>
{:else}
  <div class="space-y-4">
    {#if operation}
      <InlineNotice tone="warning"
        >{m.s8_sync_op_in_progress_notice({ operation })}</InlineNotice
      >
    {/if}
    <section class="space-y-2" aria-label={m.s8_sync_branch_and_upstream()}>
      <h3 class="text-sm font-medium">{m.s8_sync_branch_and_upstream()}</h3>
      <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-2 text-xs">
        <dt class="text-muted-foreground">{m.s8_repo_tab_branches()}</dt>
        <dd class="font-mono break-all">
          {worktree.branch ?? m.s8_detached_head_plain()}{worktree.head
            ? ` · ${worktree.head.slice(0, 8)}`
            : ""}
        </dd>
        {#if upstream}
          <dt class="text-muted-foreground">{m.s8_label_upstream()}</dt>
          <dd class="flex min-w-0 flex-wrap items-center gap-1.5">
            <UpstreamStatus state={upstream} />
          </dd>
          {#if lastFetchedAt !== undefined && upstream.kind !== "none" && upstream.kind !== "unknown"}
            <dt class="text-muted-foreground">{m.s8_label_last_fetched()}</dt>
            <dd
              title={lastFetchedAt
                ? new Date(lastFetchedAt).toLocaleString(locale.current)
                : undefined}
            >
              {m.s8_sync_fetched_approx_note({
                text: fetchedText(lastFetchedAt),
              })}
            </dd>
          {/if}
        {/if}
      </dl>
      {@render actions?.()}
    </section>
    <section class="space-y-2" aria-label={m.s8_sync_recent_commit()}>
      <h3 class="text-sm font-medium">{m.s8_sync_recent_commit()}</h3>
      {#if commit}
        <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-2 text-xs">
          <dt class="text-muted-foreground">{m.s8_label_subject()}</dt>
          <dd class="break-all">{commit.subject || m.s8_untitled_commit()}</dd>
          <dt class="text-muted-foreground">{m.s8_label_commit()}</dt>
          <dd class="flex min-w-0 items-start gap-1">
            <span class="min-w-0 flex-1 font-mono break-all">{commit.hash}</span
            >
            <Button
              size="icon-xs"
              variant="ghost"
              aria-label={m.s8_copy_commit_hash()}
              title={m.s8_copy_commit_hash()}
              onclick={() => void copyText(commit.hash)}
              ><IconCopy aria-hidden="true" /></Button
            >
          </dd>
          <dt class="text-muted-foreground">{m.s8_label_author()}</dt>
          <dd class="break-all">{commit.author}</dd>
          <dt class="text-muted-foreground">{m.s8_label_time()}</dt>
          <dd
            title={new Date(commit.committedAt).toLocaleString(locale.current)}
          >
            {relativeTime(commit.committedAt)}
          </dd>
        </dl>
      {:else}
        <p class="text-xs text-muted-foreground">
          {empty ? m.s8_no_commits_yet() : m.s8_unknown()}
        </p>
      {/if}
    </section>
    <section class="space-y-2" aria-label={m.s8_sync_change_stats()}>
      <h3 class="text-sm font-medium">{m.s8_sync_change_stats()}</h3>
      {#if worktree.changes}
        <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-2 text-xs">
          {#each changeCounts(worktree.changes) as item (item.label)}
            <dt class="text-muted-foreground">{item.label}</dt>
            <dd class="tabular-nums">{item.count}</dd>
          {/each}
        </dl>
      {:else}
        <p class="text-xs text-muted-foreground">{m.s8_unknown()}</p>
      {/if}
    </section>
  </div>
{/if}
