<script lang="ts">
  import IconChevronRight from "~icons/tabler/chevron-right";
  import { MediaQuery } from "svelte/reactivity";
  import { Button } from "$lib/components/ui/button";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import { m } from "$lib/paraglide/messages.js";
  import type {
    ArchivedSession,
    Repository,
    SessionInfo,
  } from "../../contracts/schemas";
  import { type ArchiveActions } from "../../domains/sessions/archived";
  import { sessionPins } from "../../domains/sessions/session-pins.svelte";
  import {
    sessionQueue,
    queueLocation,
    type FocusedQueueRow,
    type QueueGroup,
  } from "../../domains/workspace/session-queue";
  import ArchivedSessions from "../sessions/ArchivedSessions.svelte";
  import WorkbenchQueueRow from "./WorkbenchQueueRow.svelte";
  import WorkbenchQueuePreview from "./WorkbenchQueuePreview.svelte";

  let {
    sessions,
    available,
    repositories,
    records,
    archive,
    filtering,
    now,
    onrename,
    onclose,
  }: {
    sessions: readonly SessionInfo[];
    available: readonly SessionInfo[];
    repositories: readonly Repository[];
    records: ArchivedSession[];
    archive: ArchiveActions;
    filtering: boolean;
    now: number;
    onrename: (session: SessionInfo) => void;
    onclose: (session: SessionInfo) => void;
  } = $props();

  let focused = $state<FocusedQueueRow | null>(null);
  let menuRow = $state<FocusedQueueRow | null>(null);
  let previewRow = $state<FocusedQueueRow | null>(null);
  const groups = $derived(
    sessionQueue(sessions, sessionPins.ordered(sessions), [
      menuRow,
      previewRow,
      focused,
    ]),
  );
  const titles = $derived({
    attention: m.s8_queue_attention(),
    running: m.s8_queue_working(),
    idle: m.s8_queue_idle(),
    exited: m.s8_queue_exited(),
  });
  let expanded = $state({ idle: false, exited: false });
  let previewId = $state("");
  let previewOpen = $state(false);
  let opener = $state<HTMLElement | null>(null);
  const docked = new MediaQuery("(min-width: 1440px)");
  const preview = $derived(
    available.find((session) => session.id === previewId),
  );
  const showingPreview = $derived(Boolean(previewOpen && preview));
  $effect(() => {
    if (previewOpen && !preview) previewOpen = false;
  });

  function archiveWorktree(record: ArchivedSession) {
    const tree = repositories
      .find((repo) => repo.id === record.workspace?.repositoryId)
      ?.worktrees.find((item) => item.id === record.workspace?.worktreeId);
    return tree?.kind === "linked" && tree.available && !tree.prunable
      ? tree
      : undefined;
  }
</script>

{#snippet rows(group: QueueGroup)}
  <ul
    class={group === "attention" ? "space-y-2" : "border-t"}
    aria-label={titles[group]}
  >
    {#each groups[group] as session (session.id)}
      <WorkbenchQueueRow
        {session}
        {group}
        {now}
        {onrename}
        {onclose}
        location={queueLocation(session, repositories)}
        previewing={showingPreview && previewId === session.id}
        onpreview={(trigger) => {
          opener = trigger;
          previewId = session.id;
          previewOpen = true;
          if (!docked.current) previewRow = { id: session.id, group };
        }}
        onmenuopen={(open) => {
          if (open) menuRow = { id: session.id, group };
          else if (menuRow?.id === session.id) menuRow = null;
        }}
        onfocusrow={() => (focused = { id: session.id, group })}
        onblurrow={() => {
          if (focused?.id === session.id) focused = null;
        }}
      />
    {/each}
  </ul>
{/snippet}

<div
  class={[
    "grid min-w-0 gap-4",
    docked.current && showingPreview && "grid-cols-[minmax(0,1fr)_20rem]",
  ]}
>
  <div class="min-w-0 space-y-6">
    {#each ["attention", "running"] as group (group)}
      {@const key = group as "attention" | "running"}
      {#if groups[key].length}
        <section>
          <h2 class="mb-2 flex items-baseline gap-2 text-base font-semibold">
            {titles[key]}
            <span class="font-mono text-xs font-normal text-muted-foreground"
              >{groups[key].length}</span
            >
          </h2>
          {@render rows(key)}
        </section>
      {/if}
    {/each}
    {#each ["idle", "exited"] as group (group)}
      {@const key = group as "idle" | "exited"}
      {#if groups[key].length}
        <Collapsible.Root
          open={filtering || expanded[key]}
          onOpenChange={(open) => (expanded[key] = open)}
        >
          <Collapsible.Trigger>
            {#snippet child({ props })}
              <Button {...props} variant="ghost" class="w-full justify-start">
                <IconChevronRight
                  class={filtering || expanded[key] ? "rotate-90" : ""}
                  aria-hidden="true"
                />
                {titles[key]}
                <span class="font-mono text-xs text-muted-foreground"
                  >{groups[key].length}</span
                >
              </Button>
            {/snippet}
          </Collapsible.Trigger>
          <Collapsible.Content>{@render rows(key)}</Collapsible.Content>
        </Collapsible.Root>
      {/if}
    {/each}
    {#if records.length}
      <ArchivedSessions
        title={m.s8_queue_timeline()}
        {records}
        {archive}
        resolveWorktree={archiveWorktree}
        timeline
      />
    {/if}
  </div>

  <WorkbenchQueuePreview
    session={preview}
    location={preview ? queueLocation(preview, repositories) : ""}
    docked={docked.current}
    open={showingPreview}
    {opener}
    onclose={() => (previewOpen = false)}
    onrestored={() => (previewRow = null)}
  />
</div>
