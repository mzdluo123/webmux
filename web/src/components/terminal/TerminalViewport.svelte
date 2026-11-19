<script lang="ts">
  import type { Snippet } from "svelte";
  import IconCopy from "~icons/tabler/copy";
  import IconX from "~icons/tabler/x";
  import {
    Button,
    type ButtonSize,
    type ButtonVariant,
  } from "$lib/components/ui/button";
  import JumpToLatestButton from "../shared/JumpToLatestButton.svelte";
  import SoftKeys from "./SoftKeys.svelte";
  import { m } from "$lib/paraglide/messages.js";
  import type { Modifiers } from "../../domains/terminal/terminal-keys";
  import type { UploadProgress } from "../../hooks/shared/upload.svelte";
  import type { Connection } from "../../platform/ws-contracts";
  import { hasTouchInput } from "../../platform/touch-input";

  interface ViewportTerminal {
    readonly selection: string;
    readonly restoring: boolean;
    readonly atBottom: boolean;
    readonly connection: Connection;
    readonly controlling?: boolean;
    readonly exitCode?: number | null;
    readonly modifiers: Modifiers;
    readonly shiftLocked: boolean;
    copySelection(): void;
    clearSelection(): void;
    scrollToBottom(): void;
  }
  interface ViewportUploads {
    readonly dragging: boolean;
    readonly progress: UploadProgress | null;
  }

  // The xterm host with its overlays, above the soft-key toolbar.
  let {
    container = $bindable(),
    terminal: t,
    visible,
    sizeLocked = false,
    uploads,
    quickMenu,
    draftToggle,
    send,
    toggle,
    lock,
    paste,
    keyboard,
    findOpen = false,
    findBar,
    uploadProgress,
  }: {
    container?: HTMLDivElement;
    terminal: ViewportTerminal;
    // Whether the terminal pane is on screen (mobile may show code instead).
    visible: boolean;
    // Keeps the PTY grid fixed and lets the host scroll past its edges.
    sizeLocked?: boolean;
    uploads?: ViewportUploads;
    quickMenu: Snippet<["top" | "bottom", ButtonSize?, ButtonVariant?]>;
    // Phones keep the draft switch beside the quick commands.
    draftToggle: Snippet;
    send: (input: string) => void;
    toggle: (key: keyof Modifiers) => void;
    lock: (key: "shift") => void;
    paste: () => Promise<void>;
    keyboard: () => void;
    findOpen?: boolean;
    findBar?: Snippet;
    uploadProgress?: Snippet;
  } = $props();

  // Keep the hint off touch screens, including hybrids whose secondary pointer
  // is coarse, and mirror the platform's actual copy shortcut.
  const mac = /Mac/.test(navigator.platform);
  const shortcutHint =
    !hasTouchInput() && matchMedia("(pointer: fine)").matches;
</script>

<div class="terminal-column">
  <div class="terminal-area">
    <div
      class="terminal-host"
      class:size-locked={sizeLocked}
      class:restoring={t.restoring && t.connection !== "missing"}
      bind:this={container}
    ></div>
    {@render findBar?.()}
    {@render uploadProgress?.()}
    {#if uploads?.dragging}
      <div
        class="pointer-events-none absolute inset-0 z-20 flex items-center justify-center border-2 border-dashed border-primary bg-background/90"
        role="status"
      >
        <p class="text-sm">
          {uploads.progress ? m.s4_drop_wait_upload() : m.s4_drop_to_upload()}
        </p>
      </div>
    {/if}
    {#if visible}
      <div class="terminal-controls pointer-events-none absolute inset-0 z-10">
        {#if !t.atBottom && !t.restoring}
          <div
            class="terminal-jump pointer-events-auto absolute bottom-3 left-1/2 -translate-x-1/2"
          >
            <JumpToLatestButton
              labelClass="terminal-control-label"
              onclick={() => t.scrollToBottom()}
            />
          </div>
        {/if}
        {#if t.selection && !findOpen}
          <div
            class="terminal-selection pointer-events-auto absolute right-2 top-2 flex items-center gap-1 rounded-md border bg-popover p-1 shadow-md"
            role="toolbar"
            aria-label={m.s4_terminal_selection_aria()}
          >
            <Button
              size="sm"
              aria-label={m.s4_copy_selection()}
              title={m.s4_copy_selection()}
              aria-keyshortcuts={mac ? "Meta+C" : "Control+C"}
              onclick={() => t.copySelection()}
            >
              <IconCopy class="terminal-control-icon" aria-hidden="true" />
              <span class="terminal-control-label">{m.s4_copy_selection()}</span
              >
            </Button>
            {#if shortcutHint}<kbd
                class="terminal-control-label border px-1"
                aria-hidden="true">{mac ? "⌘C" : "Ctrl+C"}</kbd
              >{/if}
            <Button
              variant="ghost"
              size="sm"
              aria-label={m.s4_clear_selection()}
              title={m.s4_clear_selection()}
              onclick={() => t.clearSelection()}
            >
              <IconX class="terminal-control-icon" aria-hidden="true" />
              <span class="terminal-control-label"
                >{m.s4_clear_selection()}</span
              >
            </Button>
          </div>
        {/if}
      </div>
    {/if}
    {#if t.restoring && t.connection !== "missing"}<div
        class="restore-overlay"
        role="status"
      >
        {t.connection === "connecting"
          ? m.s4_connecting_ellipsis()
          : m.s4_restoring_ellipsis()}
      </div>{:else if t.connection === "connecting" || t.connection === "reconnecting"}<div
        class="pointer-events-none absolute top-2 right-2 z-20"
      >
        <span class="text-xs text-muted-foreground" role="status"
          >{m.s4_restoring_ellipsis()}</span
        >
      </div>{/if}
  </div>
  <SoftKeys
    {send}
    modifiers={t.modifiers}
    locked={t.shiftLocked}
    {toggle}
    {lock}
    paste={() => {
      void paste();
    }}
    {keyboard}
  >
    {#snippet trailing()}
      {@render draftToggle()}{@render quickMenu("top", "icon-xs", "ghost")}
    {/snippet}
  </SoftKeys>
</div>
