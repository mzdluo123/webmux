<script lang="ts">
  import type { CodeReaderPanel } from "../../domains/code/code-reader-controls";
  import type { SearchSpec } from "../../domains/code/code-search";
  import CodeSearchPanel from "./CodeSearchPanel.svelte";
  import CodeGotoLine from "./CodeGotoLine.svelte";
  import CodeZoomPanel from "./CodeZoomPanel.svelte";

  let {
    panel,
    searchAtTop = false,
    query,
    line,
    status,
    onquery,
    onnext,
    onprevious,
    ongoto,
    onclose,
  }: {
    panel?: CodeReaderPanel;
    searchAtTop?: boolean;
    query: SearchSpec;
    line: number;
    status?: string;
    onquery: (query: SearchSpec) => void;
    onnext: () => void;
    onprevious: () => void;
    ongoto: (value: string) => void;
    onclose: () => void;
  } = $props();
</script>

{#if panel}
  <div
    class={[
      "max-h-[45%] shrink-0 overflow-auto bg-background",
      searchAtTop && panel === "search"
        ? "order-first w-full self-end border-b sm:max-w-sm sm:border-l"
        : "border-t",
    ]}
  >
    {#if panel === "search"}
      <CodeSearchPanel {query} {onquery} {onnext} {onprevious} {onclose} />
      {#if status}
        <p class="px-2 pb-1 text-xs text-muted-foreground" role="status">
          {status}
        </p>
      {/if}
    {:else if panel === "goto"}
      <CodeGotoLine {line} onsubmit={ongoto} {onclose} />
    {:else}
      <CodeZoomPanel {onclose} />
    {/if}
  </div>
{/if}
