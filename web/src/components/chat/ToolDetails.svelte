<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import { m } from "$lib/paraglide/messages.js";
  import { diffStats, type ToolEntry } from "../../domains/chat/chat-model";
  import {
    isExternalLink,
    parseChatCodeLink,
    type ChatCodeLink,
    type ChatCodeLinkHandler,
  } from "../../domains/code/markdown-link";
  import DiffCount from "./DiffCount.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";

  // What a tool call produced: output text, diffs, resource links and the
  // locations it touched. Diffs open in the code panel.
  let {
    tool,
    onreview,
    onopenlocation,
  }: {
    tool: ToolEntry;
    /** Opens a changed file in the code panel. */
    onreview?: (path: string) => void;
    /** Opens a referenced code file or line in the code panel. */
    onopenlocation?: ChatCodeLinkHandler;
  } = $props();

  const blocks = $derived(
    tool.content.map((block) => ({
      block,
      stats: block.type === "diff" ? diffStats(block) : null,
    })),
  );
  let linkFailure = $state("");
  async function openLocation(target: ChatCodeLink) {
    linkFailure = "";
    const failure = await onopenlocation?.(target);
    if (failure) linkFailure = failure;
  }
</script>

{#each blocks as { block, stats }, i (i)}
  {#if block.type === "text"}
    <pre
      class="overflow-x-auto rounded-md bg-muted px-3 py-2 font-mono text-xs break-all whitespace-pre-wrap">{block.text}</pre>
  {:else if block.type === "diff"}
    <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
      <span class="min-w-0 font-mono text-xs break-all text-foreground"
        >{block.path}</span
      >
      {#if stats}<DiffCount added={stats.added} removed={stats.removed} />{/if}
      {#if block.path && onreview}
        {@const path = block.path}
        <Button variant="outline" size="xs" onclick={() => onreview?.(path)}
          >{m.s3_view_in_code_panel()}</Button
        >
      {/if}
    </div>
  {:else if block.type === "resource_link"}
    {@const label = block.name || block.uri}
    {@const target =
      block.uri && onopenlocation ? parseChatCodeLink(block.uri) : null}
    {#if block.uri && isExternalLink(block.uri)}
      <p>
        <ListButton
          variant="code-link"
          href={block.uri}
          target="_blank"
          rel="noopener noreferrer">{label}</ListButton
        >
      </p>
    {:else if target}
      <p>
        <ListButton
          variant="code-link"
          onclick={() => void openLocation(target)}>{label}</ListButton
        >
      </p>
    {:else}
      <p class="font-mono text-xs break-all">{label}</p>
    {/if}
  {:else}
    <p>
      {m.s3_unsupported_content({
        type: block.unsupported || m.s3_unknown_type(),
      })}
    </p>
  {/if}
  {#if block.truncated}<p class="text-xs">
      {m.s3_content_too_long_truncated()}
    </p>{/if}
{:else}
  <p>
    {tool.partial
      ? m.s3_tool_started_earlier_uncached()
      : m.s3_tool_no_output()}
  </p>
{/each}
{#each tool.locations as location, i (i)}
  {@const label = `${location.path}${location.line === undefined ? "" : `:${location.line}`}`}
  {#if onopenlocation && location.path}
    <p>
      <ListButton
        variant="code-link"
        onclick={() =>
          void openLocation({
            path: location.path,
            ...(location.line !== undefined && location.line >= 1
              ? { line: location.line }
              : {}),
          })}>{label}</ListButton
      >
    </p>
  {:else}
    <p class="font-mono text-xs break-all">{label}</p>
  {/if}
{/each}
{#if linkFailure}<InlineNotice tone="warning" class="mt-1"
    >{linkFailure}</InlineNotice
  >{/if}
