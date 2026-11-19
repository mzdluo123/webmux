<script lang="ts">
  import { onDestroy } from "svelte";
  import IconCopy from "~icons/tabler/copy";
  import { m } from "$lib/paraglide/messages.js";
  import { locale } from "$lib/locale";
  import { Button } from "$lib/components/ui/button";
  import { Badge } from "$lib/components/ui/badge";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import {
    branchActionLabels,
    branchActions,
    branchOperation,
    divergedText,
    type BranchOperation,
  } from "../../domains/workspace/branch-actions";
  import { copyText } from "../../platform/clipboard";
  import { formatBytes } from "../../domains/code/code-image";
  import { errorMessage } from "../../platform/error";
  import { LatestRequest } from "../../hooks/shared/remote-request.svelte";
  import { measureWorktree } from "../../domains/workspace/worktree-usage";
  import type {
    SessionInfo,
    Worktree,
    WorktreeUsage,
  } from "../../contracts/schemas";
  import { countSessions } from "../../domains/sessions/session-status";
  import WorktreeSyncSummary from "./WorktreeSyncSummary.svelte";

  // What a group of the home page stands for: its path, the worktree's Git
  // state and sync, the session totals and, for a worktree, a disk size that
  // is only walked when asked for. The one details view of a worktree, held
  // by a dialog (home page, sidebar) or the repository panel.
  let {
    path,
    worktree,
    sessions,
    lastFetchedAt,
    onoperation,
  }: {
    path?: string;
    worktree?: Worktree;
    sessions: readonly SessionInfo[];
    /** The repository's last fetch, which ahead/behind are based on. */
    lastFetchedAt?: string | null;
    /** Starts a sync action of the worktree's branch; none without it. */
    onoperation?: (operation: BranchOperation) => void;
  } = $props();
  const counts = $derived(countSessions(sessions));
  const kindLabel = $derived(
    worktree?.kind === "main"
      ? m.s8_kind_main_worktree()
      : worktree?.kind === "bare"
        ? m.s8_kind_bare_repo()
        : m.s8_kind_linked_worktree(),
  );
  const offered = $derived(worktree ? branchActions(worktree) : null);

  let usage = $state<WorktreeUsage | null>(null);
  let measuring = $state(false);
  let usageError = $state("");
  const hasUsageOutput = $derived(
    measuring || Boolean(usage) || Boolean(usageError),
  );
  const requests = new LatestRequest();
  onDestroy(() => requests.dispose());
  async function measure(target: Worktree) {
    if (measuring || !target.available) return;
    const request = requests.begin();
    measuring = true;
    usageError = "";
    try {
      const result = await measureWorktree(target, request.signal);
      if (request.current()) usage = result;
    } catch (err) {
      if (request.reportable()) usageError = errorMessage(err);
    } finally {
      if (request.current()) measuring = false;
    }
  }
</script>

<div class="space-y-4">
  <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-2 text-xs">
    {#if path}
      <dt class="text-muted-foreground">{m.s8_label_path()}</dt>
      <dd class="flex min-w-0 items-start gap-1">
        <span class="min-w-0 flex-1 font-mono break-all">{path}</span>
        <Button
          size="icon-xs"
          variant="ghost"
          aria-label={m.s8_copy_path()}
          title={m.s8_copy_path()}
          onclick={() => void copyText(path)}
          ><IconCopy aria-hidden="true" /></Button
        >
      </dd>
    {/if}
    {#if worktree}
      <dt class="text-muted-foreground">{m.s8_label_type()}</dt>
      <dd class="flex flex-wrap gap-1.5">
        <Badge variant="secondary">{kindLabel}</Badge>
        {#if worktree.available && worktree.dirty === false}<Badge
            variant="secondary">{m.s8_tracked_files_clean()}</Badge
          >{/if}
      </dd>
    {/if}
    <dt class="text-muted-foreground">{m.s8_label_sessions()}</dt>
    <dd class="flex flex-wrap gap-x-3 gap-y-1">
      <span
        >{m.s8_details_total_sessions({ count: String(sessions.length) })}</span
      >
      <span
        >{m.s8_details_running()}
        <span class="inline-block w-[3ch] text-right tabular-nums"
          >{counts.active}</span
        ></span
      >
      <span
        >{m.s8_details_waiting()}
        <span class="inline-block w-[3ch] text-right tabular-nums"
          >{counts.waiting}</span
        ></span
      >
      <span
        >{m.s8_details_exited()}
        <span class="inline-block w-[3ch] text-right tabular-nums"
          >{counts.exited}</span
        ></span
      >
    </dd>
    {#if worktree}
      <dt class="text-muted-foreground">{m.s8_label_disk_usage()}</dt>
      <dd class="grid min-w-0 justify-items-start gap-1">
        <Button
          size="xs"
          variant="outline"
          disabled={!worktree.available}
          aria-disabled={measuring || undefined}
          tabindex={measuring ? -1 : undefined}
          onclick={() => {
            if (measuring || !worktree.available) return;
            void measure(worktree);
          }}>{usage ? m.s8_recalculate() : m.s8_calculate_usage()}</Button
        >
        {#if worktree.available && hasUsageOutput}
          <ScrollArea
            class="max-h-16 w-full"
            viewportClasses="overscroll-contain wrap-anywhere"
            viewportProps={{
              role: "region",
              tabindex: usageError ? 0 : undefined,
              "aria-label": m.s8_label_disk_usage(),
            }}
          >
            {#if usageError}
              <div role="alert" class="text-destructive">{usageError}</div>
            {:else}
              <div role="status" class="tabular-nums">
                {#if measuring}
                  {m.s8_calculating()}
                {:else if usage}
                  {@const entries = usage.entries.toLocaleString(
                    locale.current,
                  )}
                  {usage.complete
                    ? m.s8_details_usage({
                        bytes: formatBytes(usage.bytes),
                        entries,
                      })
                    : m.s8_details_usage_incomplete({
                        bytes: formatBytes(usage.bytes),
                        entries,
                      })}
                {/if}
              </div>
            {/if}
          </ScrollArea>
        {/if}
      </dd>
    {/if}
  </dl>
  {#if worktree && !worktree.available}
    <p class="text-xs text-muted-foreground">
      {m.s8_details_cannot_measure_missing()}
    </p>
  {/if}
  {#if worktree}<WorktreeSyncSummary
      {worktree}
      variant="full"
      {lastFetchedAt}
      actions={onoperation && offered?.branch ? syncActions : undefined}
    />{/if}
</div>

{#snippet syncActions()}
  {#if offered?.branch && onoperation}
    {@const branch = offered.branch}
    {#if offered.diverged}
      <p class="text-xs text-muted-foreground">{divergedText()}</p>
    {/if}
    {#if offered.actions.length}
      <div class="flex flex-wrap gap-2">
        {#each offered.actions as action (action)}
          <Button
            size="sm"
            variant="outline"
            onclick={() => onoperation(branchOperation(action, branch))}
            >{branchActionLabels[action]}</Button
          >
        {/each}
      </div>
    {/if}
  {/if}
{/snippet}
