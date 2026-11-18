<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";
  import { worktreeUpstreamConfirmation as confirmation } from "../../platform/worktree-upstream";
  const challenge = $derived($confirmation);
</script>

{#if challenge}
  {#key challenge.confirmationId}
    <ResponsiveDialog
      title={m.worktree_upstream_title()}
      description={m.worktree_upstream_description()}
      onclose={() => confirmation.choose(null)}
    >
      <div class="min-w-0 space-y-4">
        <dl class="grid min-w-0 gap-3 text-sm">
          <div class="min-w-0 space-y-1">
            <dt class="break-all text-muted-foreground">
              {m.worktree_upstream_local({ branch: challenge.branch })}
            </dt>
            <dd class="break-all font-mono">{challenge.localHead || "—"}</dd>
          </div>
          <div class="min-w-0 space-y-1">
            <dt class="break-all text-muted-foreground">
              {m.worktree_upstream_remote({ branch: challenge.upstream })}
            </dt>
            <dd class="break-all font-mono">{challenge.upstreamHead || "—"}</dd>
          </div>
        </dl>
        {#if challenge.warning}
          <InlineNotice tone="warning">{challenge.warning}</InlineNotice>
        {/if}
        <div class="flex flex-wrap justify-end gap-2">
          <Button variant="outline" onclick={() => confirmation.choose(null)}
            >{m.s1_cancel()}</Button
          >
          <Button variant="outline" onclick={() => confirmation.choose(false)}
            >{m.worktree_upstream_keep()}</Button
          >
          <Button
            disabled={!challenge.canUseUpstream}
            onclick={() => confirmation.choose(true)}
            >{m.worktree_upstream_latest()}</Button
          >
        </div>
      </div>
    </ResponsiveDialog>
  {/key}
{/if}
