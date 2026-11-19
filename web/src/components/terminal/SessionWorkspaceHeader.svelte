<script lang="ts">
  import SidebarButton from "../shell/SidebarButton.svelte";
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconLayout from "~icons/tabler/layout-columns";
  import IconArrowsExchange from "~icons/tabler/arrows-exchange";
  import IconMaximize from "~icons/tabler/maximize";
  import IconMinimize from "~icons/tabler/minimize";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { m } from "$lib/paraglide/messages.js";
  import SessionSwitcherTrigger from "../shell/SessionSwitcherTrigger.svelte";
  import { findSessionSwitcher } from "../../hooks/shared/session-switcher";
  import { Button } from "$lib/components/ui/button";
  import IconFolder from "~icons/tabler/folder";
  import { tick } from "svelte";
  import { navigate } from "../../platform/api";
  import { shellUi } from "../../hooks/shared/shell.svelte";
  import { useFullscreen } from "../../hooks/shared/fullscreen.svelte";
  import { bottomToolbar } from "../../hooks/terminal/bottom-toolbar-preference.svelte";
  import { keyboardSizeHold } from "../../hooks/terminal/keyboard-size-hold-preference.svelte";
  import { terminalFont } from "../../hooks/terminal/terminal-font-preference.svelte";
  import TerminalFontDialog from "./TerminalFontDialog.svelte";
  let {
    pickerOpen,
    openPanels,
    openFiles,
    fileLabel,
    onerror,
  }: {
    pickerOpen: boolean;
    openPanels: () => void;
    openFiles?: () => void;
    fileLabel: string;
    onerror: (message: string) => void;
  } = $props();
  const phone = new MediaQuery("(max-width: 639px)");
  const openSwitcher = findSessionSwitcher();
  const fullscreen = useFullscreen((message) => onerror(message));
  let menuTrigger = $state<HTMLButtonElement | null>(null);
  let fontOpen = $state(false);
</script>

<header
  class="app-header flex h-(--app-header-height) items-center gap-2 bg-card px-1 sm:px-2"
>
  <!-- Phones open the sidebar drawer here (a home-screen app has no back
       button); wider screens return to the current mode's home. -->
  <SidebarButton class="sm:hidden" />
  <Button
    class="max-sm:hidden"
    variant="ghost"
    size="icon-sm"
    aria-label={shellUi.mode === "chat"
      ? m.s4_back_to_new_chat()
      : m.s4_back_to_session_list()}
    title={shellUi.mode === "chat"
      ? m.s4_back_to_new_chat()
      : m.s4_back_to_session_list()}
    onclick={() => navigate(shellUi.home)}
  >
    <IconArrowLeft class="size-4" aria-hidden="true" />
  </Button>
  <SessionSwitcherTrigger size="icon-sm" class="max-sm:hidden" />
  <div class="flex-1"></div>
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
    ><IconLayout class="size-4" aria-hidden="true" /></Button
  >
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
          bind:ref={menuTrigger}
          size="icon-touch-sm"
          variant="ghost"
          aria-label={m.s4_menu_group_display()}
          title={m.s4_menu_group_display()}>⋯</Button
        >
      {/snippet}
    </DropdownMenu.Trigger>
    <DropdownMenu.Content align="end">
      {#if phone.current && openSwitcher}
        <DropdownMenu.Item
          onSelect={() => {
            void tick().then(() => openSwitcher(menuTrigger));
          }}
        >
          <IconArrowsExchange aria-hidden="true" />
          {m.s1_quick_switch_session()}
        </DropdownMenu.Item>
      {/if}
      {#if fullscreen.offered}
        <DropdownMenu.Item
          class="sm:hidden"
          disabled={fullscreen.busy}
          onSelect={() => void fullscreen.toggle()}
        >
          {fullscreen.active ? m.s4_exit_fullscreen() : m.s4_enter_fullscreen()}
        </DropdownMenu.Item>
      {/if}
      <DropdownMenu.Item onSelect={() => (fontOpen = true)}
        >{m.s4_font_size()}</DropdownMenu.Item
      >
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
<TerminalFontDialog
  bind:open={fontOpen}
  fontSize={terminalFont.size}
  setFontSize={terminalFont.set}
  onCloseAutoFocus={(e) => {
    e.preventDefault();
    menuTrigger?.focus();
  }}
/>
