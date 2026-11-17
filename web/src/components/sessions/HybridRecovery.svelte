<script lang="ts">
  import { findShell } from "../../hooks/shared/shell.svelte";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import { resumeSession } from "../../domains/sessions/recoverable";
  import type { ArchivedSession } from "../../contracts/schemas";
  import HybridRecordHints from "./HybridRecordHints.svelte";
  import HybridResumeActions from "./HybridResumeActions.svelte";

  // The session page of a closed or interrupted hybrid session: the agent
  // still has the conversation, so it can resume on the recorded view.
  let {
    record,
    onresumed,
    leave,
  }: {
    record: ArchivedSession;
    onresumed: () => void;
    /** Leaves this session: the page returns home, a split closes its pane. */
    leave: () => void;
  } = $props();
  let busy = $state(false);
  let error = $state("");
  const shell = findShell();
  // A rebuilt worktree changes the repository's worktrees and the records,
  // whether or not the resume that follows succeeds; this page stays, so a
  // failure shows here like any other.
  function rebuilt(failure: string) {
    if (failure) error = failure;
    void shell?.repositories?.refresh();
    void shell?.archived?.refresh();
  }
</script>

<div
  class="flex shrink-0 flex-col gap-2 border-b bg-muted px-4 py-2 text-sm"
  role="status"
  aria-label={m.s3_resume_session_label()}
>
  <p>
    {m.s3_hybrid_recovery_banner({
      state:
        record.state === "closed"
          ? m.s3_hybrid_closed_clause()
          : m.s3_hybrid_interrupted_clause(),
      agent: record.agent.name,
      view: record.view === "chat" ? m.s3_view_chat() : m.s3_view_terminal(),
    })}
  </p>
  <HybridRecordHints {record} />
  {#if error}<p class="text-destructive" role="alert">{error}</p>{/if}
  <div class="flex flex-wrap gap-2">
    <HybridResumeActions
      {record}
      resume={resumeSession}
      onresumed={() => onresumed()}
      onrebuilt={rebuilt}
      disabled={busy}
      onbusy={(active) => (busy = active)}
      onerror={(message) => (error = message)}
    />
    <Button variant="ghost" size="sm" onclick={leave}
      >{m.s3_back_to_list()}</Button
    >
  </div>
</div>
