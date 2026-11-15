<script lang="ts">
  import type { Component } from "svelte";
  import IconArrowsMove from "~icons/tabler/arrows-move";
  import IconBulb from "~icons/tabler/bulb";
  import IconCheck from "~icons/tabler/check";
  import IconCircleDashed from "~icons/tabler/circle-dashed";
  import IconFileText from "~icons/tabler/file-text";
  import IconLoader2 from "~icons/tabler/loader-2";
  import IconPencil from "~icons/tabler/pencil";
  import IconSearch from "~icons/tabler/search";
  import IconShieldQuestion from "~icons/tabler/shield-question";
  import IconSwitchHorizontal from "~icons/tabler/switch-horizontal";
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import IconTool from "~icons/tabler/tool";
  import IconTrash from "~icons/tabler/trash";
  import IconWorld from "~icons/tabler/world";
  import IconX from "~icons/tabler/x";
  import { m } from "$lib/paraglide/messages.js";
  import { diffStats, type ToolEntry } from "../../domains/chat/chat-model";
  import DiffCount from "./DiffCount.svelte";

  // The one-line summary of a tool call: kind, title, diff size and status.
  // It renders inside a disclosure trigger, so it holds no interactive parts.
  let {
    tool,
    waiting = false,
  }: {
    tool: ToolEntry;
    /** A permission request on this call waits for an answer. */
    waiting?: boolean;
  } = $props();

  const kinds: Record<string, Component> = {
    read: IconFileText,
    edit: IconPencil,
    delete: IconTrash,
    move: IconArrowsMove,
    search: IconSearch,
    execute: IconTerminal2,
    think: IconBulb,
    fetch: IconWorld,
    switch_mode: IconSwitchHorizontal,
  };
  const KindIcon = $derived(kinds[tool.kind] ?? IconTool);
  const stats = $derived(
    tool.content.flatMap((block) =>
      block.type === "diff" ? [diffStats(block)] : [],
    ),
  );
  const added = $derived(stats.reduce((sum, item) => sum + item.added, 0));
  const removed = $derived(stats.reduce((sum, item) => sum + item.removed, 0));
  const rejected = $derived(
    tool.permission?.outcome === "cancelled" ||
      !!tool.permission?.kind?.startsWith("reject"),
  );
  const isCompleted = $derived(
    !waiting &&
      tool.permission?.outcome !== "cancelled" &&
      !rejected &&
      tool.status !== "in_progress" &&
      tool.status !== "pending" &&
      tool.status !== "failed",
  );
  const status = $derived(
    waiting
      ? m.s3_waiting_permission()
      : tool.permission?.outcome === "cancelled"
        ? m.s3_tool_status_cancelled()
        : rejected
          ? m.s3_tool_status_rejected()
          : tool.status === "in_progress"
            ? m.s3_status_in_progress()
            : tool.status === "pending"
              ? m.s3_tool_status_pending()
              : tool.status === "failed"
                ? m.s3_tool_status_failed()
                : m.s3_status_completed(),
  );
</script>

<KindIcon class="size-3.5 shrink-0" aria-hidden="true" />
<span
  class={tool.kind === "execute"
    ? "min-w-0 truncate font-mono text-xs text-foreground"
    : "min-w-0 truncate text-foreground"}
  >{tool.title ||
    (tool.partial ? m.s3_tool_earlier_call() : m.s3_tool_call())}</span
>
{#if added || removed}<DiffCount {added} {removed} />{/if}
<span
  class={waiting || tool.status === "failed" || rejected
    ? "flex shrink-0 items-center gap-1 text-xs text-destructive"
    : tool.status === "pending"
      ? "flex shrink-0 items-center gap-1 text-xs text-warning"
      : "flex shrink-0 items-center gap-1 text-xs"}
>
  {#if waiting}<IconShieldQuestion
      class="size-3.5"
      aria-hidden="true"
    />{:else if tool.status === "in_progress"}<IconLoader2
      class="size-3.5 animate-spin text-info motion-reduce:animate-none"
      aria-hidden="true"
    />{:else if tool.status === "pending"}<IconCircleDashed
      class="size-3.5"
      aria-hidden="true"
    />{:else if tool.status === "failed" || rejected}<IconX
      class="size-3.5"
      aria-hidden="true"
    />{:else}<IconCheck
      class="size-3.5 text-success"
      aria-hidden="true"
    />{/if}<span class={isCompleted ? "sr-only" : undefined}>{status}</span>
</span>
