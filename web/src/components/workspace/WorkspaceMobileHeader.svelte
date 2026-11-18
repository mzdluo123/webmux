<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import SidebarButton from "../shell/SidebarButton.svelte";
  import SessionSwitcherTrigger from "../shell/SessionSwitcherTrigger.svelte";

  let { reconnecting }: { reconnecting: boolean } = $props();
</script>

<!-- Phones reach the sidebar here; wider screens already show the sidebar. -->
<header
  class="app-header flex min-h-(--app-header-height) flex-wrap items-center justify-between gap-y-1 border-b bg-card px-1 sm:hidden"
>
  <SidebarButton />
  {#if reconnecting}
    <span role="status" class="order-last w-full px-2 pb-1 text-xs text-warning"
      >{m.s8_reconnecting_banner()}</span
    >
  {/if}
  <SessionSwitcherTrigger size="icon-touch" />
</header>
