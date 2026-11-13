<script lang="ts">
  import TerminalDisplayControls from "../../src/components/terminal/TerminalDisplayControls.svelte";
  import * as DropdownMenu from "../../src/lib/components/ui/dropdown-menu";
  import type { XrHeaderControl } from "../../src/domains/terminal/xr-workspace";
  let { xr }: { xr: XrHeaderControl } = $props();
  const fullscreen = {
    offered: true,
    active: false,
    busy: false,
    async toggle() {},
  };
</script>

<TerminalDisplayControls {xr} {fullscreen} />
<DropdownMenu.Root>
  <DropdownMenu.Trigger>显示菜单</DropdownMenu.Trigger>
  <DropdownMenu.Content>
    <TerminalDisplayControls {xr} {fullscreen} menu />
  </DropdownMenu.Content>
</DropdownMenu.Root>
