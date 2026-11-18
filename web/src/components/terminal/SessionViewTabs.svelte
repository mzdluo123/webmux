<script lang="ts">
  import IconMessageCircle from "~icons/tabler/message-circle";
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import * as Tabs from "$lib/components/ui/tabs";
  import { m } from "$lib/paraglide/messages.js";
  import type { SwitchTarget } from "../../domains/sessions/hybrid";

  let {
    hybrid,
  }: {
    hybrid?: {
      side: SwitchTarget | null;
      /** Where a running switch goes. */
      target: SwitchTarget | null;
      switching: boolean;
      chatUnavailable: string;
      request: (to: SwitchTarget) => void;
    };
  } = $props();
  const shown = $derived(hybrid?.target ?? hybrid?.side ?? "");
  const unavailable = $derived(hybrid?.chatUnavailable ?? "");
  const switching = $derived(!!hybrid?.switching);
</script>

{#if hybrid}
  <Tabs.Root
    activationMode="manual"
    bind:value={
      () => shown,
      (to) => {
        if ((to === "chat" || to === "terminal") && to !== shown)
          hybrid.request(to);
      }
    }
  >
    <Tabs.List variant="line" animated aria-label={m.s4_session_view_tabs()}>
      <Tabs.Trigger
        value="chat"
        disabled={switching}
        aria-disabled={unavailable && !switching ? "true" : undefined}
        aria-label={unavailable
          ? m.s4_tab_chat_unavailable_aria({ reason: unavailable })
          : m.s4_tab_chat()}
        title={unavailable
          ? m.s4_tab_chat_unavailable_title({ reason: unavailable })
          : m.s4_tab_chat()}
        ><IconMessageCircle aria-hidden="true" /><span>{m.s4_tab_chat()}</span
        ></Tabs.Trigger
      >
      <Tabs.Trigger
        value="terminal"
        disabled={switching}
        aria-label={m.s4_tab_terminal()}
        title={m.s4_tab_terminal()}
        ><IconTerminal2 aria-hidden="true" /><span>{m.s4_tab_terminal()}</span
        ></Tabs.Trigger
      >
    </Tabs.List>
  </Tabs.Root>
{/if}
