<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { worktreeTitle } from "../../domains/workspace/home-worktrees";
  import type { RemovalPreviewItem } from "../../contracts/schemas";
  import type { BatchPreviewGroups } from "../../domains/workspace/worktree-batch";
  import { Badge } from "$lib/components/ui/badge";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import PushCheckNotice from "../settings/PushCheckNotice.svelte";
  import WorktreeSessionList from "./WorktreeSessionList.svelte";

  let {
    groups,
    includeForce = $bindable(false),
    busy = false,
  }: {
    groups: BatchPreviewGroups;
    includeForce: boolean;
    busy?: boolean;
  } = $props();

  function keepsRecords(item: RemovalPreviewItem, forced: boolean): boolean {
    if (item.commit === "") return false;
    if (item.archived.length > 0) return true;
    return forced && item.sessions.some((s) => s.agent !== null);
  }
</script>

<div class="space-y-4">
  <div class="rounded-lg border bg-muted/40 p-3 text-sm">
    <div class="flex flex-wrap items-center gap-2 font-medium">
      <span
        >{m.s8_batch_prev_safe_count({
          count: String(groups.safe.length),
        })}</span
      >
      {#if groups.blocked.length > 0}
        <span class="text-muted-foreground">·</span>
        <span
          >{m.s8_batch_prev_blocked_count({
            count: String(groups.blocked.length),
          })}</span
        >
      {/if}
      {#if groups.unsupported.length > 0}
        <span class="text-muted-foreground">·</span>
        <span
          >{m.s8_batch_prev_unsupported_count({
            count: String(groups.unsupported.length),
          })}</span
        >
      {/if}
    </div>
    <p class="mt-1.5 text-xs text-muted-foreground">
      {m.s8_batch_prev_summary_hint()}
    </p>
  </div>

  <div class="max-h-[48dvh] space-y-4 overflow-y-auto pr-1">
    {#if groups.safe.length > 0}
      <section class="space-y-2" aria-label={m.s8_batch_prev_safe_aria()}>
        <h3 class="text-sm font-medium">
          {m.s8_batch_prev_safe_heading({ count: String(groups.safe.length) })}
        </h3>
        <ul class="space-y-2">
          {#each groups.safe as item (item.worktree.id)}
            {@const retained = item.commit !== "" && item.archived.length > 0}
            <li class="space-y-1.5 rounded-lg border p-3 text-sm">
              <div class="flex flex-wrap items-center gap-1.5">
                <span class="font-medium break-all"
                  >{worktreeTitle(item.worktree)}</span
                >
                {#if item.worktree.prunable && !item.worktree.available}
                  <Badge variant="outline"
                    >{m.s8_batch_prune_stale_badge()}</Badge
                  >
                {/if}
                {#if item.archived.length > 0}
                  <Badge variant="secondary"
                    >{retained
                      ? m.s8_batch_archived_retained_released({
                          count: String(item.archived.length),
                        })
                      : m.s8_batch_archived_kept_as_is({
                          count: String(item.archived.length),
                        })}</Badge
                  >
                {/if}
              </div>
              <p class="font-mono text-xs break-all text-muted-foreground">
                {item.worktree.path}
              </p>
              {#if item.worktree.detached && !retained}
                <p class="text-xs text-destructive">
                  {m.s8_batch_detached_unprotected_warn()}
                </p>
              {/if}
              {#if item.warnings.length > 0}
                <InlineNotice tone="warning">
                  <ul class="space-y-1">
                    {#each item.warnings as warning (warning)}
                      <li>{warning}</li>
                    {/each}
                  </ul>
                </InlineNotice>
              {/if}
              {#if keepsRecords(item, false)}
                <PushCheckNotice push={item.push} />
              {/if}
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    {#if groups.blocked.length > 0}
      <section class="space-y-2.5" aria-label={m.s8_batch_prev_blocked_aria()}>
        <div class="flex flex-wrap items-baseline justify-between gap-2">
          <h3 class="text-sm font-medium">
            {m.s8_batch_prev_blocked_heading({
              count: String(groups.blocked.length),
            })}
          </h3>
          <span class="text-xs text-muted-foreground">
            {includeForce
              ? m.s8_batch_prev_force_checked()
              : m.s8_batch_prev_skip_default()}
          </span>
        </div>
        <ul class="space-y-2">
          {#each groups.blocked as item (item.worktree.id)}
            {@const activeSessions = item.sessions.filter(
              (s) => s.status !== "exited",
            )}
            <li
              class="space-y-1.5 rounded-lg border border-destructive/30 p-3 text-sm"
            >
              <div class="flex flex-wrap items-center gap-1.5">
                <span class="font-medium break-all"
                  >{worktreeTitle(item.worktree)}</span
                >
                {#each item.blockers as blocker (blocker)}
                  <Badge variant="warning">{blocker}</Badge>
                {/each}
              </div>
              <p class="font-mono text-xs break-all text-muted-foreground">
                {item.worktree.path}
              </p>
              {#if activeSessions.length > 0}
                <WorktreeSessionList
                  title={m.s8_batch_prev_active_sessions_title()}
                  sessions={activeSessions}
                />
              {/if}
              {#if item.warnings.length > 0}
                <p class="text-xs text-muted-foreground">
                  {item.warnings.join(m.s8_semicolon_sep())}
                </p>
              {/if}
              {#if includeForce && keepsRecords(item, true)}
                <PushCheckNotice push={item.push} />
              {/if}
            </li>
          {/each}
        </ul>
        <div class="rounded-lg border border-destructive/40 bg-muted/30 p-3">
          <Label class="items-start">
            <Checkbox
              bind:checked={includeForce}
              disabled={busy}
              class="mt-0.5"
            />
            <span class="text-sm leading-snug">
              {m.s8_batch_prev_force_ack({
                count: String(groups.blocked.length),
              })}
            </span>
          </Label>
        </div>
      </section>
    {/if}

    {#if groups.unsupported.length > 0}
      <section
        class="space-y-2"
        aria-label={m.s8_batch_prev_unsupported_aria()}
      >
        <h3 class="text-sm font-medium text-muted-foreground">
          {m.s8_batch_prev_unsupported_heading({
            count: String(groups.unsupported.length),
          })}
        </h3>
        <ul class="space-y-2">
          {#each groups.unsupported as item (item.worktree.id)}
            <li class="space-y-1 rounded-lg border bg-muted/30 p-3 text-sm">
              <p class="font-medium break-all">
                {worktreeTitle(item.worktree)}
              </p>
              <p class="font-mono text-xs break-all text-muted-foreground">
                {item.worktree.path}
              </p>
              <p class="text-xs break-all text-destructive">
                {item.blockers.join(m.s8_semicolon_sep())}
              </p>
            </li>
          {/each}
        </ul>
      </section>
    {/if}
  </div>
</div>
