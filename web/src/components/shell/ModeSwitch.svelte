<script lang="ts">
  import IconLayoutDashboard from "~icons/tabler/layout-dashboard";
  import IconMessageCircle from "~icons/tabler/message-circle";
  import { Button } from "$lib/components/ui/button";
  import * as m from "$lib/paraglide/messages.js";
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

<div class="flex shrink-0 items-center" role="group" aria-label={m.s1_mode()}>
  <Button
    variant="ghost"
    size="icon-touch"
    aria-label={m.s1_chat_mode()}
    aria-pressed={mode === "chat"}
    title={m.s1_chat_mode_title()}
    onclick={() => onchange("chat")}
    ><IconMessageCircle aria-hidden="true" /></Button
  >
  <Button
    variant="ghost"
    size="icon-touch"
    aria-label={workspaces ? m.s1_workbench_mode() : m.s1_sessions_mode()}
    aria-pressed={mode === "workbench"}
    title={workspaces
      ? m.s1_workbench_mode_title()
      : m.s1_sessions_mode_title()}
    onclick={() => onchange("workbench")}
    ><IconLayoutDashboard aria-hidden="true" /></Button
  >
</div>
