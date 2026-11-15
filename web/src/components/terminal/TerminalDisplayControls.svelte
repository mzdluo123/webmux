<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import * as m from "$lib/paraglide/messages.js";
  import { tvMode } from "../../hooks/shared/tv-mode.svelte";
  import { headsetMode } from "../../hooks/shared/headset-mode.svelte";
  import type { Fullscreen } from "../../hooks/shared/fullscreen.svelte";
  import type { XrHeaderControl } from "../../domains/terminal/xr-workspace";

  let {
    xr,
    fullscreen,
    menu = false,
    keyboard,
  }: {
    xr?: XrHeaderControl;
    fullscreen: Fullscreen;
    menu?: boolean;
    keyboard?: () => void;
  } = $props();
</script>

{#if menu}
  <DropdownMenu.CheckboxItem
    checked={tvMode.enabled}
    onCheckedChange={(value) => (tvMode.enabled = value === true)}
  >
    {m.s4_tv_mode()}
  </DropdownMenu.CheckboxItem>
  <DropdownMenu.CheckboxItem
    checked={headsetMode.enabled}
    onCheckedChange={(value) => (headsetMode.enabled = value === true)}
  >
    {m.s4_headset_browser_mode()}
  </DropdownMenu.CheckboxItem>
  {#if xr}
    <DropdownMenu.Item
      class={tvMode.enabled || (!xr.supported && !xr.active)
        ? undefined
        : "sm:hidden"}
      disabled={xr.busy || fullscreen.busy}
      onSelect={() => (xr?.active ? xr.exit() : xr?.enter())}
    >
      {xr.active ? m.s4_exit_vr() : m.s4_enter_vr()}
    </DropdownMenu.Item>
  {/if}
{:else}
  {#if xr && !tvMode.enabled && (xr.supported || xr.active)}
    <Button
      variant="outline"
      size="sm"
      class="max-sm:hidden"
      aria-label={xr.active ? m.s4_exit_vr() : m.s4_enter_vr()}
      title={xr.active ? m.s4_exit_vr_title() : m.s4_enter_vr_title()}
      aria-pressed={xr.active}
      disabled={xr.busy || fullscreen.busy}
      onclick={() => (xr?.active ? xr.exit() : xr?.enter())}
    >
      {xr.active ? m.s4_exit_vr() : m.s4_enter_vr()}
    </Button>
  {/if}
  {#if headsetMode.enabled && keyboard}
    <Button variant="outline" size="sm" onclick={keyboard}
      >{m.s4_keyboard_short()}</Button
    >
  {/if}
{/if}
