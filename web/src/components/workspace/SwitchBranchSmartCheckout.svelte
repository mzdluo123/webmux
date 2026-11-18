<script lang="ts">
  import IconArchive from "~icons/tabler/archive";
  import IconArrowsSplit2 from "~icons/tabler/arrows-split-2";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconSwitchHorizontal from "~icons/tabler/switch-horizontal";
  import { m } from "$lib/paraglide/messages.js";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import type { Worktree } from "../../contracts/schemas";
  import type { createSmartCheckoutController } from "../../hooks/workspace/git-smart-checkout.svelte";
  import BlockerList from "../shared/BlockerList.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import GitRecoveryCommandBox from "./GitRecoveryCommandBox.svelte";

  type SmartController = ReturnType<typeof createSmartCheckoutController>;

  let {
    smart,
    worktree,
    busy = false,
    onopenconflict,
  }: {
    smart: SmartController;
    worktree: Worktree;
    busy?: boolean;
    onopenconflict?: (tree: Worktree, path?: string) => void;
  } = $props();

  const short = (oid: string | undefined) => (oid ? oid.slice(0, 7) : "");
  const showUntrackedToggle = $derived(
    Boolean(
      worktree.hasUntracked ||
      (worktree.changes?.untracked ?? 0) > 0 ||
      (smart.preview?.untrackedCount ?? 0) > 0 ||
      smart.includeUntracked,
    ),
  );
  const nonSessionWarnings = $derived(
    (smart.preview?.warnings ?? []).filter((w) => w.code !== "sessions"),
  );
</script>

