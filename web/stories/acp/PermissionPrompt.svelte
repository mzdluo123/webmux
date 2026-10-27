<script lang="ts">
  import IconTerminal from "~icons/tabler/terminal";
  import { ListButton } from "$lib/components/ui/list-button";
  import type { PermissionOption, ToolCall } from "./fixtures";

  // `session/request_permission`, laid out like Claude Code's prompt: the
  // request docks above the composer (where a phone thumb already is) and
  // lists the agent's own `options` as numbered choices.
  let {
    call,
    options,
    onresolve,
  }: {
    call: ToolCall;
    options: PermissionOption[];
    onresolve: (option: PermissionOption) => void;
  } = $props();
</script>

<section
  class="space-y-3 rounded-2xl border border-warning/40 bg-card p-3 shadow-sm"
  aria-label="等待授权"
>
  <div class="space-y-1.5">
    <p class="flex items-center gap-1.5 text-sm font-medium">
      <IconTerminal class="size-4 text-warning" aria-hidden="true" />运行命令
    </p>
    <pre
      class="rounded-lg bg-muted px-3 py-2 font-mono text-xs whitespace-pre-wrap break-all">{call.title}</pre>
    <p class="text-sm text-muted-foreground">是否允许 Agent 运行这个命令？</p>
  </div>
  <ol class="space-y-1">
    {#each options as option, i (option.optionId)}
      <li>
        <ListButton
          variant="option"
          class={[
            "flex w-full items-center gap-3 text-sm any-pointer-coarse:min-h-11",
            option.kind.startsWith("reject") && "text-destructive",
          ]}
          onclick={() => onresolve(option)}
        >
          <span
            class="flex size-5 shrink-0 items-center justify-center rounded border text-xs text-muted-foreground tabular-nums"
            >{i + 1}</span
          >
          {option.name}
        </ListButton>
      </li>
    {/each}
  </ol>
</section>
