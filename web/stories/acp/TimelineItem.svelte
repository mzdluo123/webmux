<script lang="ts">
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconHandStop from "~icons/tabler/hand-stop";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import InlineNotice from "../../src/components/InlineNotice.svelte";
  import ToolRow from "./ToolRow.svelte";
  import type { TimelineEntry } from "./fixtures";

  // One transcript entry in the Claude / Codex reading style: the user's
  // turn is a bubble, the agent's reply is plain prose and its work is a
  // column of quiet one-line rows.
  let { entry }: { entry: TimelineEntry } = $props();

  // Inline `code` only; the real view would reuse the US-I16 Markdown pipeline.
  function segments(text: string) {
    return text.split("`").map((part, i) => ({ part, code: i % 2 === 1 }));
  }
</script>

{#snippet disclosure(label: string, body: string[])}
  <Collapsible.Root>
    <Collapsible.Trigger>
      {#snippet child({ props })}
        <ListButton
          {...props}
          variant="row"
          class="group/disclosure flex items-center text-sm text-muted-foreground hover:text-foreground"
        >
          {label}
          <IconChevronRight
            class="size-3.5 transition-transform group-aria-expanded/disclosure:rotate-90"
            aria-hidden="true"
          />
        </ListButton>
      {/snippet}
    </Collapsible.Trigger>
    <Collapsible.Content>
      <div
        class="mt-1 ml-2 space-y-1 border-l-2 pl-3 text-sm leading-relaxed text-muted-foreground"
      >
        {#each body as line, i (i)}<p class="break-words">{line}</p>{/each}
      </div>
    </Collapsible.Content>
  </Collapsible.Root>
{/snippet}

{#if entry.type === "user"}
  <div class="flex justify-end pt-2">
    <p
      class="max-w-[85%] rounded-2xl bg-muted px-4 py-2.5 text-sm leading-relaxed break-words whitespace-pre-wrap"
    >
      <span class="sr-only">你：</span>{entry.text}
    </p>
  </div>
{:else if entry.type === "agent"}
  <p class="text-[0.9375rem] leading-7 break-words">
    {#each segments(entry.text) as { part, code }, i (i)}
      {#if code}<code
          class="rounded bg-muted px-1.5 py-0.5 font-mono text-[0.85em]"
          >{part}</code
        >{:else}{part}{/if}
    {/each}
  </p>
{:else if entry.type === "thought"}
  {@render disclosure(`已思考 ${entry.seconds} 秒`, [entry.text])}
{:else if entry.type === "reads"}
  {@render disclosure(
    `已浏览 ${entry.calls.length} 个文件`,
    entry.calls.map((call) => call.title),
  )}
{:else if entry.type === "tool"}
  <ToolRow call={entry.call} />
{:else if entry.type === "compaction"}
  <Collapsible.Root>
    <div class="flex items-center gap-3 py-1 text-xs text-muted-foreground">
      <span class="h-px flex-1 bg-border"></span>
      <Collapsible.Trigger>
        {#snippet child({ props })}
          <ListButton
            {...props}
            variant="row"
            class="group/compact flex items-center"
          >
            上下文已压缩 {entry.before} → {entry.after}
            <IconChevronRight
              class="size-3 transition-transform group-aria-expanded/compact:rotate-90"
              aria-hidden="true"
            />
          </ListButton>
        {/snippet}
      </Collapsible.Trigger>
      <span class="h-px flex-1 bg-border"></span>
    </div>
    <Collapsible.Content>
      <p
        class="mt-1 rounded-lg bg-muted px-3 py-2 text-sm leading-relaxed text-muted-foreground"
      >
        {entry.summary}
      </p>
    </Collapsible.Content>
  </Collapsible.Root>
{:else if entry.type === "notice"}
  <InlineNotice tone={entry.tone}>{entry.text}</InlineNotice>
{:else if entry.type === "turnEnd" && entry.stopReason === "cancelled"}
  <p class="flex items-center gap-1.5 text-xs text-warning">
    <IconHandStop class="size-3.5" aria-hidden="true" />已中断 ·
    已输出的内容保留
  </p>
{/if}
