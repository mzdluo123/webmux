<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import { api, APIError } from "../../platform/api";
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

  async function removeWhenExited() {
    for (let attempt = 1; ; attempt++) {
      try {
        await api(
          `/sessions/${session.id}?remove=1`,
          NoContentSchema,
          "DELETE",
        );
        return;
      } catch (err) {
        const running = err instanceof APIError && err.status === 409;
        if (!running || attempt >= REMOVE_ATTEMPTS) throw err;
        await new Promise((resolve) => setTimeout(resolve, REMOVE_INTERVAL_MS));
      }
    }
  }

  async function submit() {
    await api(
      `/sessions/${session.id}${removing ? "?remove=1" : ""}`,
      NoContentSchema,
      "DELETE",
    );
    // A plain terminal closed on purpose is not kept as an exited record.
    if (plain) await removeWhenExited();
    if (removing || plain) onremoved?.();
    // The user ended it on purpose: a page showing it leaves rather than
    // offering to resume or showing an exit banner.
    if (archiving) onarchived?.(session);
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
