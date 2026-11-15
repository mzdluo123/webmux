<script lang="ts">
  import IconSearch from "~icons/tabler/search";
  import { Button, type ButtonSize } from "$lib/components/ui/button";
  import * as m from "$lib/paraglide/messages.js";
  import {
    findSessionSwitcher,
    sessionSwitcherKeyLabel,
    sessionSwitcherKeys,
  } from "../../hooks/shared/session-switcher";

  let { size }: { size: ButtonSize } = $props();
  const open = findSessionSwitcher();
</script>

{#if open}
  <Button
    variant="ghost"
    {size}
    aria-label={m.s1_quick_switch_session()}
    aria-haspopup="dialog"
    aria-keyshortcuts={sessionSwitcherKeys}
    title={m.s1_quick_switch_session_shortcut({
      shortcut: sessionSwitcherKeyLabel,
    })}
    onclick={(event) => open(event.currentTarget)}
  >
    <IconSearch aria-hidden="true" />
  </Button>
{/if}
