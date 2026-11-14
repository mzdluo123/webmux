<script lang="ts">
  import { Button } from "$lib/components/ui/button";
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
          （{outcome.reasons.join("；")}）
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
            在历史 (Log) 中查看提交
          </Button>
        {/if}
        {#if outcome.status === "uncertain"}
          <Button size="sm" variant="outline" wrap onclick={onReloadVerify}>
            刷新历史与暂存区核实
          </Button>
        {/if}
        <Button size="sm" variant="ghost" wrap onclick={onDismiss}>
          关闭提示
        </Button>
      </div>
    </div>
  </InlineNotice>
</div>
