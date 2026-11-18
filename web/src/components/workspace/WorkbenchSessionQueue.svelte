<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import { m } from "$lib/paraglide/messages.js";
  import type {
    ArchivedSession,
    Repository,
    SessionInfo,
  } from "../../contracts/schemas";
  import {
    releasableRecordWorktree,
    type ArchiveActions,
  } from "../../domains/sessions/archived";
  import { sessionPins } from "../../domains/sessions/session-pins.svelte";
  import {
    sessionQueue,
    queueLocation,
    type FocusedQueueRow,
    type QueueGroup,
  } from "../../domains/workspace/session-queue";
  import ArchivedSessions from "../sessions/ArchivedSessions.svelte";
  import SessionListEmpty from "../sessions/SessionListEmpty.svelte";
  import WorkbenchQueueGroup from "./WorkbenchQueueGroup.svelte";
  import WorkbenchQueueRow from "./WorkbenchQueueRow.svelte";
  import WorkbenchQueuePreview from "./WorkbenchQueuePreview.svelte";

  let {
    sessions,
    available,
    repositories,
    records,
    archive,
    attentionOnly,
    filtering,
    now,
    onclear,
    onrename,
    onclose,
  }: {
    sessions: readonly SessionInfo[];
    available: readonly SessionInfo[];
    repositories: readonly Repository[];
    records: ArchivedSession[];
    archive: ArchiveActions;
    attentionOnly: boolean;
    filtering: boolean;
    now: number;
    onclear: () => void;
    onrename: (session: SessionInfo) => void;
    onclose: (session: SessionInfo) => void;
  } = $props();

  let focused = $state<FocusedQueueRow | null>(null);
  let menuRow = $state<FocusedQueueRow | null>(null);
  let previewRow = $state<FocusedQueueRow | null>(null);
  const held = $derived([menuRow, previewRow, focused]);
  const candidates = $derived(
    attentionOnly && held.some(Boolean)
      ? [
          ...sessions,
          ...available.filter(
            (session) =>
              held.some((row) => row?.id === session.id) &&
              !sessions.includes(session),
          ),
        ]
      : sessions,
  );
  const groups = $derived(
    sessionQueue(candidates, sessionPins.ordered(candidates), held),
  );
  const titles = $derived({
    attention: m.s8_queue_attention(),
    running: m.s8_queue_working(),
    idle: m.s8_queue_idle(),
    exited: m.s8_queue_exited(),
  });
  const primaryGroups = $derived<QueueGroup[]>(
    attentionOnly ? ["attention"] : ["attention", "running"],
  );
  const foldableGroups = ["idle", "exited"] as const;
  type FoldableGroup = "idle" | "exited";
  let expanded = $state<Record<FoldableGroup, boolean>>({
    idle: false,
    exited: false,
  });
  let filteredExpanded = $state<Partial<Record<FoldableGroup, boolean>>>({});
  $effect(() => {
    if (!filtering) filteredExpanded = {};
  });
  let previewId = $state("");
  let previewOpen = $state(false);
  let opener = $state<HTMLElement | null>(null);
  const docked = new MediaQuery("(min-width: 1440px)");
  const preview = $derived(
    available.find((session) => session.id === previewId),
  );
  const showingPreview = $derived(Boolean(previewOpen && preview));
  $effect(() => {
    if (previewOpen && !preview) {
      previewOpen = false;
      previewRow = null;
    }
  });
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
          previewRow = { id: session.id, group };
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
    {#each primaryGroups as key (key)}
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
    {#each attentionOnly ? [] : foldableGroups as key (key)}
      {@const open = filtering
        ? (filteredExpanded[key] ?? true)
        : expanded[key]}
      {#if groups[key].length}
        <WorkbenchQueueGroup
          title={titles[key]}
          count={groups[key].length}
          {open}
          onOpenChange={(value) => {
            if (filtering) filteredExpanded[key] = value;
            else expanded[key] = value;
          }}
        >
          {@render rows(key)}
        </WorkbenchQueueGroup>
      {/if}
    {/each}
    {#if attentionOnly && !groups.attention.length}
      {#if filtering}
        <SessionListEmpty filtered {onclear} />
      {:else}
        <p role="status" class="py-6 text-sm text-muted-foreground">
          {m.s8_queue_empty_attention()}
        </p>
      {/if}
    {:else if records.length}
      <ArchivedSessions
        title={m.s8_queue_timeline()}
        {records}
        {archive}
        resolveWorktree={(record) =>
          releasableRecordWorktree(record, repositories)}
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
