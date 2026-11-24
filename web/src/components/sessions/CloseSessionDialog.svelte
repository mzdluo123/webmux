<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { api, APIError } from "../../platform/api";
  import { lookupSessionRecord } from "../../domains/sessions/recoverable";
  import { NoContentSchema, type SessionInfo } from "../../contracts/schemas";
  import ConfirmDialog from "../shared/ConfirmDialog.svelte";

  // Ends a session. A live hybrid session is archived instead: its agent
  // stops, the conversation stays with the agent and the record can be
  // resumed from its worktree's archived section. A plain terminal closed
  // by the user disappears instead of lingering as an exited record.
  let {
    session,
    onclose,
    onremoved,
    onarchived,
  }: {
    session: SessionInfo;
    onclose: () => void;
    onremoved?: () => void;
    onarchived?: (session: SessionInfo) => void;
  } = $props();
  let open = $state(true);
  const removing = $derived(session.status === "exited");
  const plain = $derived(!removing && !session.views.includes("chat"));
  const archiving = $derived(
    !removing && session.views.includes("chat") && session.state === "active",
  );

  // A closed shell reports its exit a moment after the request; removal is
  // refused (409) until then.
  const REMOVE_ATTEMPTS = 25;
  const REMOVE_INTERVAL_MS = 200;

  async function removeWhenExited(id: string) {
    for (let attempt = 1; ; attempt++) {
      try {
        await api(`/sessions/${id}?remove=1`, NoContentSchema, "DELETE");
        return;
      } catch (err) {
        if (err instanceof APIError && err.status === 404) return;
        const running = err instanceof APIError && err.status === 409;
        if (!running || attempt >= REMOVE_ATTEMPTS) throw err;
        await new Promise((resolve) => setTimeout(resolve, REMOVE_INTERVAL_MS));
      }
    }
  }

  async function submit() {
    // Snapshot reactive props before awaiting network I/O so live state updates
    // arriving while DELETE is in flight cannot skip onarchived/onremoved.
    const target = session;
    const isRemoving = removing;
    const isPlain = plain;
    const isArchiving = archiving;
    try {
      await api(
        `/sessions/${target.id}${isRemoving ? "?remove=1" : ""}`,
        NoContentSchema,
        "DELETE",
      );
    } catch (err) {
      if (err instanceof APIError && err.status === 404) {
        if (isRemoving || isPlain) {
          onremoved?.();
          return;
        }
        if (isArchiving) {
          const state = await lookupSessionRecord(target.id).catch(() => null);
          if (state?.kind === "record") {
            onarchived?.(target);
            return;
          }
        }
      }
      throw err;
    }
    // A plain terminal closed on purpose is not kept as an exited record.
    if (isPlain) await removeWhenExited(target.id);
    if (isRemoving || isPlain) onremoved?.();
    // The user ended it on purpose: a page showing it leaves rather than
    // offering to resume or showing an exit banner.
    if (isArchiving) onarchived?.(target);
  }
</script>

<ConfirmDialog
  bind:open={
    () => open,
    (value) => {
      open = value;
      if (!value) onclose();
    }
  }
  title={removing
    ? m.s3_remove_session_title({ title: session.title })
    : archiving
      ? m.s3_archive_session_title({ title: session.title })
      : m.s3_close_session_title({ title: session.title })}
  confirmLabel={removing
    ? m.s3_confirm_remove()
    : archiving
      ? m.s3_stop_and_archive()
      : m.s3_confirm_close()}
  destructive={!archiving}
  onconfirm={submit}
>
  {removing
    ? m.s3_remove_session_body()
    : archiving
      ? m.s3_archive_session_body()
      : m.s3_close_session_body()}
</ConfirmDialog>
