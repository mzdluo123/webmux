<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitCommitOutcome } from "../../contracts/git-workbench-schemas";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    outcome,
    onInspectCommit,
    onReloadVerify,
    onDismiss,
  }: {
    outcome: GitCommitOutcome;
    onInspectCommit: (commitOid: string) => void;
    onReloadVerify: () => void;
    onDismiss: () => void;
  } = $props();
</script>

<div
  class="border-b bg-background px-3 py-2"
  data-testid="git-commit-outcome-banner"
>
  <InlineNotice tone={outcome.status === "busy_locked" ? "danger" : "warning"}>
    <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
      <div class="min-w-0">
        <span class="font-semibold">[{outcome.status}]</span>
        {outcome.summary}
        {#if outcome.reasons.length > 0}
          {m.s6_outcome_reasons_wrap({
            reasons: outcome.reasons.join(m.s6_list_semicolon_sep()),
          })}
        {/if}
      </div>
      <div class="flex min-w-0 max-w-full flex-wrap items-center gap-1.5">
        {#if outcome.commitOid}
          <Button
            size="sm"
            variant="outline"
            wrap
            onclick={() =>
              outcome.commitOid && onInspectCommit(outcome.commitOid)}
          >
            {m.s6_outcome_inspect_in_log()}
          </Button>
        {/if}
        {#if outcome.status === "uncertain"}
          <Button size="sm" variant="outline" wrap onclick={onReloadVerify}>
            {m.s6_reload_verify_btn()}
          </Button>
        {/if}
        <Button size="sm" variant="ghost" wrap onclick={onDismiss}>
          {m.s6_dismiss_notice()}
        </Button>
      </div>
    </div>
  </InlineNotice>
</div>
