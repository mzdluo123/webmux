<script lang="ts">
  import IconCheck from "~icons/tabler/check";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconCircleDashed from "~icons/tabler/circle-dashed";
  import IconLoader2 from "~icons/tabler/loader-2";
  import IconX from "~icons/tabler/x";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import type { ToolCall } from "./fixtures";

  // A Codex-style one-line tool call: status, verb and target, diff size.
  // Expanding shows the raw title and output. Keyed by `toolCallId` so
  // `tool_call_update` can upsert it in place.
  let { call }: { call: ToolCall } = $props();
  const verbs = {
    read: "已读取",
    search: "已搜索",
    edit: "已编辑",
    execute: "运行",
    other: "调用",
  };
  const running = {
    read: "正在读取",
    search: "正在搜索",
    edit: "正在编辑",
    execute: "正在运行",
    other: "正在调用",
  };
  const verb = $derived(
    call.status === "in_progress"
      ? running[call.kind]
      : call.status === "pending"
        ? "等待批准"
        : verbs[call.kind],
  );
  const target = $derived(
    call.diff ? call.diff.path.split("/").at(-1) : call.title,
  );
</script>

<Collapsible.Root data-tool-call={call.toolCallId}>
  <Collapsible.Trigger>
    {#snippet child({ props })}
      <ListButton
        {...props}
        variant="row"
        class="group/tool flex w-full min-w-0 flex-wrap items-center text-sm text-muted-foreground hover:text-foreground"
      >
        {#if call.status === "completed"}
          <IconCheck
            class="size-3.5 shrink-0 text-success"
            aria-hidden="true"
          />
        {:else if call.status === "in_progress"}
          <IconLoader2
            class="size-3.5 shrink-0 animate-spin text-info"
            aria-hidden="true"
          />
        {:else if call.status === "failed"}
          <IconX
            class="size-3.5 shrink-0 text-destructive"
            aria-hidden="true"
          />
        {:else}
          <IconCircleDashed
            class="size-3.5 shrink-0 text-warning"
            aria-hidden="true"
          />
        {/if}
        <span class="shrink-0">{verb}</span>
        <span
          class={[
            "min-w-0 truncate text-foreground",
            call.kind === "execute" && "font-mono text-xs",
          ]}>{target}</span
        >
        {#if call.diff}
          <span class="shrink-0 font-mono text-xs tabular-nums">
            <span class="text-success">+{call.diff.added}</span>
            <span class="text-destructive">−{call.diff.removed}</span>
          </span>
        {/if}
        {#if call.status === "failed"}<span class="shrink-0 text-destructive"
            >已中断</span
          >{/if}
        <IconChevronRight
          class="size-3.5 shrink-0 opacity-0 transition group-hover/tool:opacity-100 group-aria-expanded/tool:rotate-90 group-aria-expanded/tool:opacity-100"
          aria-hidden="true"
        />
      </ListButton>
    {/snippet}
  </Collapsible.Trigger>
  <Collapsible.Content>
    <pre
      class="mt-1 ml-5 overflow-x-auto rounded-md bg-muted px-3 py-2 font-mono text-xs whitespace-pre-wrap text-muted-foreground">{call.title}{call.output
        ? `\n\n${call.output}`
        : ""}</pre>
  </Collapsible.Content>
</Collapsible.Root>
