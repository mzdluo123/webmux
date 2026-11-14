<script lang="ts">
  import InlineNotice from "../shared/InlineNotice.svelte";
  import { Button } from "$lib/components/ui/button";

  let {
    prefix,
    pluginErrors,
    error,
    reloadPage,
    dismissError,
  }: {
    prefix: boolean;
    pluginErrors?: string[];
    error: string;
    reloadPage: () => void;
    dismissError: () => void;
  } = $props();
</script>

{#if prefix}
  <p class="split-notice prefix-notice" role="status">
    Ctrl+B · % 左右分屏 · " 上下分屏 · 方向键 / o 切换 · z 放大 · x 关闭窗格 ·
    Esc 取消
  </p>
{/if}
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

<style>
  .split-notice {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    justify-content: space-between;
    padding: 4px 12px;
    font-size: 12px;
    background: var(--muted);
  }
  .prefix-notice {
    position: absolute;
    inset: auto 8px 8px;
    z-index: 10;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    pointer-events: none;
  }
</style>
