<script lang="ts">
  import IconDownload from "~icons/tabler/download";
  import IconRefresh from "~icons/tabler/refresh";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitDiffPanelReader } from "../../hooks/workspace/git-log-diff.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import type { GitDiff, GitSnapshotSide } from "../../contracts/schemas";
  import CodeImageView from "../code/CodeImageView.svelte";
  import CodePdfView from "../code/CodePdfView.svelte";

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

  function previewKind(doc: GitDiff["old"]) {
    if (doc.kind === "pdf") return "pdf";
    if (
      doc.kind === "image" ||
      (doc.kind === "text" && doc.mediaType === "image/svg+xml")
    )
      return "image";
    return null;
  }

  const hasPreview = $derived(
    rawDiff && (previewKind(rawDiff.old) || previewKind(rawDiff.new)),
  );
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

{#snippet binarySide(
  reader: GitDiffPanelReader,
  snapshotId: string,
  side: GitSnapshotSide,
  doc: GitDiff["old"],
)}
  {#if doc.source !== "empty"}
    {@const kind = previewKind(doc)}
    {@const before = side === "before"}
    {@const read = (signal: AbortSignal) =>
      reader.readSnapshotSide(snapshotId, side, signal)}
    <div class="min-w-0 flex-1 basis-64 space-y-2">
      <div
        class="flex flex-wrap items-center justify-between gap-2 text-muted-foreground"
      >
        {#if kind}
          <span>{before ? m.s5_file_before() : m.s5_file_after()}</span>
        {:else}
          {@const size = {
            size: String(doc.size),
            mediaType: doc.mediaType || "binary",
          }}
          <span
            >{before
              ? m.s6_diff_fallback_before_size(size)
              : m.s6_diff_fallback_after_size(size)}</span
          >
        {/if}
        {#if doc.downloadable}
          {@render downloadSideButton(
            side,
            before
              ? m.s6_diff_fallback_download_before()
              : m.s6_diff_fallback_download_after(),
          )}
        {/if}
      </div>
      {#if kind}
        <div class="flex h-56 min-w-0 flex-col">
          {#if kind === "image"}
            <CodeImageView source={read} document={doc} />
          {:else}
            <CodePdfView source={read} document={doc} />
          {/if}
        </div>
      {/if}
    </div>
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
  <ScrollArea class="min-h-0 flex-1">
    <div class="space-y-2.5 p-4 text-xs">
      {#if !hasPreview}
        <InlineNotice tone="info">
          {reason || m.s6_binary_no_text_diff()}
        </InlineNotice>
      {/if}
      {#if rawDiff && logDiff}
        {#key rawDiff.snapshotId}
          <div class="flex flex-wrap gap-4">
            {@render binarySide(
              logDiff,
              rawDiff.snapshotId,
              "before",
              rawDiff.old,
            )}
            {@render binarySide(
              logDiff,
              rawDiff.snapshotId,
              "after",
              rawDiff.new,
            )}
          </div>
        {/key}
      {/if}
    </div>
  </ScrollArea>
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
