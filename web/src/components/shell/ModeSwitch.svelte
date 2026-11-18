<script lang="ts">
  import IconLayoutDashboard from "~icons/tabler/layout-dashboard";
  import IconMessageCircle from "~icons/tabler/message-circle";
  import { sidebarModeIndicator } from "./sidebar-mode-indicator";
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
  class="relative flex flex-wrap items-center gap-x-1 gap-y-2"
  role="group"
  aria-label={m.s1_mode()}
>
  <Button
    variant="mode"
    size="sm-touch"
    class="min-w-[min(100%,6rem)] flex-auto"
    aria-label={m.s1_chat_mode()}
    data-sidebar-mode="chat"
    aria-pressed={mode === "chat"}
    title={m.s1_chat_mode_title()}
    onclick={() => onchange("chat")}
    ><IconMessageCircle aria-hidden="true" />{m.s1_page_chat()}</Button
  >
  <Button
    variant="mode"
    size="sm-touch"
    class="min-w-[min(100%,6rem)] flex-auto"
    aria-label={workspaces ? m.s1_workbench_mode() : m.s1_sessions_mode()}
    data-sidebar-mode="workbench"
    aria-pressed={mode === "workbench"}
    title={workspaces
      ? m.s1_workbench_mode_title()
      : m.s1_sessions_mode_title()}
    onclick={() => onchange("workbench")}
    ><IconLayoutDashboard aria-hidden="true" />{workspaces
      ? m.s1_workbench()
      : m.s1_sessions()}</Button
  >
  <span
    use:sidebarModeIndicator={mode}
    hidden
    aria-hidden="true"
    data-sidebar-mode-indicator
    class="pointer-events-none absolute top-0 left-0 h-0.5 w-(--mode-width) translate-x-(--mode-x) translate-y-(--mode-y) bg-foreground motion-safe:data-moving:transition-all motion-safe:data-moving:duration-200 motion-safe:data-moving:ease-out motion-reduce:transition-none"
  ></span>
</div>
