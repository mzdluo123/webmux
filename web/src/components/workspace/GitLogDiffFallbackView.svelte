<script lang="ts">
  import IconDownload from "~icons/tabler/download";
  import IconRefresh from "~icons/tabler/refresh";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
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
  const sideDoc = $derived(logDiff?.state.snapshotSideDoc ?? null);
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

{#if sideDoc}
  <ScrollArea class="min-h-8 flex-1">
    <div class="p-3">
      {#if sideDoc.loading}
        <p class="text-xs text-muted-foreground" role="status">
          {m.s6_diff_fallback_loading_full_doc()}
        </p>
      {:else if sideDoc.error}
        <InlineNotice tone="danger">{sideDoc.error}</InlineNotice>
      {:else if sideDoc.file}
        <div
          class="mb-2 flex items-center justify-between gap-2 text-xs text-muted-foreground"
        >
          <span class="font-mono">
            {sideDoc.side === "before"
              ? m.s6_diff_fallback_before_snapshot()
              : m.s6_diff_fallback_after_snapshot()} · {sideDoc.file.document
              .path}
          </span>
          <span
            >{m.s6_diff_fallback_line_count({
              count: String(sideDoc.file.document.lineCount),
            })}</span
          >
        </div>
        {#if sideDoc.file.document.kind === "text"}
          <div class="rounded-md border bg-background">
            <ScrollArea orientation="horizontal">
              <pre
                class="p-3 font-mono text-xs leading-relaxed text-foreground">{sideDoc
                  .file.document.text}</pre>
            </ScrollArea>
          </div>
        {:else}
          <InlineNotice tone="info">
            {sideDoc.file.document.reason || m.s6_diff_fallback_non_text_doc()}
          </InlineNotice>
        {/if}
      {/if}
    </div>
  </ScrollArea>
{:else if phase === "idle"}
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
