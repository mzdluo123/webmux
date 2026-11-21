<script lang="ts">
  import IconLoader2 from "~icons/tabler/loader-2";
  import { findShell } from "../../hooks/shared/shell.svelte";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import { resumeSession } from "../../domains/sessions/recoverable";
  import type { ArchivedSession, SessionInfo } from "../../contracts/schemas";
  import HybridRecordHints from "./HybridRecordHints.svelte";
  import HybridResumeActions from "./HybridResumeActions.svelte";

  // The session page of a closed or interrupted hybrid session: the agent
  // still has the conversation, so it can resume on the recorded view.
  let {
    record,
    claimAutoResume,
    autoResuming = false,
    onbusy = () => {},
    onresumed,
    leave,
  }: {
    record: ArchivedSession;
    claimAutoResume: () => boolean;
    /** True on the first render of a resume-on-open session before mount fires. */
    autoResuming?: boolean;
    /** Reports when a resume request is in flight so the stage can yield full space. */
    onbusy?: (busy: boolean) => void;
    onresumed: (session: SessionInfo) => void;
    /** Leaves this session: the page returns home, a split closes its pane. */
    leave: () => void;
  } = $props();
  let busy = $state(false);
  let error = $state("");
  const resuming = $derived(busy || autoResuming);
  const shell = findShell();
  function setBusy(active: boolean) {
    busy = active;
    onbusy(active);
  }
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
  class={resuming
    ? "flex min-h-0 flex-1 flex-col items-center justify-center gap-4 overflow-y-auto bg-background p-6 motion-safe:animate-in motion-safe:fade-in-0 motion-safe:duration-200"
    : "flex shrink-0 flex-col gap-2 border-b bg-muted px-4 py-2 text-sm"}
  role="status"
  aria-live={resuming ? "polite" : undefined}
  aria-label={m.s3_resume_session_label()}
>
  {#if resuming}
    <IconLoader2
      class="size-8 shrink-0 animate-spin text-muted-foreground motion-reduce:animate-none"
      aria-hidden="true"
    />
    <h2 class="text-center text-base font-medium">
      {record.view === "chat"
        ? m.s3_resuming_to_chat()
        : m.s3_resuming_to_terminal()}
    </h2>
    <p class="max-w-xs text-center text-xs text-muted-foreground">
      {m.s3_resuming_agent_hint({ agent: record.agent.name })}
    </p>
    <Button variant="ghost" size="sm" onclick={leave}
      >{m.s3_back_to_list()}</Button
    >
  {:else}
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
  {/if}
  <div class={resuming ? "contents" : "flex flex-wrap gap-2"}>
    <HybridResumeActions
      {record}
      {claimAutoResume}
      resume={resumeSession}
      {onresumed}
      onrebuilt={rebuilt}
      disabled={busy}
      hideButton={resuming}
      onbusy={setBusy}
      onerror={(message) => (error = message)}
    />
    {#if !resuming}
      <Button variant="ghost" size="sm" onclick={leave}
        >{m.s3_back_to_list()}</Button
      >
    {/if}
  </div>
</div>
