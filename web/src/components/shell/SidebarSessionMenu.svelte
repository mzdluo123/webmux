<script lang="ts">
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import type { SessionInfo } from "../../contracts/schemas";
  import { sessionPins } from "../../domains/sessions/session-pins.svelte";
  import { findShell, shellUi } from "../../hooks/shared/shell.svelte";
  import { insertAtSelection } from "../../lib/text-insertion";
  import { sessionReference } from "../../domains/sessions/session-reference";
  import SidebarRowMenu from "./SidebarRowMenu.svelte";
  import * as m from "$lib/paraglide/messages.js";

  // The "⋯" button over the end of a sidebar session row and its menu: the
  // row's actions, the same in the chat history and the workbench tree.
  type Target =
    | {
        kind: "live";
        session: SessionInfo;
        onrename: (session: SessionInfo) => void;
        onclose: (session: SessionInfo) => void;
      }
    | {
        kind: "archived";
        onresume: () => void;
        /** Offered only while the record's worktree can be released. */
        onrelease?: () => void;
        /** Deletes only the record. */
        onremove: () => void;
        /**
         * Offered while the record's linked worktree can be removed; deletes
         * the worktree and this record after the worktree confirmation.
         */
        onremoveworktree?: () => void;
      };
  let {
    label,
    target,
    tabindex,
    disabled = false,
  }: {
    /** The row's title, naming the button. */
    label: string;
    target: Target;
    tabindex?: number;
    disabled?: boolean;
  } = $props();

  const shell = findShell();
  function reference(session: SessionInfo) {
    if (!shell?.activeChat || shell.activeChat.id === session.id) return;
    const draft = shell.drafts.get(shell.activeChat.id);
    if (draft.submission) return;
    const start = draft.selectionStart ?? draft.text.length;
    const end = draft.selectionEnd ?? start;
    const prefix = start && !/\s/.test(draft.text[start - 1] ?? "") ? " " : "";
    const suffix =
      end < draft.text.length && !/\s/.test(draft.text[end] ?? "") ? " " : "";
    const result = insertAtSelection(
      draft.text,
      start,
      end,
      prefix + sessionReference(session.title, session.id) + suffix,
    );
    shell.drafts.edit(draft, result.text, result.caret);
    shellUi.drawerOpen = false;
  }

  const live = $derived(shell?.sessions.sessions ?? []);
  const isArchiveClose = $derived(
    target.kind === "live" &&
      target.session.status !== "exited" &&
      target.session.views.includes("chat") &&
      target.session.state === "active",
  );
  const closeLabel = $derived.by(() => {
    if (target.kind !== "live") return "";
    if (target.session.status === "exited") return m.s1_remove_session();
    return isArchiveClose
      ? m.s1_stop_and_archive_session()
      : m.s1_close_session();
  });
</script>

<SidebarRowMenu
  ariaLabel={m.s1_session_actions({ label })}
  {tabindex}
  {disabled}
>
  {#if target.kind === "live"}
    {@const { session, onrename, onclose } = target}
    {#if shell?.activeChat && shell.activeChat.id !== session.id && session.status !== "exited"}
      <DropdownMenu.Item
        disabled={!!shell.drafts.get(shell.activeChat.id).submission}
        onSelect={() => reference(session)}
        >{m.s1_reference_in_chat()}</DropdownMenu.Item
      >
    {/if}
    {#if session.status !== "exited"}
      {@const pinned = sessionPins.has(session.id)}
      <DropdownMenu.Item
        disabled={!sessionPins.ready || (!pinned && sessionPins.full)}
        onSelect={() => sessionPins.toggle(session.id)}
      >
        {pinned ? m.s1_unpin_session() : m.s1_pin_session()}
      </DropdownMenu.Item>
      {#if pinned}
        <DropdownMenu.Item
          disabled={!sessionPins.ready ||
            !sessionPins.canMove(session.id, -1, live)}
          onSelect={() => sessionPins.move(session.id, -1, live)}
          >{m.s1_move_pin_up()}</DropdownMenu.Item
        >
        <DropdownMenu.Item
          disabled={!sessionPins.ready ||
            !sessionPins.canMove(session.id, 1, live)}
          onSelect={() => sessionPins.move(session.id, 1, live)}
          >{m.s1_move_pin_down()}</DropdownMenu.Item
        >
      {/if}
    {/if}
    <DropdownMenu.Item onSelect={() => onrename(session)}
      >{m.s1_edit_title()}</DropdownMenu.Item
    >
    <DropdownMenu.Separator />
    <DropdownMenu.Item
      variant={isArchiveClose ? "default" : "destructive"}
      onSelect={() => onclose(session)}>{closeLabel}</DropdownMenu.Item
    >
  {:else}
    <DropdownMenu.Item onSelect={target.onresume}
      >{m.s1_resume_session()}</DropdownMenu.Item
    >
    {#if target.onrelease}<DropdownMenu.Item onSelect={target.onrelease}
        >{m.s1_release_space()}</DropdownMenu.Item
      >{/if}
    <DropdownMenu.Separator />
    <DropdownMenu.Item variant="destructive" onSelect={target.onremove}
      >{m.s1_delete_record()}</DropdownMenu.Item
    >
    {#if target.onremoveworktree}<DropdownMenu.Item
        variant="destructive"
        onSelect={target.onremoveworktree}
        >{m.s1_delete_record_and_worktree()}</DropdownMenu.Item
      >{/if}
  {/if}
</SidebarRowMenu>
