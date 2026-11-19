<script lang="ts" module>
  import type {
    ArchivedSession,
    LaunchConfigSummary,
    Repository,
    SessionInfo,
    Worktree,
  } from "../../contracts/schemas";
  import { resumesOnOpen } from "../../domains/sessions/archived";
  const byCreation = (a: SessionInfo, b: SessionInfo) =>
    Date.parse(b.createdAt) - Date.parse(a.createdAt) ||
    a.id.localeCompare(b.id);

  // Pending and live chats share their creation order across a restart.
  function chatHistorySections(
    sessions: readonly SessionInfo[],
    records: readonly ArchivedSession[],
  ) {
    const live = sessions.filter(
      (session) => session.views.includes("chat") && session.state === "active",
    );
    const ids = new Set(live.map((session) => session.id));
    const recent: (
      | { kind: "live"; session: SessionInfo }
      | { kind: "dormant"; session: ArchivedSession }
    )[] = live.map((session) => ({ kind: "live", session }));
    const archived: ArchivedSession[] = [];
    for (const session of records) {
      if (ids.has(session.id)) continue;
      if (resumesOnOpen(session)) recent.push({ kind: "dormant", session });
      else archived.push(session);
    }
    recent.sort((a, b) => byCreation(a.session, b.session));
    archived.sort(
      (a, b) =>
        Date.parse(b.archive.updatedAt) - Date.parse(a.archive.updatedAt) ||
        a.id.localeCompare(b.id),
    );
    return { recent, archived };
  }
</script>

<script lang="ts">
  import { tick } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconLoader2 from "~icons/tabler/loader-2";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import { ListButton } from "$lib/components/ui/list-button";
  import { releasableRecordWorktree } from "../../domains/sessions/archived";
  import { queueLocation } from "../../domains/workspace/session-queue";
  import LaunchConfigIcon from "../settings/LaunchConfigIcon.svelte";
  import RowStatus from "../shared/RowStatus.svelte";
  import SidebarSessionName from "../shell/SidebarSessionName.svelte";
  import SidebarSessionRow from "../shell/SidebarSessionRow.svelte";
  import SidebarSessionMenu from "../shell/SidebarSessionMenu.svelte";

  // Recent chats stay visible; manually archived or interrupted chats fold
  // below them. Both use the same archive actions for stopped records.
  let {
    sessions,
    records,
    configs,
    repositories,
    current,
    resuming = "",
    onopen,
    onresume,
    onrename,
    onclose,
    onrelease,
    onremove,
    onremoveworktree,
  }: {
    sessions: readonly SessionInfo[];
    records: readonly ArchivedSession[];
    configs: readonly LaunchConfigSummary[];
    /** Resolve which archived records' worktrees can be released. */
    repositories: readonly Repository[];
    /** The session page shown, or "" on a home page. */
    current: string;
    /** The record being resumed; its row shows the progress. */
    resuming?: string;
    onopen: (session: SessionInfo) => void;
    onresume: (record: ArchivedSession) => void;
    onrename: (session: SessionInfo) => void;
    onclose: (session: SessionInfo) => void;
    /** Asks to release the disk space of an archived record's worktree. */
    onrelease: (worktree: Worktree) => void;
    /** Asks to delete an archived record; its worktree stays. */
    onremove: (record: ArchivedSession) => void;
    /** Asks to delete an archived record and its linked worktree. */
    onremoveworktree?: (record: ArchivedSession, worktree: Worktree) => void;
  } = $props();
  // Mount one page on demand and enough pages to reveal the current record.
  // Once the closing transition finishes, the section releases its rows.
  const PAGE = 20;
  const locationIdPrefix = $props.id();
  const { recent, archived } = $derived(chatHistorySections(sessions, records));
  let archivedExpanded = $state(false);
  let archivedOverride = $state<boolean | undefined>();
  let archivedLimit = $state(PAGE);
  let archiveContent = $state<HTMLDivElement | null>(null);
  const selectedArchiveIndex = $derived(
    archived.findIndex(
      (record) => record.id === current || record.id === resuming,
    ),
  );
  const selectedArchive = $derived(archived[selectedArchiveIndex]?.id);
  $effect(() => {
    void selectedArchive;
    archivedOverride = undefined;
  });
  const archivedOpen = $derived(
    archivedOverride ?? (selectedArchiveIndex >= 0 || archivedExpanded),
  );
  const motion = Collapsible.useManualDisclosureMotion(
    () => archiveContent,
    () => archivedOpen,
  );
  const archivedShown = $derived(
    archived.slice(
      0,
      Math.max(
        archivedLimit,
        Math.ceil((selectedArchiveIndex + 1) / PAGE) * PAGE,
      ),
    ),
  );
  const archivedRemaining = $derived(archived.length - archivedShown.length);

  function revealSelectedArchive() {
    if (!archivedOpen || !selectedArchive) return;
    archiveContent
      ?.querySelector('[aria-current="page"], [aria-busy="true"]')
      ?.scrollIntoView({ block: "nearest", behavior: "instant" });
  }

  $effect(() => {
    if (motion.moving) return;
    if (!archivedOpen) archivedLimit = PAGE;
    else if (selectedArchive) void tick().then(revealSelectedArchive);
  });

  function toggleArchived(open: boolean) {
    motion.toggle(open, (next) => {
      if (selectedArchiveIndex >= 0) archivedOverride = next;
      else archivedExpanded = next;
    });
  }

  function showMore(event: MouseEvent) {
    const lastRow = archiveContent?.querySelector<HTMLButtonElement>(
      "li:last-child > button",
    );
    if (event.currentTarget === document.activeElement)
      lastRow?.focus({ preventScroll: true });
    archivedLimit = archivedShown.length + PAGE;
  }
