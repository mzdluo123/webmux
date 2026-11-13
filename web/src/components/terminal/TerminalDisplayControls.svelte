<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
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
    TV 模式
  </DropdownMenu.CheckboxItem>
  <DropdownMenu.CheckboxItem
    checked={headsetMode.enabled}
    onCheckedChange={(value) => (headsetMode.enabled = value === true)}
  >
    头显浏览器模式
  </DropdownMenu.CheckboxItem>
  {#if xr}
    <DropdownMenu.Item
      class={tvMode.enabled || (!xr.supported && !xr.active)
        ? undefined
        : "sm:hidden"}
      disabled={xr.busy || fullscreen.busy}
      onSelect={() => (xr?.active ? xr.exit() : xr?.enter())}
    >
      {xr.active ? "退出 VR" : "进入 VR"}
    </DropdownMenu.Item>
  {/if}
{:else}
  {#if xr && !tvMode.enabled && (xr.supported || xr.active)}
    <Button
      variant="outline"
      size="sm"
      class="max-sm:hidden"
      aria-label={xr.active ? "退出 VR" : "进入 VR"}
      title={xr.active ? "退出沉浸式 VR" : "进入沉浸式 VR 工作台"}
      aria-pressed={xr.active}
      disabled={xr.busy || fullscreen.busy}
      onclick={() => (xr?.active ? xr.exit() : xr?.enter())}
    >
      {xr.active ? "退出 VR" : "进入 VR"}
    </Button>
  {/if}
  {#if headsetMode.enabled && keyboard}
    <Button variant="outline" size="sm" onclick={keyboard}>键盘</Button>
  {/if}
{/if}
