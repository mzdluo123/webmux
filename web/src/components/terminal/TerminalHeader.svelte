<script lang="ts">
  import SidebarButton from "../shell/SidebarButton.svelte";
  import { findShell, shellUi } from "../../hooks/shared/shell.svelte";
  import type { Snippet } from "svelte";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconPlus from "~icons/tabler/plus";
  import IconMaximize from "~icons/tabler/maximize";
  import IconMinimize from "~icons/tabler/minimize";
  import IconUpload from "~icons/tabler/upload";
  import IconSearch from "~icons/tabler/search";
  import { sessionStatusText } from "../../platform/api";
  import {
    Button,
    type ButtonSize,
    type ButtonVariant,
  } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { m } from "$lib/paraglide/messages.js";
  import type { Fullscreen } from "../../hooks/shared/fullscreen.svelte";
  import type { HybridView } from "../../hooks/sessions/hybrid.svelte";
  import type { SwitchTarget } from "../../domains/sessions/hybrid";
  import type { SessionInfo } from "../../contracts/schemas";
  import { sessionStatusTone } from "../../domains/sessions/session-status";
  import type { TerminalDraft } from "../../domains/terminal/terminal-draft.svelte";
  import type { Connection } from "../../platform/ws-contracts";
  import DraftToggle from "./DraftToggle.svelte";
  import type { TerminalFeatures } from "../../features/terminal";
  import SessionSwitcherTrigger from "../shell/SessionSwitcherTrigger.svelte";
  import { tvMode } from "../../hooks/shared/tv-mode.svelte";
  import TerminalDisplayControls from "./TerminalDisplayControls.svelte";

  interface HeaderTerminal {
    readonly info: SessionInfo | undefined;
    readonly connection: Connection;
    readonly exitCode: number | null;
    readonly canFind: boolean;
    clear(): void;
    resync(): void;
  }
  // A hybrid session switches between its chat view and its terminal.
  interface HeaderHybrid {
    readonly view: HybridView;
    readonly side: SwitchTarget | null;
    readonly target: SwitchTarget | null;
    readonly chatUnavailable: string;
    request(to: SwitchTarget): void;
  }

  let {
    terminal: t,
    features = {},
    fullscreen,
    draft,
    draftSaved,
    scratchSaved,
    uploading,
    sizeLocked,
    setSizeLocked,
    keyboardSizeHold,
    setKeyboardSizeHold,
    codeShown = $bindable(),
    forceSoftKeys = $bindable(),
    quickMenu,
    upload,
    toggleDraft,
    showScratch,
    showFontSize,
    close,
    leave,
    showFind,
    hybrid,
    openPanels,
    chatActions,
    keyboard,
  }: {
    features?: TerminalFeatures;
    terminal: HeaderTerminal;
    fullscreen: Fullscreen;
    draft: TerminalDraft;
    draftSaved: string;
    scratchSaved: string;
    uploading: boolean;
    sizeLocked: boolean;
    setSizeLocked: (value: boolean) => void;
    keyboardSizeHold: boolean;
    setKeyboardSizeHold: (value: boolean) => void;
    codeShown: boolean;
    forceSoftKeys: boolean;
    quickMenu: Snippet<
      ["top" | "bottom", ButtonSize?, ButtonVariant?, boolean?]
    >;
    upload?: (files: File[]) => Promise<void>;
    toggleDraft: () => void;
    showScratch: () => void;
    showFontSize: () => void;
    close: () => void;
    /** Leaves this session: the page returns home, a split closes its pane. */
    leave: () => void;
    showFind: () => void;
    hybrid?: HeaderHybrid;
    openPanels: () => void;
    chatActions?: Snippet;
    keyboard?: () => void;
  } = $props();
  // A rename reaches the session list first; the list title wins here so
  // the header follows it on either side.
  const shell = findShell();
  const title = $derived(
    shell?.sessions.sessions.find((session) => session.id === t.info?.id)
      ?.title ||
      t.info?.title ||
      m.s4_tab_terminal(),
  );
  let fileInput = $state<{ open(): void }>();
  let panelTrigger = $state<HTMLButtonElement | null>(null);

  export function focusPanels() {
    panelTrigger?.focus();
  }
  let openingFind = false;
  // Off the terminal view there is no PTY: terminal-only actions are hidden;
  // fullscreen, the code panel and closing the session stay.
  const terminalShown = $derived(!hybrid || hybrid.view === "terminal");
  // The chat view names the agent it talks to beside the session title.
  const chatAgent = $derived(
    hybrid?.view === "chat" ? (t.info?.agent ?? undefined) : undefined,
  );
  const connectionText = $derived(
    t.connection === "connected"
      ? m.s4_conn_connected()
      : t.connection === "exited"
        ? m.s4_conn_exited()
        : t.connection === "missing"
          ? m.s4_conn_missing()
          : m.s4_conn_reconnecting(),
  );
  const uploadDisabled = $derived(
    uploading ||
      (!draft.enabled && t.connection !== "connected") ||
      t.connection === "missing" ||
      t.exitCode !== null,
  );
</script>

