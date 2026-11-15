<script lang="ts">
  import IconFile from "~icons/tabler/file";
  import IconHandStop from "~icons/tabler/hand-stop";
  import * as m from "$lib/paraglide/messages.js";
  import CodeMarkdownPreview from "../code/CodeMarkdownPreview.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import type { DisplayEntry } from "../../domains/chat/chat-model";
  import type { ChatCodeLinkHandler } from "../../domains/code/markdown-link";
  import type { StopReason } from "../../contracts/schemas";
  import { sessionReferenceParts } from "../../domains/sessions/session-reference";
  import ChatAttachmentImage from "./ChatAttachmentImage.svelte";
  import ChatDisclosure from "./ChatDisclosure.svelte";
  import TurnChanges from "./TurnChanges.svelte";
  import ToolGroup from "./ToolGroup.svelte";

  // One transcript entry: the user's turn is a bubble, the agent's reply is
  // prose, and its work is a column of quiet one-line rows.
  let {
    entry,
    sessionId,
    preview,
    waiting,
    onreview,
    onopenlocation,
  }: {
    entry: DisplayEntry;
    sessionId?: string;
    preview?: (path: string) => string | undefined;
    /** Tool calls whose permission request waits for an answer. */
    waiting: readonly string[];
    onreview?: (path: string) => void;
    onopenlocation?: ChatCodeLinkHandler;
  } = $props();

  function turnEndInfo(
    reason: StopReason,
  ): { text: string; error: boolean } | undefined {
    switch (reason) {
      case "cancelled":
        return { text: m.s3_turn_end_cancelled(), error: false };
      case "max_tokens":
        return { text: m.s3_turn_end_max_tokens(), error: false };
      case "max_turn_requests":
        return { text: m.s3_turn_end_max_requests(), error: false };
      case "refusal":
        return { text: m.s3_turn_end_refusal(), error: true };
      case "error":
        return { text: m.s3_turn_end_error(), error: true };
      case "auth_required":
        return { text: m.s3_turn_end_auth_required(), error: true };
      default:
        return undefined;
    }
  }
  function markerLabel(kind: "loaded" | "truncated" | "reconnected"): string {
    switch (kind) {
      case "loaded":
        return m.s3_marker_loaded();
      case "truncated":
        return m.s3_marker_truncated();
      case "reconnected":
        return m.s3_marker_reconnected();
    }
  }
</script>

{#snippet separator(text: string)}
  <div class="flex items-center gap-3 py-1 text-xs text-muted-foreground">
    <span class="h-px flex-1 bg-border"></span>
    <span class="min-w-0 text-center wrap-anywhere">{text}</span>
    <span class="h-px flex-1 bg-border"></span>
  </div>
{/snippet}

{#if entry.type === "message" && entry.role === "user"}
  <div class="flex justify-end pt-2">
    <div
      class="max-w-[85%] min-w-0 space-y-1.5 rounded-lg bg-muted px-4 py-2.5 text-sm leading-relaxed wrap-anywhere"
    >
      {#if entry.text}<p class="whitespace-pre-wrap">
          <span class="sr-only">{m.s3_timeline_you_sr()}</span
          >{#each sessionReferenceParts(entry.text) as part, i (i)}{#if part.href}<a
                class="rounded-sm text-primary underline underline-offset-2 hover:opacity-80 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                href={part.href}>{part.text}</a
              >{:else}{part.text}{/if}{/each}
        </p>{/if}
      {#if entry.attachments?.length}
        <ul class="flex flex-wrap gap-1" aria-label={m.s3_attachments_label()}>
          {#each entry.attachments as attachment (attachment.path)}
            <li
              class="flex max-w-full min-w-0 items-center gap-1.5 rounded-sm border bg-background/60 px-1.5 py-1 text-xs"
            >
              {#if attachment.kind === "image"}
                <ChatAttachmentImage
                  {attachment}
                  {sessionId}
                  preview={preview?.(attachment.path)}
                />
              {:else}
                <IconFile class="size-3.5 shrink-0" aria-hidden="true" />
              {/if}
              <span class="min-w-0 truncate"
                >{attachment.name ?? attachment.path}</span
              >
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>
{:else if entry.type === "message" && entry.role === "agent"}
  <div class="min-w-0">
    <CodeMarkdownPreview
      text={entry.text}
      variant="message"
      onopenlink={onopenlocation}
    />
    {#if entry.truncated}<p class="text-xs text-muted-foreground">
        {m.s3_content_too_long_truncated()}
      </p>{/if}
  </div>
{:else if entry.type === "work"}
  <ToolGroup steps={entry.steps} {waiting} {onreview} {onopenlocation} />
{:else if entry.type === "compaction"}
  {#if entry.summary}
    <ChatDisclosure>
      {#snippet summary()}
        <span class="h-px flex-1 bg-border"></span>
        <span class="shrink-0 text-xs"
          >{entry.status === "failed"
            ? m.s3_compaction_failed()
            : m.s3_compaction_done()}</span
        >
        <span class="h-px flex-1 bg-border"></span>
      {/snippet}
      <p class="wrap-anywhere whitespace-pre-wrap">{entry.summary}</p>
    </ChatDisclosure>
  {:else}
    {@render separator(
      entry.status === "running"
        ? m.s3_compaction_running()
        : entry.status === "failed"
          ? m.s3_compaction_failed()
          : m.s3_compaction_done(),
    )}
  {/if}
{:else if entry.type === "notice"}
  <InlineNotice
    tone={entry.level === "error"
      ? "danger"
      : entry.level === "warning"
        ? "warning"
        : "info"}>{entry.text}</InlineNotice
  >
{:else if entry.type === "turnEnd"}
  {@const end = turnEndInfo(entry.stopReason)}
  {#if entry.changes?.length}<TurnChanges changes={entry.changes} />{/if}
  {#if end?.error}
    <InlineNotice tone="danger"
      >{entry.error
        ? m.s3_turn_end_with_detail({ text: end.text, error: entry.error })
        : end.text}</InlineNotice
    >
  {:else if end}
    <p class="flex items-center gap-1.5 text-xs text-warning">
      <IconHandStop class="size-3.5 shrink-0" aria-hidden="true" />{end.text}
    </p>
  {/if}
{:else if entry.type === "marker"}
  {@render separator(
    entry.kind === "switched"
      ? entry.view === "terminal"
        ? m.s3_marker_switched_terminal()
        : m.s3_marker_switched_chat()
      : markerLabel(entry.kind),
  )}
{/if}
