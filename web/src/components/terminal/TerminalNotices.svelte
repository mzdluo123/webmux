<script lang="ts">
  import type { Snippet } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import * as m from "$lib/paraglide/messages.js";
  import IconX from "~icons/tabler/x";
  import type { HybridNotice } from "../../hooks/sessions/hybrid.svelte";
  import type { UploadProgress } from "../../hooks/shared/upload.svelte";
  import type { TerminalPlugins } from "../../plugins/terminal";
  import StripNotice from "../shared/StripNotice.svelte";
  import type { ControlState, ListeningPort } from "../../contracts/schemas";
  import type { TerminalDraft } from "../../domains/terminal/terminal-draft.svelte";
  import type { Connection } from "../../platform/ws";

  interface NoticeTerminal {
    readonly control: ControlState | undefined;
    readonly controlling: boolean;
    readonly connection: Connection;
    readonly exitCode: number | null;
    readonly error: string;
    claim(): void;
  }
  interface NoticeUploads {
    readonly progress: UploadProgress | null;
    cancel(): void;
  }

  // Status strips between the header and the terminal, in display order.
  let {
    terminal: t,
    plugins = {},
    uploads,
    draft,
    draftSaved,
    error,
    pendingCopy = false,
    copyPending = () => {},
    dismissCopy = () => {},
    notice = $bindable(),
    showScratch,
    dismissScratch,
    remove,
    notices = [],
    dismiss = () => {},
    recovery,
    session = "",
    listening,
    showForwards,
    leave,
  }: {
    terminal: NoticeTerminal;
    plugins?: TerminalPlugins;
    uploads?: NoticeUploads;
    draft: TerminalDraft;
    draftSaved: string;
    error: string;
    pendingCopy?: boolean;
    copyPending?: () => void;
    dismissCopy?: () => void;
    notice: string;
    showScratch: () => void;
    dismissScratch: () => void;
    remove: () => void;
    /** A hybrid session's switch and identity notices. */
    notices?: HybridNotice[];
    dismiss?: (key: string) => void;
    /** Resume choices that replace the missing and exited strips. */
    recovery?: Snippet;
    /** The session ID whose listening ports get forwarding prompts. */
    session?: string;
    /** Listening ports, or undefined while unknown (see TerminalPortPrompts). */
    listening?: readonly ListeningPort[];
    /** Opens the port-forwarding dialog. */
    showForwards?: () => void;
    /** Leaves this session: the page returns home, a split closes its pane. */
    leave: () => void;
  } = $props();
  const alertText = $derived([error, t.error].filter(Boolean).join(" "));
  let dismissedError = $state("");
  $effect(() => {
    if (dismissedError && dismissedError !== alertText) dismissedError = "";
  });
</script>

{#if !draft.enabled && !draft.scratchUpdatedWhileHidden && (draft.text || draft.updatedWhileHidden || draft.scratch.length)}
  <p class="sr-only" role="status">
    {#if draft.updatedWhileHidden}{m.s4_notice_hidden_draft_updated()}{:else if draft.text}{m.s4_notice_hidden_draft_kept()}{/if}
    {#if draft.scratch.length}
      {m.s4_notice_scratch_count_part({
        count: String(draft.scratch.length),
      })}{/if}
    {m.s4_notice_enable_draft_part({ saved: draftSaved })}
  </p>
{/if}
{#if !draft.enabled && draft.scratchUpdatedWhileHidden}
  <StripNotice ondismiss={dismissScratch}>
    {#if draft.updatedWhileHidden}{m.s4_notice_hidden_draft_updated()} ·
    {/if}{m.s4_notice_scratch_updated_strip({
      count: String(draft.scratch.length),
    })}
    {#snippet actions()}
      <Button variant="outline" size="sm" onclick={showScratch}
        >{m.s4_view_scratch()}</Button
      >
    {/snippet}
  </StripNotice>
{/if}
{#if plugins.files && uploads?.progress}<plugins.files.Progress
    progress={uploads.progress}
    cancel={() => uploads?.cancel()}
  />{/if}
{#if pendingCopy}
  <StripNotice label={m.s4_pending_copy_label()}>
    {m.s4_pending_copy_desc()}
    {#snippet actions()}
      <Button variant="outline" size="sm" onclick={copyPending}
        >{m.s4_copy_pending_text()}</Button
      >
      <Button variant="ghost" size="sm" onclick={dismissCopy}
        >{m.s4_ignore()}</Button
      >
    {/snippet}
  </StripNotice>
{/if}
{#if notice}<StripNotice ondismiss={() => (notice = "")}>{notice}</StripNotice
  >{/if}
{#each notices as item (item.key)}
  <StripNotice tone={item.tone} ondismiss={() => dismiss(item.key)}
    >{item.text}</StripNotice
  >
{/each}
{#if plugins.forward && session}<plugins.forward.PortPrompts
    {session}
    {listening}
    onshowall={showForwards}
  />{/if}
{#if t.control && !t.controlling && t.connection === "connected" && t.exitCode === null}
  <StripNotice label={m.s4_terminal_control_label()}>
    {t.control.controller
      ? m.s4_control_observing_controller({
          device: t.control.controller.label || m.s4_other_device(),
          cols: String(t.control.cols),
          rows: String(t.control.rows),
        })
      : m.s4_control_observing_none({
          cols: String(t.control.cols),
          rows: String(t.control.rows),
        })}
    {#snippet actions()}
      <Button variant="outline" size="sm" onclick={() => t.claim()}
        >{m.s4_claim_control()}</Button
      >
    {/snippet}
  </StripNotice>
{/if}
{#if alertText && alertText !== dismissedError}<StripNotice tone="danger">
    {alertText}
    {#snippet actions()}
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={m.s4_dismiss_notice()}
        title={m.s4_dismiss_notice()}
        onclick={() => (dismissedError = alertText)}
        ><IconX aria-hidden="true" /></Button
      >
    {/snippet}
  </StripNotice>{/if}
{#if recovery}{@render recovery()}{:else if t.connection === "missing"}<StripNotice
    tone="warning"
  >
    {m.s4_session_missing_strip()}
    {#snippet actions()}
      <Button variant="outline" size="sm" onclick={leave}
        >{m.s4_back_to_list()}</Button
      >
    {/snippet}
  </StripNotice>{/if}
{#if t.exitCode !== null && !recovery}
  <StripNotice>
    {m.s4_process_exited_strip({ code: String(t.exitCode) })}
    {#snippet actions()}
      <Button variant="outline" size="sm" class="shrink-0" onclick={leave}
        >{m.s4_back_to_list()}</Button
      >
      <Button variant="outline" size="sm" class="shrink-0" onclick={remove}
        >{m.s4_remove_session()}</Button
      >
    {/snippet}
  </StripNotice>
{/if}