{#if smart.execError}
  <InlineNotice tone="danger">{smart.execError}</InlineNotice>
{/if}
{#if smart.conflictMessage}
  <InlineNotice tone="warning">
    {m.s8_smart_conflict_refreshed({ error: smart.conflictMessage })}
  </InlineNotice>
{/if}

{#if smart.lastResult?.outcome === "rolled_back"}
  <InlineNotice tone="warning">
    <div class="space-y-1" data-testid="smart-checkout-rolled-back-notice">
      <p class="font-medium text-foreground">
        {m.s8_smart_rolled_back_title()}
      </p>
      <p class="text-xs text-muted-foreground">{smart.lastResult.summary}</p>
    </div>
  </InlineNotice>
{:else if smart.lastResult?.outcome === "conflict_needs_resolution"}
  {@const res = smart.lastResult}
  <section
    aria-label={m.s8_smart_conflict_guide_aria()}
    class="space-y-3 rounded-lg border border-border bg-background p-3 text-xs"
    data-testid="smart-checkout-conflict-card"
  >
    <div class="flex flex-wrap items-center gap-2">
      <Badge variant="warning">{m.s8_smart_restore_conflict_badge()}</Badge>
      {#if res.stashPreserved && res.stashOid}
        <Badge variant="outline">
          <IconArchive aria-hidden="true" />
          {m.s8_smart_stash_preserved({ oid: short(res.stashOid) })}
        </Badge>
      {/if}
    </div>
    <p class="leading-relaxed text-foreground">{res.summary}</p>
    {#if res.recoveryCommand}
      <GitRecoveryCommandBox
        label={m.s8_smart_recovery_cmd_label()}
        copyLabel={m.s8_smart_copy_recovery_cmd()}
        command={res.recoveryCommand}
        testId="smart-checkout-recovery-cmd"
      />
    {/if}
    {#if onopenconflict}
      <div class="flex flex-wrap items-center gap-2">
        <Button
          type="button"
          variant="default"
          wrap
          onclick={() =>
            onopenconflict(
              res.worktree,
              res.inProgress.conflictedFiles[0]?.path,
            )}
          data-testid="smart-checkout-open-conflict-btn"
        >
          <IconArrowsSplit2 aria-hidden="true" />
          {m.s8_smart_open_conflict_resolver({
            count: String(res.inProgress.conflictedCount),
          })}
        </Button>
      </div>
    {/if}
  </section>
{/if}

{#if smart.visible && smart.lastResult?.outcome !== "conflict_needs_resolution"}
  <section
    aria-label={m.s8_smart_section_aria()}
    class="space-y-3 rounded-lg border bg-muted/20 p-3 text-sm"
    data-testid="smart-checkout-section"
  >
    {#if smart.previewing && !smart.preview}
      <p role="status" class="text-xs text-muted-foreground">
        {m.s8_smart_evaluating()}
      </p>
    {:else if smart.previewError}
      <p role="alert" class="text-xs break-all text-destructive">
        {m.s8_smart_eval_failed({ error: smart.previewError })}
      </p>
    {:else if smart.preview}
      {@const sp = smart.preview}
      {#if smart.hasActiveSessions}
        <div class="space-y-2.5">
          <p class="text-xs leading-relaxed text-muted-foreground">
            {m.s8_smart_active_sessions_lead()}
            <strong class="text-foreground"
              >{m.s8_smart_active_sessions_cta()}</strong
            >{m.s8_smart_active_sessions_tail()}
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            wrap
            disabled={busy}
            aria-expanded={smart.disclosureOpen}
            onclick={() => (smart.disclosureOpen = !smart.disclosureOpen)}
            data-testid="smart-checkout-session-disclosure"
          >
            {#if smart.disclosureOpen}
              <IconChevronDown aria-hidden="true" />
            {:else}
              <IconChevronRight aria-hidden="true" />
            {/if}
            {m.s8_smart_in_place_disclosure()}
          </Button>
        </div>
      {/if}

      {#if !smart.hasActiveSessions || smart.disclosureOpen}
        <div class="space-y-3" data-testid="smart-checkout-body">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <span class="flex items-center gap-1.5 text-xs font-semibold">
              <IconSwitchHorizontal
                class="size-4 shrink-0 text-muted-foreground"
                aria-hidden="true"
              />
              {m.s8_smart_heading()}
            </span>
            <div class="flex flex-wrap items-center gap-1.5">
              <Badge variant="outline"
                >{m.s8_changes_staged()} {sp.stagedCount}</Badge
              >
              <Badge variant="outline"
                >{m.s8_changes_unstaged()} {sp.unstagedCount}</Badge
              >
              {#if sp.untrackedCount > 0}
                <Badge variant="outline"
                  >{m.s8_changes_untracked()} {sp.untrackedCount}</Badge
                >
              {/if}
            </div>
          </div>

          <p class="text-xs leading-relaxed text-muted-foreground">
            {m.s8_smart_steps_desc()}
          </p>

          {#if sp.blockers.length > 0}
            <BlockerList
              title={m.s8_smart_cannot_exec_title()}
              blockers={sp.blockers.map((b) => b.message)}
            />
          {/if}

          {#if nonSessionWarnings.length > 0}
            <ul class="space-y-1 text-xs text-muted-foreground">
              {#each nonSessionWarnings as w (w.code)}
                <li class="break-all">{w.message}</li>
              {/each}
            </ul>
          {/if}

          {#if showUntrackedToggle}
            <Label class="items-start">
              <Checkbox
                checked={smart.includeUntracked}
                onCheckedChange={(v) => smart.setIncludeUntracked(Boolean(v))}
                disabled={busy || smart.previewing}
                class="mt-0.5"
                data-testid="smart-checkout-include-untracked"
              />
              <span class="text-xs leading-snug">
                {m.s8_smart_include_untracked_label()}
              </span>
            </Label>
          {/if}

          {#if smart.hasActiveSessions && sp.blockers.length === 0}
            <Label class="items-start">
              <Checkbox
                bind:checked={smart.acknowledgeSessions}
                disabled={busy}
                class="mt-0.5"
                data-testid="smart-checkout-ack-sessions"
              />
              <span class="text-xs leading-snug">
                {m.s8_smart_ack_sessions_label()}
              </span>
            </Label>
          {/if}

          <div class="flex flex-wrap items-center justify-end gap-2">
            <Button
              type="button"
              variant={smart.hasActiveSessions ? "outline" : "default"}
              size="sm"
              wrap
              disabled={!smart.canExecute}
              onclick={() => void smart.execute()}
              data-testid="smart-checkout-submit-btn"
            >
              <IconSwitchHorizontal aria-hidden="true" />
              {busy
                ? m.s8_smart_executing()
                : smart.hasActiveSessions
                  ? m.s8_smart_confirm_in_place()
                  : m.s8_smart_submit_btn()}
            </Button>
          </div>
        </div>
      {/if}
    {/if}
  </section>
{/if}