<header
  data-tv-terminal-toolbar
  class="app-header terminal-header flex h-(--app-header-height) shrink-0 items-center gap-2 border-b bg-card px-1 sm:px-2"
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
    onclick={leave}
  >
    <IconArrowLeft class="size-4" aria-hidden="true" />
  </Button>
  <SessionSwitcherTrigger size="icon-sm" />
  <!-- The title and the session's state read as one unit; the actions keep
       their places while the title truncates. Phones put the dot first and
       move a connection problem to a second line, so it never pushes the
       title out. -->
  <div class="flex min-w-8 flex-1 items-center gap-2">
    {#if t.info}<span
        class={`status-dot shrink-0 status-${t.info.status} sm:order-3`}
        class:status-failed={sessionStatusTone(t.info) === "destructive"}
        class:motion-safe:animate-pulse={t.connection !== "connected"}
        role="img"
        aria-label={`${sessionStatusText(t.info)} · ${connectionText}`}
        title={`${sessionStatusText(t.info)} · ${connectionText}`}

      ></span>{:else}<!-- Holds the dot's place on phones, where it leads the
           title, so the title does not shift when the session loads. -->
      <span class="shrink-0 sm:hidden" aria-hidden="true"
        ><span class="status-dot invisible"></span></span
      >{/if}
    <div class="relative flex min-w-0 flex-col sm:order-1">
      <span class="h-5 min-w-0 truncate text-sm leading-5 font-medium"
        >{title}</span
      >
      <!-- Overlaid under the title, so a status change never moves it. -->
      {#if !t.info || t.connection !== "connected"}<span
          class="absolute top-full left-0 -mt-0.5 max-w-full truncate text-xs leading-none text-muted-foreground sm:hidden"
          role="status">{connectionText}</span
        >{/if}
    </div>
    {#if features.chat && chatAgent}<span
        class="inline-flex shrink-0 items-center gap-1 text-xs text-muted-foreground sm:order-2"
        role="img"
        aria-label={m.s4_agent_label({ title: chatAgent.title })}
        title={m.s4_agent_label({ title: chatAgent.title })}
        data-slot="chat-agent"
        >{#if features.chat.hasLaunchIcon(chatAgent.icon)}<features.chat.LaunchIcon
            icon={chatAgent.icon}
            size="sm"
          /><span class="max-sm:hidden">{chatAgent.title}</span>{:else}<span
            >{chatAgent.title}</span
          >{/if}</span
      >{/if}
    {#if !t.info || t.connection !== "connected"}<span
        class="shrink-0 text-xs whitespace-nowrap text-muted-foreground max-sm:hidden sm:order-4"
        role="status">{connectionText}</span
      >{/if}
  </div>
  <!-- Actions, left to right: where the session runs, terminal actions
       (icons), the quick commands and the panes it opens (outlined), then
       the window and the menu. -->
  {#if features.chat}<features.chat.ViewTabs
      hybrid={hybrid && hybrid.view !== "recovery"
        ? {
            side: hybrid.side,
            target: hybrid.target,
            switching: hybrid.view === "switching",
            chatUnavailable: hybrid.chatUnavailable,
            request: (to) => hybrid.request(to),
          }
        : undefined}
      code={{ shown: codeShown, set: (value) => (codeShown = value) }}
    />{/if}
  {#if terminalShown && !tvMode.enabled && upload}
    <Button
      variant="ghost"
      size="icon-sm"
      class="max-sm:hidden"
      aria-label={m.s4_upload_files()}
      title={m.s4_upload_files()}
      disabled={uploadDisabled}
      onclick={() => fileInput?.open()}
      ><IconUpload class="size-4" aria-hidden="true" /></Button
    >
  {/if}
  {#if features.files && upload}<features.files.Input
      bind:this={fileInput}
      {upload}
    />{/if}
  {#if terminalShown && !tvMode.enabled}
    <div class="quick-commands-header flex items-center gap-1">
      {@render quickMenu("bottom", undefined, "ghost")}
    </div>
    <DraftToggle
      class="max-sm:hidden"
      {draft}
      {draftSaved}
      {scratchSaved}
      disabled={t.connection === "missing"}
      toggle={toggleDraft}
    />
  {/if}
  <Button
    bind:ref={panelTrigger}
    variant="ghost"
    size="icon-touch-sm"
    aria-label={m.s4_open_panel()}
    title={m.s4_open_panel()}
    aria-expanded={codeShown}
    onclick={openPanels}><IconPlus class="size-4" aria-hidden="true" /></Button
  >
  {#if fullscreen.offered && !tvMode.enabled}<Button
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
  <TerminalDisplayControls keyboard={terminalShown ? keyboard : undefined} />
  <DropdownMenu.Root>
    <DropdownMenu.Trigger>
      {#snippet child({ props })}<Button
          {...props}
          variant="ghost"
          size="icon-touch-sm"
          aria-label={terminalShown
            ? m.s4_terminal_menu()
            : m.s4_session_menu()}
          title={terminalShown ? m.s4_terminal_menu() : m.s4_session_menu()}
          >⋯</Button
        >{/snippet}
    </DropdownMenu.Trigger>
    <DropdownMenu.Content
      class={chatActions ? "min-w-48" : undefined}
      align="end"
      onCloseAutoFocus={(event) => {
        if (openingFind) {
          event.preventDefault();
          openingFind = false;
        }
      }}
    >
      <!-- Grouped by purpose. Below 640px the header keeps only the view
           switch and this menu, so the entries of the hidden buttons live
           here (marked sm:hidden). -->
      <TerminalDisplayControls menu />
      {#if tvMode.enabled && terminalShown}
        {@render quickMenu("bottom", "sm", "ghost", true)}
      {/if}
      <DropdownMenu.Separator />
      {#if fullscreen.offered}<DropdownMenu.Item
          class={tvMode.enabled ? undefined : "sm:hidden"}
          disabled={fullscreen.busy}
          onSelect={() => {
            void fullscreen.toggle();
          }}
          >{fullscreen.active
            ? m.s4_exit_fullscreen()
            : m.s4_enter_fullscreen()}</DropdownMenu.Item
        >{#if terminalShown}<DropdownMenu.Separator
            class="sm:hidden"
          />{/if}{/if}
      {#if chatActions}<DropdownMenu.Group
          ><DropdownMenu.GroupHeading
            >{m.s4_menu_group_handoff()}</DropdownMenu.GroupHeading
          >{@render chatActions()}</DropdownMenu.Group
        ><DropdownMenu.Separator />{/if}
      {#if terminalShown}
        <DropdownMenu.Group>
          <DropdownMenu.GroupHeading
            >{m.s4_menu_group_session()}</DropdownMenu.GroupHeading
          >
          <DropdownMenu.CheckboxItem
            class={tvMode.enabled ? undefined : "sm:hidden"}
            closeOnSelect
            checked={draft.enabled}
            disabled={t.connection === "missing"}
            onCheckedChange={toggleDraft}
            >{draft.scratch.length
              ? m.s4_menu_draft_with_scratch({
                  count: String(draft.scratch.length),
                })
              : m.s4_draft_mode()}</DropdownMenu.CheckboxItem
          >
          <DropdownMenu.Item
            disabled={!t.canFind}
            onSelect={() => {
              openingFind = true;
              showFind();
            }}><IconSearch aria-hidden="true" />{m.s4_find()}</DropdownMenu.Item
          >
          <DropdownMenu.Item onSelect={showScratch}
            >{m.s4_scratch_title()}{draft.scratch.length
              ? ` (${draft.scratch.length})`
              : ""}</DropdownMenu.Item
          >
          {#if upload}
            <DropdownMenu.Item
              class={tvMode.enabled ? undefined : "sm:hidden"}
              disabled={uploadDisabled}
              onSelect={() => fileInput?.open()}
              >{m.s4_upload_files()}</DropdownMenu.Item
            >
          {/if}
        </DropdownMenu.Group>
        <DropdownMenu.Separator />
        <DropdownMenu.Group>
          <DropdownMenu.GroupHeading
            >{m.s4_tab_terminal()}</DropdownMenu.GroupHeading
          >
          <DropdownMenu.Item onSelect={() => t.clear()}
            >{m.s4_clear_screen()}</DropdownMenu.Item
          >
          <DropdownMenu.Item onSelect={() => t.resync()}
            >{m.s4_resync()}</DropdownMenu.Item
          >
          <DropdownMenu.Item onSelect={showFontSize}
            >{m.s4_font_size()}</DropdownMenu.Item
          >
        </DropdownMenu.Group>
        <DropdownMenu.Separator />
        <DropdownMenu.Group>
          <DropdownMenu.GroupHeading
            >{m.s4_menu_group_display()}</DropdownMenu.GroupHeading
          >
          <DropdownMenu.CheckboxItem
            checked={sizeLocked}
            onCheckedChange={(value) => setSizeLocked(value === true)}
          >
            {m.s4_lock_size()}
          </DropdownMenu.CheckboxItem>
          <DropdownMenu.CheckboxItem
            checked={keyboardSizeHold}
            onCheckedChange={(value) => setKeyboardSizeHold(value === true)}
          >
            {m.s4_hold_size_on_keyboard()}
          </DropdownMenu.CheckboxItem>
          <DropdownMenu.CheckboxItem bind:checked={forceSoftKeys}>
            {m.s4_force_soft_keys()}
          </DropdownMenu.CheckboxItem>
        </DropdownMenu.Group>
      {/if}
      <DropdownMenu.Separator />
      {@const archivable =
        t.exitCode === null &&
        !!t.info?.views.includes("chat") &&
        t.info.state === "active"}
      <DropdownMenu.Item
        variant={archivable ? "default" : "destructive"}
        disabled={!t.info}
        onSelect={close}
        >{t.exitCode !== null
          ? m.s4_remove_session()
          : archivable
            ? m.s4_stop_and_archive_session()
            : m.s4_close_session()}</DropdownMenu.Item
      >
    </DropdownMenu.Content>
  </DropdownMenu.Root>
</header>
