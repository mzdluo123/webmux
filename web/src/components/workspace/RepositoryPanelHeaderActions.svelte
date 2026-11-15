<script lang="ts">
  import IconCloudDownload from "~icons/tabler/cloud-download";
  import IconLoader2 from "~icons/tabler/loader-2";
  import { Button } from "$lib/components/ui/button";
  import type { FetchResult } from "../../contracts/schemas";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitWorkbenchButton from "./GitWorkbenchButton.svelte";

  let {
    mode = "actions",
    fetching = false,
    fetchBlocked = "",
    fetchError = "",
    fetchResults = null,
    failedFetches = 0,
    checkedAt = "",
    refreshing = false,
    refreshError = "",
    onfetch,
    ongit,
  }: {
    mode?: "actions" | "notices";
    fetching?: boolean;
    fetchBlocked?: string;
    fetchError?: string;
    fetchResults?: FetchResult[] | null;
    failedFetches?: number;
    checkedAt?: string;
    refreshing?: boolean;
    refreshError?: string;
    onfetch?: () => void;
    ongit?: () => void;
  } = $props();

  const fetchReasonId = $props.id();
  const fetchLabel = $derived(fetching ? "正在获取远程" : "获取远程");
</script>

{#if mode === "actions"}
  <div class="inline-flex items-center gap-1.5">
    {#if ongit}<GitWorkbenchButton {ongit} />{/if}
    <!-- A disabled button shows no hint, so the wrapper carries the reason. -->
    <span class="inline-flex" title={fetchBlocked || fetchLabel}>
      <Button
        variant="outline"
        size="sm"
        class="max-sm:size-(--control-default)"
        aria-label={fetchLabel}
        aria-describedby={fetchBlocked ? fetchReasonId : undefined}
        aria-busy={fetching}
        disabled={fetching || !!fetchBlocked}
        onclick={onfetch}
      >
        {#if fetching}
          <IconLoader2
            aria-hidden="true"
            class="animate-spin motion-reduce:animate-none"
          />
        {:else}
          <IconCloudDownload aria-hidden="true" />
        {/if}
        <span class="hidden sm:inline">
          {fetching ? "正在获取…" : "获取远程"}
        </span>
      </Button>
      {#if fetchBlocked}
        <span id={fetchReasonId} class="sr-only">{fetchBlocked}</span>
      {/if}
    </span>
  </div>
{:else}
  {#if refreshing}
    <p role="status" class="text-xs text-muted-foreground">
      正在重新读取状态…（先显示上次读取的结果）
    </p>
  {:else if refreshError}
    <InlineNotice tone="danger">
      重新读取失败：{refreshError}。以下为 {new Date(
        checkedAt,
      ).toLocaleString()} 读取的状态，可能已过时。
    </InlineNotice>
  {/if}
  {#if fetchError}
    <InlineNotice tone="danger">获取远程失败：{fetchError}</InlineNotice>
  {:else if fetchResults}
    {@const tone =
      failedFetches === 0
        ? "success"
        : failedFetches === fetchResults.length
          ? "danger"
          : "warning"}
    <InlineNotice {tone}>
      <ul class="space-y-1" aria-label="获取结果">
        {#each fetchResults as result (result.remote)}
          <li class="wrap-anywhere">
            <span class="font-medium">{result.remote}</span>：{result.status ===
            "ok"
              ? "已获取"
              : result.message || "获取失败"}
          </li>
        {/each}
      </ul>
    </InlineNotice>
  {/if}
{/if}
