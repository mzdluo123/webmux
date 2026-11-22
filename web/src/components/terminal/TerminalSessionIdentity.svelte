<script lang="ts">
  import type { ComponentProps } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import type { SessionInfo } from "../../contracts/schemas";
  import type { TerminalFeatures } from "../../features/terminal";
  import { findShell } from "../../hooks/shared/shell.svelte";
  import SessionTitle from "./SessionTitle.svelte";
  import TerminalPaneStatus from "./TerminalPaneStatus.svelte";

  let {
    terminal,
    compact = false,
    chat,
    agent,
  }: {
    terminal: ComponentProps<typeof TerminalPaneStatus>["terminal"];
    compact?: boolean;
    chat?: TerminalFeatures["chat"];
    agent?: SessionInfo["agent"];
  } = $props();
  const shell = findShell();
  const session = $derived(
    shell?.sessions.sessions.find((entry) => entry.id === terminal.info?.id) ??
      terminal.info,
  );
  const title = $derived(session?.title || m.s4_tab_terminal());
</script>

<TerminalPaneStatus {terminal} {compact}>
  {#if compact}
    <SessionTitle {title} {session} agent={agent?.title} />
  {:else}
    <div class="flex min-w-0 items-center gap-1.5">
      <div class="flex min-w-0 flex-col">
        <SessionTitle {title} {session} agent={agent?.title} />
      </div>
      {#if chat && agent}
        <span
          class="hidden shrink-0 text-xs text-muted-foreground/60 md:inline"
          aria-hidden="true">·</span
        >
        <span
          class="hidden shrink-0 items-center gap-1 text-xs text-muted-foreground md:inline-flex"
          role="img"
          aria-label={m.s4_agent_label({ title: agent.title })}
          title={m.s4_agent_label({ title: agent.title })}
          data-slot="chat-agent"
        >
          {#if chat.hasLaunchIcon(agent.icon)}
            <chat.LaunchIcon icon={agent.icon} size="sm" />
            <span class="max-lg:hidden">{agent.title}</span>
          {:else}
            <span>{agent.title}</span>
          {/if}
        </span>
      {/if}
    </div>
  {/if}
</TerminalPaneStatus>
