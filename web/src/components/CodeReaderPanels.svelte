<script lang="ts">
  import type { SearchSpec } from "../code-search";
  import type { CodeHorizontalTrack } from "../code-reader-controls";
  import CodeSearchPanel from "./CodeSearchPanel.svelte";
  import CodeGotoLine from "./CodeGotoLine.svelte";

  let {
    panel,
    query,
    line,
    status,
    onquery,
    onnext,
    onprevious,
    ongoto,
    onclose,
    track,
    trackElement = $bindable(),
    ontrackscroll,
  }: {
    panel?: "search" | "goto";
    query?: SearchSpec;
    line?: number;
    status?: string;
    onquery?: (query: SearchSpec) => void;
    onnext?: () => void;
    onprevious?: () => void;
    ongoto?: (value: string) => void;
    onclose?: () => void;
    track?: CodeHorizontalTrack;
    trackElement?: HTMLDivElement;
    ontrackscroll?: () => void;
  } = $props();
</script>

{#if track}
  <div
    bind:this={trackElement}
    data-code-scrollbar
    aria-hidden="true"
    onscroll={ontrackscroll}
    class={[
      "h-3 shrink-0 overflow-x-auto overflow-y-hidden border-t bg-card",
      !track.shown && "hidden",
    ]}
  >
    <div
      style:--code-scroll-width={`${track.width}px`}
      class="h-px w-(--code-scroll-width)"
    ></div>
  </div>
{:else if panel === "search" && query && onquery && onnext && onprevious && onclose}
  <div class="shrink-0 border-b">
    <CodeSearchPanel {query} {onquery} {onnext} {onprevious} {onclose} />
    {#if status}
      <p class="px-2 pb-1 text-xs text-muted-foreground" role="status">
        {status}
      </p>
    {/if}
  </div>
{:else if panel === "goto" && line !== undefined && ongoto && onclose}
  <div class="shrink-0 border-b">
    <CodeGotoLine {line} onsubmit={ongoto} {onclose} />
  </div>
{/if}
