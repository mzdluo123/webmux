<script lang="ts">
  import IconArrowRight from "~icons/tabler/arrow-right";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import type { SwitchPreview, Worktree } from "../../contracts/schemas";
  import BlockerList from "../shared/BlockerList.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import WorktreeSessionList from "./WorktreeSessionList.svelte";

  let {
    preview,
    previewing = false,
    previewError = "",
    conflict = "",
    worktree,
    acknowledged = $bindable(false),
    busy = false,
    onRetry,
  }: {
    preview: SwitchPreview | null;
    previewing?: boolean;
    previewError?: string;
    conflict?: string;
    worktree: Worktree;
    acknowledged?: boolean;
    busy?: boolean;
    onRetry: () => void;
  } = $props();

  const short = (hash: string | null | undefined) => hash?.slice(0, 8) ?? "";
  const currentLabel = $derived.by(() => {
    const tree = preview?.worktree ?? worktree;
    return (
      tree.branch ??
      m.s8_detached_dot({ ref: short(tree.head) || m.s8_no_commits_short() })
    );
  });
  const sessionsTitle = $derived(
    preview?.warnings.find((item) => item.code === "sessions")?.message ??
      m.s8_running_sessions_title(),
  );
  const otherWarnings = $derived(
    (preview?.warnings ?? []).filter((item) => item.code !== "sessions"),
  );
  const agentWorking = $derived(
    (preview?.sessions ?? []).some(
      (item) => item.agent !== null && item.status === "running",
    ),
  );
</script>

{#if previewing}
  <p role="status" class="text-sm text-muted-foreground">
    {m.s8_previewing_switch()}
  </p>
{:else if previewError}
  <p role="alert" class="text-sm break-all text-destructive">
    {m.s8_cannot_preview({ error: previewError })}
  </p>
  <Button type="button" variant="outline" size="sm" onclick={onRetry}>
    {m.s8_reload_preview_btn()}
  </Button>
{/if}
{#snippet conflictNotice()}
  {#if conflict}
    <InlineNotice tone="warning">
      {m.s8_conflict_repreviewed({ error: conflict })}
    </InlineNotice>
  {/if}
{/snippet}

{#if !preview}
  {@render conflictNotice()}
{/if}
{#if preview}
  <section aria-label={m.s8_switch_preview_aria()} class="space-y-3">
    {#if preview.blockers.length}
      <div role="alert">
        <BlockerList
          title={m.s8_cannot_switch_title()}
          blockers={preview.blockers.map((item) => item.message)}
        />
      </div>
    {/if}
    {@render conflictNotice()}
    <div class="space-y-1 rounded-lg border p-3 text-sm">
      <p
        class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 font-medium"
      >
        <span class="break-all">{currentLabel}</span>
        <IconArrowRight
          class="size-4 shrink-0 text-muted-foreground"
          aria-hidden="true"
        /><span class="sr-only">{m.s8_switch_to_sr()}</span>
        <span class="break-all"
          >{preview.target.branch ??
            m.s8_detached_dot({ ref: short(preview.target.commit) })}</span
        >
      </p>
      <p class="text-xs break-all text-muted-foreground">
        <span class="font-mono">{short(preview.target.commit)}</span>
        · {preview.target.subject || m.s8_untitled_commit()}
      </p>
      {#if preview.target.createsBranch}<p
          class="text-xs break-all text-muted-foreground"
        >
          {preview.target.upstream
            ? m.s8_switch_creates_branch_with_upstream({
                branch: preview.target.branch ?? "",
                upstream: preview.target.upstream,
              })
            : m.s8_switch_creates_branch_no_upstream({
                branch: preview.target.branch ?? "",
              })}
        </p>{/if}
    </div>
    {#if otherWarnings.length}
      <InlineNotice tone="warning">
        <ul class="space-y-1">
          {#each otherWarnings as item (item.code)}<li class="break-all">
              {item.message}
            </li>{/each}
        </ul>
      </InlineNotice>
    {/if}
    {#if preview.sessions.length}
      <WorktreeSessionList title={sessionsTitle} sessions={preview.sessions} />
      {#if agentWorking}<InlineNotice tone="warning"
          >{m.s8_switch_agent_working_warn()}</InlineNotice
        >{/if}
      {#if !preview.blockers.length}
        <Label class="items-start">
          <Checkbox
            bind:checked={acknowledged}
            disabled={busy}
            class="mt-0.5"
          />
          <span class="text-sm leading-snug">{m.s8_switch_ack_sessions()}</span>
        </Label>
      {/if}
    {/if}
    {#if !preview.blockers.length}<p class="text-xs text-muted-foreground">
        {m.s8_switch_preview_footer()}
      </p>{/if}
  </section>
{/if}
