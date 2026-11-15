<script lang="ts">
  import IconDownload from "~icons/tabler/download";
  import IconRefresh from "~icons/tabler/refresh";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitDiffPanelReader } from "../../hooks/workspace/git-log-diff.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    phase,
    reason,
    logDiff,
  }: {
    phase: "idle" | "loading" | "binary" | "limited" | "error";
    reason: string;
    logDiff?: GitDiffPanelReader;
  } = $props();

  const rawDiff = $derived(logDiff?.state.rawDiff ?? null);
</script>

{#snippet downloadSideButton(side: "before" | "after", label: string)}
  {#if logDiff}
    <Button
      size="sm"
      variant="outline"
      disabled={logDiff.state.downloadingSide !== null}
      onclick={() => void logDiff.downloadSnapshotSide(side)}
    >
      <IconDownload aria-hidden="true" />
      {label}
    </Button>
  {/if}
{/snippet}

{#if phase === "idle"}
  <div
    class="flex flex-1 items-center justify-center p-4 text-xs text-muted-foreground"
  >
    {m.s6_diff_fallback_idle_hint()}
  </div>
{:else if phase === "loading"}
  <div
    class="flex flex-1 items-center justify-center p-4 text-xs text-muted-foreground"
    role="status"
  >
    {m.s6_diff_fallback_loading_diff()}
  </div>
{:else if phase === "binary"}
  <div class="space-y-2.5 p-4 text-xs">
    <InlineNotice tone="info">
      {reason || m.s6_binary_no_text_diff()}
    </InlineNotice>
    {#if rawDiff && logDiff}
      <div class="flex flex-wrap items-center gap-2 text-muted-foreground">
        {#if rawDiff.old.source !== "empty"}
          <span
            >{m.s6_diff_fallback_before_size({
              size: String(rawDiff.old.size),
              mediaType: rawDiff.old.mediaType || "binary",
            })}</span
          >
          {#if rawDiff.old.downloadable}
            {@render downloadSideButton(
              "before",
              m.s6_diff_fallback_download_before(),
            )}
          {/if}
        {/if}
        {#if rawDiff.new.source !== "empty"}
          <span
            >{m.s6_diff_fallback_after_size({
              size: String(rawDiff.new.size),
              mediaType: rawDiff.new.mediaType || "binary",
            })}</span
          >
          {#if rawDiff.new.downloadable}
            {@render downloadSideButton(
              "after",
              m.s6_diff_fallback_download_after(),
            )}
          {/if}
        {/if}
      </div>
    {/if}
  </div>
{:else if phase === "limited"}
  <div class="space-y-2.5 p-4 text-xs">
    <InlineNotice tone="warning">
      {reason || m.s6_diff_fallback_limited_reason()}
    </InlineNotice>
    {#if rawDiff && logDiff}
      <div class="flex flex-wrap items-center gap-2">
        {#if rawDiff.old.source !== "empty" && rawDiff.old.downloadable}
          {@render downloadSideButton(
            "before",
            m.s6_diff_fallback_download_raw_before(),
          )}
        {/if}
        {#if rawDiff.new.source !== "empty" && rawDiff.new.downloadable}
          {@render downloadSideButton(
            "after",
            m.s6_diff_fallback_download_raw_after(),
          )}
        {/if}
      </div>
    {/if}
  </div>
{:else if phase === "error"}
  <div class="space-y-2 p-4 text-xs">
    <InlineNotice tone="danger">
      {m.s6_diff_fallback_error({ reason })}
    </InlineNotice>
    {#if logDiff}
      <Button
        size="sm"
        variant="outline"
        onclick={() => logDiff.retryActiveDiff()}
      >
        <IconRefresh aria-hidden="true" />
        {m.s6_diff_fallback_retry()}
      </Button>
    {/if}
  </div>
{/if}
