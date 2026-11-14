<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import InlineNotice from "../components/shared/InlineNotice.svelte";
  import type { PluginID } from "../contracts/plugin-schemas";

  let { errors }: { errors: Partial<Record<PluginID, string>> } = $props();
</script>

{#if Object.keys(errors).length}
  <div class="fixed inset-x-2 bottom-2 z-50 space-y-2">
    {#each Object.entries(errors) as [id, message] (id)}
      <InlineNotice tone="warning">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span>{id} 加载失败：{message}。其他功能仍可使用。</span>
          <Button variant="outline" size="sm" onclick={() => location.reload()}
            >重新加载页面</Button
          >
        </div>
      </InlineNotice>
    {/each}
  </div>
{/if}
