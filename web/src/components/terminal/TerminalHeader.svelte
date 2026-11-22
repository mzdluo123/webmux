<script lang="ts">
  import { terminalFont } from "../../hooks/terminal/terminal-font-preference.svelte";
  import TerminalFontDialog from "./TerminalFontDialog.svelte";
  import { bottomToolbar } from "../../hooks/terminal/bottom-toolbar-preference.svelte";
  import { type Snippet } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  import IconDots from "~icons/tabler/dots";
  import IconSearch from "~icons/tabler/search";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import {
    Button,
    type ButtonSize,
    type ButtonVariant,
  } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { m } from "$lib/paraglide/messages.js";
  import type { HybridView } from "../../hooks/sessions/hybrid.svelte";
  import type { SwitchTarget } from "../../domains/sessions/hybrid";
  import type { SessionInfo } from "../../contracts/schemas";
  import type { TerminalDraft } from "../../domains/terminal/terminal-draft.svelte";
  import type { Connection } from "../../platform/ws-contracts";
  import type { TerminalFeatures } from "../../features/terminal";
  import TerminalSessionIdentity from "./TerminalSessionIdentity.svelte";

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
    draft,
    uploading,
    sizeLocked,
    setSizeLocked,
    codeShown,
    quickMenu,
    openUpload,
    toggleDraft,
    showScratch,
    close,
    showFind,
    hybrid,
    chatActions,
  }: {
    features?: TerminalFeatures;
    terminal: HeaderTerminal;
    draft: TerminalDraft;
    uploading: boolean;
    sizeLocked: boolean;
    setSizeLocked: (value: boolean) => void;
    codeShown: boolean;
    quickMenu: Snippet<["bottom" | "submenu", ButtonSize?, ButtonVariant?]>;
    openUpload?: () => void;
    toggleDraft: () => void;
    showScratch: () => void;
    close: () => void;
    showFind: () => void;
    hybrid?: HeaderHybrid;
    chatActions?: Snippet;
  } = $props();
  const phone = new MediaQuery("(max-width: 639px)");
  const compactActions = new MediaQuery("(width < 48rem)");
  let menuTrigger = $state<HTMLButtonElement | null>(null);
  let fontOpen = $state(false);
  let skipMenuFocus = false;
  const terminalShown = $derived(!hybrid || hybrid.view === "terminal");
  const showViewTabs = $derived(
    !!(
      features.chat &&
      hybrid &&
      hybrid.view !== "recovery" &&
      !compactActions.current
    ),
  );
  const menuLabel = $derived(
    terminalShown ? m.s4_terminal_menu() : m.s4_session_menu(),
  );
  // The chat view names the agent it talks to beside the session title.
  const chatAgent = $derived(
    hybrid?.view === "chat" ? (t.info?.agent ?? undefined) : undefined,
  );
  const showQuickCommands = $derived(
    terminalShown && !codeShown && !bottomToolbar.shown,
  );
  const uploadDisabled = $derived(
    uploading ||
      (!draft.enabled && t.connection !== "connected") ||
      t.connection === "missing" ||
      t.exitCode !== null,
  );
</script>

<div class="terminal-header flex min-w-0 flex-1 items-center gap-2">
  {#if !phone.current}
    <TerminalSessionIdentity
      terminal={t}
      chat={features.chat}
      agent={chatAgent}
    />
  {/if}
  <!-- Center: hybrid chat/terminal view switch. -->
  {#if showViewTabs && features.chat && hybrid}
    <div class="shrink-0">
      <features.chat.ViewTabs
        hybrid={{
          side: hybrid.side,
          target: hybrid.target,
          switching: hybrid.view === "switching",
          chatUnavailable: hybrid.chatUnavailable,
          request: (to) => hybrid.request(to),
        }}
      />
    </div>
  {/if}
  <!-- Right: session quick commands and session/terminal menu. -->
  <div
    class={[
      "flex shrink-0 items-center justify-end gap-1",
      showViewTabs && "lg:basis-0 lg:grow",
    ]}
  >
    {#if !phone.current && showQuickCommands}
      {@render quickMenu("bottom", "icon-touch-sm", "ghost")}
    {/if}
    {@render terminalMenu()}
  </div>
</div>

{#snippet terminalMenu()}
  <DropdownMenu.Root>
    <DropdownMenu.Trigger>
      {#snippet child({ props })}<Button
          {...props}
          variant="ghost"
          size={compactActions.current ? "icon-touch-sm" : "sm"}
          bind:ref={menuTrigger}
          aria-label={menuLabel}
          title={menuLabel}
        >
          {#if compactActions.current}<IconDots aria-hidden="true" />{:else}
            {terminalShown ? m.s4_tab_terminal() : m.s4_menu_group_session()}
            <IconChevronDown aria-hidden="true" />
          {/if}
        </Button>{/snippet}
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
      {#if compactActions.current && features.chat && hybrid && hybrid.view !== "recovery"}
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
      {#if phone.current && showQuickCommands}
        {@render quickMenu("submenu")}
      {/if}
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
          {#if openUpload}
            <DropdownMenu.Item disabled={uploadDisabled} onSelect={openUpload}
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
        </DropdownMenu.Group>
        <DropdownMenu.Separator />
        <DropdownMenu.Group>
          <DropdownMenu.GroupHeading
            >{m.s4_menu_group_display()}</DropdownMenu.GroupHeading
          >
          <DropdownMenu.Item
            onSelect={() => {
              skipMenuFocus = true;
              fontOpen = true;
            }}>{m.s4_font_size()}</DropdownMenu.Item
          >
          <DropdownMenu.CheckboxItem
            checked={sizeLocked}
            onCheckedChange={(value) => setSizeLocked(value === true)}
          >
            {m.s4_lock_size()}
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
{/snippet}

<TerminalFontDialog
  bind:open={fontOpen}
  fontSize={terminalFont.size}
  setFontSize={terminalFont.set}
  onCloseAutoFocus={(e) => {
    e.preventDefault();
    menuTrigger?.focus();
  }}
/>
