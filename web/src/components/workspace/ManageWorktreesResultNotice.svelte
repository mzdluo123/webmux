<script lang="ts">
  import type { BatchRemoveResult } from "../../contracts/schemas";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    removed,
    skipped,
    failed,
  }: {
    removed: number;
    skipped: number;
    failed: BatchRemoveResult["results"];
  } = $props();
</script>

<div class="rounded-lg border bg-muted/40 p-3 text-sm">
  <p class="font-medium">
    已删除 {removed} 个
    {#if skipped > 0}，已跳过 {skipped} 个{/if}
    ，失败 {failed.length} 个
  </p>
  <p class="mt-1 text-xs text-muted-foreground">
    失败项的文件与未授权会话均未删除，已保留勾选以便重新读取最新状态后重试。
  </p>
</div>

<InlineNotice tone="danger">
  <ul class="space-y-1.5">
    {#each failed as item (item.worktreeId)}
      <li class="break-all">
        <span class="font-medium">{item.branch || item.path}</span>
        {#if item.branch}
          <span class="font-mono text-xs">（{item.path}）</span>
        {/if}
        ：{item.error || "删除未完成"}
      </li>
    {/each}
  </ul>
</InlineNotice>