</script>

{#snippet archivedRow(record: ArchivedSession, dormant: boolean)}
  {@const title = record.title || record.agent.name}
  {@const releasable = releasableRecordWorktree(record, repositories)}
  {@const state = dormant
    ? m.sessions_resume_on_open()
    : record.released
      ? m.s3_row_status_released()
      : m.s3_row_status_ended()}
  <li
    class="group/row relative"
    data-sidebar-session-id={dormant ? record.id : undefined}
  >
    <ListButton
      variant="navigation-action"
      class="scroll-my-1"
      aria-current={record.id === current ? "page" : undefined}
      aria-label={m.s3_session_row_label({
        title,
        status: resuming === record.id ? m.s3_history_resuming_status() : state,
      })}
      aria-describedby={`${locationIdPrefix}-${record.id}`}
      data-allow-wrap
      title={dormant
        ? m.s3_session_row_label({ title, status: state })
        : record.released
          ? m.s3_history_released_title({
              title,
              agent: record.agent.name,
              path: record.released.path,
              commit: record.released.commit.slice(0, 12),
            })
          : m.s3_history_ended_title({ title, agent: record.agent.name })}
      aria-busy={resuming === record.id}
      disabled={!!resuming}
      onclick={() => onresume(record)}
    >
      <LaunchConfigIcon
        icon={configs.find((config) => config.id === record.configId)?.icon ??
          ""}
        size="sm"
      />
      <SidebarSessionName
        {title}
        location={queueLocation(record, repositories)}
        locationId={`${locationIdPrefix}-${record.id}`}
        archived={!dormant}
      />
      {#if resuming === record.id}<span
          class="flex shrink-0 items-center gap-1 text-xs font-normal text-muted-foreground"
          ><IconLoader2
            class="size-3.5 animate-spin motion-reduce:animate-none"
            aria-hidden="true"
          />{m.s3_history_resuming_badge()}</span
        >{:else}<RowStatus
          status={dormant ? "dormant" : record.released ? "released" : "ended"}
        />{/if}
    </ListButton>
    <SidebarSessionMenu
      label={title}
      disabled={!!resuming}
      target={{
        kind: "archived",
        onresume: () => onresume(record),
        onrelease: releasable ? () => onrelease(releasable) : undefined,
        onremove: () => onremove(record),
        onremoveworktree:
          releasable && onremoveworktree
            ? () => onremoveworktree(record, releasable)
            : undefined,
      }}
    />
  </li>
{/snippet}

<h2 class="px-3 pt-1 pb-1 text-xs text-muted-foreground">
  {m.s3_history_recent()}
</h2>
{#if archived.length || recent.length}
  {#if recent.length}
    <ul class="space-y-0.5" aria-label={m.s3_history_list_label()}>
      {#each recent as entry (entry.session.id)}
        {#if entry.kind === "dormant"}
          {@render archivedRow(entry.session, true)}
        {:else}
          {@const session = entry.session}
          <SidebarSessionRow
            {session}
            {current}
            icon={session.agent?.icon ?? ""}
            location={queueLocation(session, repositories)}
            {onopen}
            {onrename}
            {onclose}
          />
        {/if}
      {/each}
    </ul>
  {/if}
  {#if archived.length}
    <Collapsible.Root
      open={archivedOpen}
      onOpenChange={toggleArchived}
      class={recent.length ? "mt-2" : ""}
    >
      <Collapsible.Trigger>
        {#snippet child({ props })}
          <Button
            {...props}
            variant="ghost"
            size="sm"
            class="w-full justify-start"
          >
            <IconChevronDown
              aria-hidden="true"
              class={archivedOpen ? "" : "-rotate-90"}
            />
            <span class="font-medium text-muted-foreground"
              >{m.s3_archived()}</span
            >
            <Badge variant="secondary" class="ml-auto">{archived.length}</Badge>
          </Button>
        {/snippet}
      </Collapsible.Trigger>
      <div
        bind:this={archiveContent}
        inert={!archivedOpen}
        aria-hidden={!archivedOpen}
        class:overflow-clip={motion.moving}
      >
        <Collapsible.Content forceMount={motion.moving}>
          {#if archivedOpen || motion.moving}
            <div class="pt-0.5">
              <ul
                class="space-y-0.5"
                aria-label={m.s3_history_archived_list_label()}
              >
                {#each archivedShown as record (record.id)}
                  {@render archivedRow(record, false)}
                {/each}
              </ul>
              {#if archivedRemaining > 0}
                <Button
                  variant="link"
                  size="sm"
                  class="w-full justify-start"
                  onclick={showMore}
                >
                  {m.s3_history_show_more({ count: archivedRemaining })}
                </Button>
              {/if}
            </div>
          {/if}
        </Collapsible.Content>
      </div>
    </Collapsible.Root>
  {/if}
{:else}
  <p class="px-3 py-2 text-sm text-muted-foreground">
    {m.s3_history_empty()}
  </p>
{/if}
