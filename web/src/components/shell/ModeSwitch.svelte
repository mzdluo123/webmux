<script lang="ts">
  import IconLayoutDashboard from "~icons/tabler/layout-dashboard";
  import IconMessageCircle from "~icons/tabler/message-circle";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import type { ShellMode } from "../../hooks/shared/shell.svelte";

  // The two mode buttons at the top of the sidebar. Each keeps its variant
  // and only toggles aria-pressed; focus stays on the pressed button.
  let {
    mode,
    onchange,
    workspaces = true,
  }: {
    mode: ShellMode;
    onchange: (mode: ShellMode) => void;
    workspaces?: boolean;
  } = $props();
</script>

<div
  class="flex flex-wrap items-center gap-1"
  role="group"
  aria-label={m.s1_mode()}
>
  <Button
    variant="ghost"
    size="sm-touch"
    class="min-w-[min(100%,6rem)] flex-1"
    aria-label={m.s1_chat_mode()}
    aria-pressed={mode === "chat"}
    title={m.s1_chat_mode_title()}
    onclick={() => onchange("chat")}
    ><IconMessageCircle aria-hidden="true" />{m.s1_page_chat()}</Button
  >
  <Button
    variant="ghost"
    size="sm-touch"
    class="min-w-[min(100%,6rem)] flex-1"
    aria-label={workspaces ? m.s1_workbench_mode() : m.s1_sessions_mode()}
    aria-pressed={mode === "workbench"}
    title={workspaces
      ? m.s1_workbench_mode_title()
      : m.s1_sessions_mode_title()}
    onclick={() => onchange("workbench")}
    ><IconLayoutDashboard aria-hidden="true" />{workspaces
      ? m.s1_workbench()
      : m.s1_sessions()}</Button
  >
</div>
