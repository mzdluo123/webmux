<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { InlineNotice } from "$lib/components/ui/inline-notice";

  let {
    text = "仓库详情加载失败，请刷新页面后重试。",
    tone = "danger",
    surface = "inline",
    showAction = true,
    actionSlot = true,
    onaction = () => {},
  }: {
    text?: string;
    tone?: "info" | "success" | "warning" | "danger";
    surface?: "inline" | "floating";
    showAction?: boolean;
    actionSlot?: boolean;
    onaction?: () => void;
  } = $props();
</script>

{#if actionSlot}
  <InlineNotice {tone} {surface}>
    <span data-testid="notice-copy">{text}</span>
    {#snippet actions()}
      {#if showAction}
        <Button variant="outline" size="sm" onclick={onaction}>关闭</Button>
      {/if}
    {/snippet}
  </InlineNotice>
{:else}
  <InlineNotice {tone} {surface}>
    <span data-testid="notice-copy">{text}</span>
  </InlineNotice>
{/if}
