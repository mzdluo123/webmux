<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import InlineNotice from "../components/shared/InlineNotice.svelte";
  import type { PluginID } from "../contracts/plugin-schemas";

  let { errors }: { errors: Partial<Record<PluginID, string>> } = $props();
  let dismissed = $state<Partial<Record<PluginID, string>>>({});
  const visibleErrors = $derived(
    Object.entries(errors).filter(
      ([id, message]) => dismissed[id as PluginID] !== message,
    ),
  );
</script>

{#if visibleErrors.length}
  <div
    class="fixed inset-x-2 bottom-2 z-50 max-h-[50dvh] space-y-2 overflow-y-auto"
  >
    {#each visibleErrors as [id, message] (id)}
      <InlineNotice tone="warning">
        <div class="min-w-0 space-y-2">
          <p class="break-words">
            {id} 加载失败：{message}。其他功能仍可使用。
          </p>
          <div class="flex flex-wrap justify-end gap-2">
            <Button
              variant="ghost"
              size="sm"
              aria-label={`关闭 ${id} 加载提示`}
              onclick={() => (dismissed = { ...dismissed, [id]: message })}
              >关闭提示</Button
            >
            <Button
              variant="outline"
              size="sm"
              onclick={() => location.reload()}>重新加载页面</Button
            >
          </div>
        </div>
      </InlineNotice>
    {/each}
  </div>
{/if}
