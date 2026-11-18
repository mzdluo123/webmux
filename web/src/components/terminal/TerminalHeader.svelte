<script lang="ts">
  import SidebarButton from "../shell/SidebarButton.svelte";
  import { findShell, shellUi } from "../../hooks/shared/shell.svelte";
  import { tick, type Snippet } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconLayout from "~icons/tabler/layout-columns";
  import IconArrowsExchange from "~icons/tabler/arrows-exchange";
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
  import { findSessionSwitcher } from "../../hooks/shared/session-switcher";
  import SessionTitle from "./SessionTitle.svelte";

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
    codeShown,
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
    pickerOpen = false,
    chatActions,
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
    quickMenu: Snippet<["top" | "bottom", ButtonSize?, ButtonVariant?]>;
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
    pickerOpen?: boolean;
    chatActions?: Snippet;
  } = $props();
  // A rename reaches the session list first; the list title wins here so
  // the header follows it on either side.
  const shell = findShell();
  const session = $derived(
    shell?.sessions.sessions.find((session) => session.id === t.info?.id) ??
      t.info,
  );
  const title = $derived(session?.title || m.s4_tab_terminal());
  const phone = new MediaQuery("(max-width: 639px)");
  const openSwitcher = findSessionSwitcher();
  let menuTrigger = $state<HTMLButtonElement | null>(null);
  let fileInput = $state<{ open(): void }>();
  let skipMenuFocus = false;
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
  <SessionSwitcherTrigger size="icon-sm" class="max-sm:hidden" />
  <!-- The title and the session's state read as one unit; the actions keep
       their places while the title truncates. On phones the connection
       label sits below the centered title without resizing its row. -->
  <div class="flex min-w-0 flex-1 items-center gap-1 sm:gap-2">
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
      <SessionTitle {title} {session} agent={chatAgent?.title} />
      {#if !t.info || t.connection !== "connected"}<span
          class="pointer-events-none absolute inset-x-0 top-1/2 mt-2 truncate text-xs leading-none text-muted-foreground sm:hidden"
          role="status">{connectionText}</span
        >{/if}
    </div>
    {#if features.chat && chatAgent}<span
        class="inline-flex shrink-0 items-center gap-1 text-xs text-muted-foreground max-sm:hidden sm:order-2"
        role="img"
        aria-label={m.s4_agent_label({ title: chatAgent.title })}
        title={m.s4_agent_label({ title: chatAgent.title })}
        data-slot="chat-agent"
      >
        {#if features.chat.hasLaunchIcon(chatAgent.icon)}
          <features.chat.LaunchIcon icon={chatAgent.icon} size="sm" />
        {/if}
        <span>{chatAgent.title}</span>
      </span>{/if}
    {#if !t.info || t.connection !== "connected"}<span
        class="shrink-0 text-xs whitespace-nowrap text-muted-foreground max-sm:hidden sm:order-4"
        role="status">{connectionText}</span
      >{/if}
  </div>
  <!-- Actions, left to right: where the session runs, terminal actions
       (icons), the quick commands and the panes it opens (outlined), then
       the window and the menu. -->
  {#if features.chat && hybrid && hybrid.view !== "recovery" && !phone.current}
    <features.chat.ViewTabs
      hybrid={{
        side: hybrid.side,
        target: hybrid.target,
        switching: hybrid.view === "switching",
        chatUnavailable: hybrid.chatUnavailable,
        request: (to) => hybrid.request(to),
      }}
    />
  {/if}
  {#if terminalShown && upload}
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
  {#if terminalShown && !codeShown}
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
      {#snippet child({ props })}<Button
          {...props}
          bind:ref={menuTrigger}
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
        if (skipMenuFocus) {
          event.preventDefault();
          skipMenuFocus = false;
        }
      }}
    >
      {#if phone.current && features.chat && hybrid && hybrid.view !== "recovery"}
        <DropdownMenu.RadioGroup
          value={hybrid.target ?? hybrid.side ?? ""}
          aria-label={m.s4_input_side()}
          onValueChange={(to) => {
            if (to === "chat" || to === "terminal") hybrid.request(to);
          }}
        >
          <DropdownMenu.GroupHeading
            >{m.s4_input_side()}</DropdownMenu.GroupHeading
          >
          <DropdownMenu.RadioItem
            value="chat"
            closeOnSelect
            disabled={hybrid.view === "switching" || !!hybrid.chatUnavailable}
            >{m.s4_tab_chat()}</DropdownMenu.RadioItem
          >
          {#if hybrid.chatUnavailable}
            <DropdownMenu.Label class="wrap-anywhere">
              {hybrid.chatUnavailable}
            </DropdownMenu.Label>
          {/if}
          <DropdownMenu.RadioItem
            value="terminal"
            closeOnSelect
            disabled={hybrid.view === "switching"}
            >{m.s4_tab_terminal()}</DropdownMenu.RadioItem
          >
        </DropdownMenu.RadioGroup>
        <DropdownMenu.Separator />
      {/if}
      {#if phone.current && openSwitcher}
        <DropdownMenu.Item
          onSelect={() => {
            skipMenuFocus = true;
            void tick().then(() => openSwitcher(menuTrigger));
          }}
        >
          <IconArrowsExchange aria-hidden="true" />
          {m.s1_quick_switch_session()}
        </DropdownMenu.Item>
      {/if}
      {#if fullscreen.offered}<DropdownMenu.Item
          class="sm:hidden"
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
            class="sm:hidden"
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
              skipMenuFocus = true;
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
              class="sm:hidden"
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
