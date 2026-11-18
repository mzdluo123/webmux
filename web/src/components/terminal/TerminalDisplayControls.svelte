<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { m } from "$lib/paraglide/messages.js";
  import { tvMode } from "../../hooks/shared/tv-mode.svelte";
  import { headsetMode } from "../../hooks/shared/headset-mode.svelte";

  let {
    menu = false,
    keyboard,
  }: {
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
{:else if headsetMode.enabled && keyboard}
  <Button variant="outline" size="sm" onclick={keyboard}
    >{m.s4_keyboard_short()}</Button
  >
{/if}
