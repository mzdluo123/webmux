<script lang="ts">
  import SidebarButton from "../shell/SidebarButton.svelte";
  import IconAdjustmentsHorizontal from "~icons/tabler/adjustments-horizontal";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconLayout from "~icons/tabler/layout-columns";
  import IconMaximize from "~icons/tabler/maximize";
  import IconMinimize from "~icons/tabler/minimize";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { Separator } from "$lib/components/ui/separator";
  import { m } from "$lib/paraglide/messages.js";
  import SessionSwitcherTrigger from "../shell/SessionSwitcherTrigger.svelte";
  import SessionPaneMenu from "./SessionPaneMenu.svelte";
  import type DockWorkspace from "../shared/DockWorkspace.svelte";
  import { Button } from "$lib/components/ui/button";
  import IconFolder from "~icons/tabler/folder";
  import { navigate } from "../../platform/api";
  import { useFullscreen } from "../../hooks/shared/fullscreen.svelte";
  import { bottomToolbar } from "../../hooks/terminal/bottom-toolbar-preference.svelte";
  import { keyboardSizeHold } from "../../hooks/terminal/keyboard-size-hold-preference.svelte";
  let {
    dock,
    closePane,
    pickerOpen,
    openPanels,
    openFiles,
    fileLabel,
    onerror,
  }: {
    dock: DockWorkspace | undefined;
    closePane: (id: string) => void;
    pickerOpen: boolean;
    openPanels: () => void;
    openFiles?: () => void;
    fileLabel: string;
    onerror: (message: string) => void;
  } = $props();
  const pane = $derived(dock?.activeCommands());
  const fullscreen = useFullscreen((message) => onerror(message));
</script>

<header
  class="app-header flex h-(--app-header-height) items-center border-b bg-card sm:gap-2 sm:px-2"
>
  <!-- Phones open the sidebar drawer here (a home-screen app has no back
       button); wider screens return to the current mode's home. -->
  <SidebarButton class="sm:hidden" />
  <Button
    class="max-sm:hidden"
    variant="ghost"
    size="icon-sm"
    aria-label={m.s4_back_to_session_list()}
    title={m.s4_back_to_session_list()}
    onclick={() => navigate("/")}
  >
    <IconArrowLeft class="size-4" aria-hidden="true" />
  </Button>
  {#if pane && !pane.compact}
    {@render activeCommands()}
    <Separator
      orientation="vertical"
      class="mx-0.5 h-4 data-vertical:self-center max-sm:hidden"
    />
  {:else}
    <div class="flex-1"></div>
  {/if}
  <SessionSwitcherTrigger size="icon-touch-sm" class="sm:hidden" />
  {#if openFiles}<Button
      variant="ghost"
      size="icon-touch-sm"
      aria-label={fileLabel}
      title={fileLabel}
      onclick={openFiles}><IconFolder aria-hidden="true" /></Button
    >{/if}
  <Button
    variant="ghost"
    size="icon-touch-sm"
    aria-label={m.s4_open_panel()}
    title={m.s4_open_panel()}
    aria-expanded={pickerOpen}
    aria-haspopup="dialog"
    onclick={openPanels}
  >
    <IconLayout aria-hidden="true" />
  </Button>
  {#if pane?.compact}{@render activeCommands()}{/if}
  <SessionPaneMenu {dock} {closePane} />
  <Separator
    orientation="vertical"
    class="mx-0.5 h-4 data-vertical:self-center max-sm:hidden"
  />
  {#if fullscreen.offered}<Button
      variant="ghost"
      size="icon-sm"
      class="max-sm:hidden"
      aria-label={fullscreen.active
        ? m.s4_exit_fullscreen()
        : m.s4_enter_fullscreen()}
      title={fullscreen.active
        ? m.s4_exit_fullscreen()
        : m.s4_enter_fullscreen()}
      aria-pressed={fullscreen.active}
      aria-busy={fullscreen.busy}
      disabled={fullscreen.busy}
      onclick={() => {
        void fullscreen.toggle();
      }}
    >
      {#if fullscreen.active}<IconMinimize class="size-4" aria-hidden="true" />
      {:else}<IconMaximize class="size-4" aria-hidden="true" />{/if}
    </Button>{/if}
  <DropdownMenu.Root>
    <DropdownMenu.Trigger>
      {#snippet child({ props })}
        <Button
          {...props}
          size="icon-touch-sm"
          variant="ghost"
          aria-label={m.s4_menu_group_display()}
          title={m.s4_menu_group_display()}
        >
          <IconAdjustmentsHorizontal aria-hidden="true" />
        </Button>
      {/snippet}
    </DropdownMenu.Trigger>
    <DropdownMenu.Content align="end">
      {#if fullscreen.offered}
        <DropdownMenu.Item
          class="sm:hidden"
          disabled={fullscreen.busy}
          onSelect={() => void fullscreen.toggle()}
        >
          {fullscreen.active ? m.s4_exit_fullscreen() : m.s4_enter_fullscreen()}
        </DropdownMenu.Item>
      {/if}
      <DropdownMenu.CheckboxItem
        checked={keyboardSizeHold.enabled}
        onCheckedChange={(value) => (keyboardSizeHold.enabled = value === true)}
      >
        {m.s4_hold_size_on_keyboard()}
      </DropdownMenu.CheckboxItem>
      <DropdownMenu.CheckboxItem
        checked={bottomToolbar.shown}
        onCheckedChange={(value) => bottomToolbar.set(value === true)}
      >
        {m.s4_force_soft_keys()}
      </DropdownMenu.CheckboxItem>
    </DropdownMenu.Content>
  </DropdownMenu.Root>
</header>

{#snippet activeCommands()}
  {#if pane?.commands}
    <div
      class="flex min-w-0 flex-1 items-center justify-end max-sm:flex-none"
      data-dock-session-commands={pane.id}
    >
      {#key pane.id}{@render pane.commands()}{/key}
    </div>
  {/if}
{/snippet}

<style>
  @media (max-width: 639px) {
    .app-header {
      --control-touch: min(2.75rem, 44px);
      gap: min(0.375rem, 6px);
      padding-inline: min(0.25rem, 4px);
    }
  }
</style>
