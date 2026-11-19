<script lang="ts">
  import type { Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import type { SessionInfo } from "../../contracts/schemas";
  import { sessionStatusTone } from "../../domains/sessions/session-status";
  import { sessionStatusText } from "../../platform/api";
  import type { Connection } from "../../platform/ws-contracts";

  let {
    terminal,
    compact = false,
    children,
  }: {
    terminal: {
      readonly info: SessionInfo | undefined;
      readonly connection: Connection;
      readonly exitCode: number | null;
    };
    compact?: boolean;
    children: Snippet;
  } = $props();

  const showConnection = $derived(
    terminal.exitCode !== null ||
      !terminal.info ||
      terminal.connection !== "connected",
  );
  const connectionText = $derived(
    terminal.exitCode !== null
      ? m.s4_exit_status_code({ code: String(terminal.exitCode) })
      : terminal.connection === "connected"
        ? m.s4_conn_connected()
        : terminal.connection === "exited"
          ? m.s4_conn_exited()
          : terminal.connection === "missing"
            ? m.s4_conn_missing()
            : m.s4_conn_reconnecting(),
  );
</script>

<div class="flex min-w-0 flex-1 items-center gap-1">
  {#if terminal.info}
    {@const label =
      terminal.exitCode !== null
        ? connectionText
        : `${sessionStatusText(terminal.info)} · ${connectionText}`}
    <span
      class={`status-dot shrink-0 status-${terminal.info.status}`}
      class:status-failed={sessionStatusTone(terminal.info) === "destructive"}
      class:motion-safe:animate-pulse={terminal.connection === "connecting" ||
        terminal.connection === "reconnecting"}
      role="img"
      aria-label={label}
      title={label}
    ></span>
  {:else}
    <span class="status-dot invisible" aria-hidden="true"></span>
  {/if}
  {#if compact}
    <div class="relative flex min-w-0 flex-col">
      {@render children()}
      {#if showConnection}<span
          class="pointer-events-none absolute inset-x-0 top-1/2 mt-2 truncate text-xs leading-none text-muted-foreground"
          role="status">{connectionText}</span
        >{/if}
    </div>
  {:else}
    {@render children()}
    {#if showConnection}<span
        class="shrink-0 text-xs whitespace-nowrap text-muted-foreground"
        role="status">{connectionText}</span
      >{/if}
  {/if}
</div>
