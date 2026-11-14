<script lang="ts">
  import InlineNotice from "../shared/InlineNotice.svelte";
  import { Button } from "$lib/components/ui/button";

  let {
    pluginErrors,
    error,
    reloadPage,
    dismissError,
  }: {
    pluginErrors?: string[];
    error: string;
    reloadPage: () => void;
    dismissError: () => void;
  } = $props();
</script>

{#if pluginErrors?.length}
  <InlineNotice tone="warning">
    <div class="flex items-center justify-between gap-2">
      <span>部分插件未能加载；终端仍可使用。{pluginErrors.join("；")}</span>
      <Button variant="outline" size="sm" onclick={reloadPage}
        >重新加载页面</Button
      >
    </div>
  </InlineNotice>
{/if}
{#if error}
  <InlineNotice tone="danger">
    <div class="flex items-center justify-between gap-2">
      <span>{error}</span>
      <Button variant="ghost" size="sm" onclick={dismissError}>关闭</Button>
    </div>
  </InlineNotice>
{/if}
