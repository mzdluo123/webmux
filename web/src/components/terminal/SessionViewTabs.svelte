<script lang="ts">
  import IconMessageCircle from "~icons/tabler/message-circle";
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import * as Tabs from "$lib/components/ui/tabs";
  import { m } from "$lib/paraglide/messages.js";
  import { MediaQuery } from "svelte/reactivity";
  import type { SwitchTarget } from "../../domains/sessions/hybrid";

  // Which side a hybrid session is on, as one tab control: "聊天 | 终端".
  // An ordinary session has no sides and shows nothing. The code panel has
  // its own header button; while it covers a phone's screen no side is
  // selected, and choosing either closes it. A side that cannot be reached
  // now stays focusable with its reason; choosing it explains instead of
  // switching. While a switch runs or its target is unknown, the tabs keep
  // their places and none is selected.
  let {
    hybrid,
    code,
  }: {
    /** A hybrid session's sides; absent for an ordinary session. */
    hybrid?: {
      side: SwitchTarget | null;
      /** Where a running switch goes. */
      target: SwitchTarget | null;
      switching: boolean;
      chatUnavailable: string;
      request: (to: SwitchTarget) => void;
    };
    code?: { shown: boolean; set: (shown: boolean) => void };
  } = $props();
  const phone = new MediaQuery("(max-width: 639px)");
  const inCode = $derived(phone.current && code?.shown);
  const shown = $derived(hybrid?.target ?? hybrid?.side ?? "");
  const unavailable = $derived(hybrid?.chatUnavailable ?? "");
  const switching = $derived(!!hybrid?.switching);
</script>

{#if hybrid}
  <Tabs.Root
    activationMode="manual"
    bind:value={
      () => (inCode ? "" : shown),
      (to) => {
        if (inCode) code?.set(false);
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
        ><IconMessageCircle
          class={phone.current ? "hidden" : ""}
          aria-hidden="true"
        /><span>{m.s4_tab_chat()}</span></Tabs.Trigger
      >
      <Tabs.Trigger
        value="terminal"
        disabled={switching}
        aria-label={m.s4_tab_terminal()}
        title={m.s4_tab_terminal()}
        ><IconTerminal2
          class={phone.current ? "hidden" : ""}
          aria-hidden="true"
        /><span>{m.s4_tab_terminal()}</span></Tabs.Trigger
      >
    </Tabs.List>
  </Tabs.Root>
{/if}
