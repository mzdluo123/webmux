<script lang="ts">
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconPlus from "~icons/tabler/plus";
  import { Button } from "$lib/components/ui/button";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import { ListButton } from "$lib/components/ui/list-button";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";
  import type { Repository, SessionInfo } from "../../contracts/schemas";
  import {
    queueGroup,
    queueLocation,
    sessionsByPin,
  } from "../../domains/workspace/session-queue";
  import { workbenchLayout } from "../../domains/workspace/workbench-layout.svelte";
  import SidebarLiveSessionRow from "./SidebarLiveSessionRow.svelte";
  import SidebarProjectAvatar from "./SidebarProjectAvatar.svelte";

  let {
    sessions,
    pinned,
    repositories,
    configIcons,
    current,
    allCurrent,
    attentionCurrent,
    onfilter,
    oncreate,
    onopen,
    onrename,
    onclose,
  }: {
    sessions: readonly SessionInfo[];
    pinned: readonly SessionInfo[];
    repositories: readonly Repository[];
    configIcons: ReadonlyMap<string, string>;
    current: string;
    allCurrent: boolean;
    attentionCurrent: boolean;
    onfilter: (attentionOnly: boolean) => void;
    oncreate: (repositoryId?: string) => void;
    onopen: (session: SessionInfo) => void;
    onrename: (session: SessionInfo) => void;
    onclose: (session: SessionInfo) => void;
  } = $props();

  const pinnedIds = $derived(new Set(pinned.map((session) => session.id)));
  const attentionOnly = $derived(workbenchLayout.attentionOnly);
  const pending = $derived(
    sessions.filter((session) => queueGroup(session) === "attention"),
  );
  const groups = $derived.by(() => {
    const byRepository = new Map(
      repositories.map(({ id, name }) => [
        id,
        { id, name, sessions: [] as SessionInfo[] },
      ]),
    );
    const direct = {
      id: "direct",
      name: m.s3_scope_direct_sessions(),
      sessions: [] as SessionInfo[],
    };
    for (const session of sessionsByPin(
      attentionOnly ? pending : sessions,
      pinned,
    )) {
      const group =
        byRepository.get(session.workspace?.repositoryId ?? "") ?? direct;
      group.sessions.push(session);
    }
    return [...byRepository.values(), direct].filter(
      (group) => group.sessions.length || (!attentionOnly && group !== direct),
    );
  });

  let navigation = $state<HTMLDivElement | null>(null);
  let attentionFilter = $state<HTMLElement | null>(null);
  $effect.pre(() => {
    if (!attentionOnly) return;
    const matchingSessions = pending;
    const matchingProjects = groups;
    const focused = document.activeElement;
    if (!focused || !navigation?.contains(focused)) return;
    const sessionId = focused
      .closest("[data-sidebar-session-id]")
      ?.getAttribute("data-sidebar-session-id");
    const projectId = focused
      .closest("[data-sidebar-project-id]")
      ?.getAttribute("data-sidebar-project-id");
    if (
      (sessionId &&
        !matchingSessions.some((session) => session.id === sessionId)) ||
      (projectId && !matchingProjects.some((group) => group.id === projectId))
    ) {
      attentionFilter?.focus();
    }
  });
</script>

<div class="space-y-3" bind:this={navigation}>
  <SegmentedControl
    size="navigation"
    aria-label={m.s8_sidebar_session_filter()}
    value={attentionCurrent ? "attention" : allCurrent ? "all" : ""}
    onValueChange={(value) => onfilter(value === "attention")}
  >
    <SegmentedControlItem value="all">{m.s3_filter_all()}</SegmentedControlItem>
    <SegmentedControlItem value="attention" bind:ref={attentionFilter}>
      <span class="min-w-0 wrap-anywhere">{m.s8_sidebar_attention()}</span>
      {#if pending.length}
        <span class="shrink-0 font-mono tabular-nums">{pending.length}</span>
      {/if}
    </SegmentedControlItem>
  </SegmentedControl>
  {#if attentionOnly && !groups.length}
    <p class="px-2 py-2 text-xs text-muted-foreground" role="status">
      {m.s8_queue_empty_attention()}
    </p>
  {/if}
  {#each groups as group (group.id)}
    {@const hasSessions = group.sessions.length > 0}
    {@const open = hasSessions && workbenchLayout.isProjectOpen(group.id)}
    {@const repositoryId = group.id === "direct" ? undefined : group.id}
    {@const createLabel = repositoryId
      ? m.s8_sidebar_create_session({ project: group.name })
      : m.s8_sidebar_create_direct_session()}
    <section aria-label={group.name} data-sidebar-project-id={group.id}>
      <Collapsible.Root
        {open}
        onOpenChange={(next) => workbenchLayout.setProjectOpen(group.id, next)}
      >
        <div
          class="flex items-center gap-1"
          data-layout-row={repositoryId ? `r:${repositoryId}` : "direct"}
        >
          <h2 class="min-w-0 flex-1">
            <Collapsible.Trigger disabled={!hasSessions}>
              {#snippet child({ props })}
                <ListButton {...props} variant="section-header">
                  <SidebarProjectAvatar name={group.name} {repositoryId} />
                  <span class="min-w-0 flex-1 truncate" title={group.name}
                    >{group.name}</span
                  >
                  <span class="font-mono font-normal tabular-nums"
                    >{group.sessions.length}</span
                  >
                  <IconChevronRight
                    class={[
                      "size-3.5 shrink-0 motion-safe:transition-transform",
                      open && "rotate-90",
                      !hasSessions && "invisible",
                    ]}
                    aria-hidden="true"
                  />
                </ListButton>
              {/snippet}
            </Collapsible.Trigger>
          </h2>
          <Button
            variant="quiet"
            size="icon-touch-sm"
            aria-label={createLabel}
            title={createLabel}
            onclick={() => oncreate(repositoryId)}
          >
            <IconPlus aria-hidden="true" />
          </Button>
        </div>
        <Collapsible.Content>
          <ul class="mt-1 space-y-1" aria-label={group.name}>
            {#each group.sessions as session (session.id)}
              <SidebarLiveSessionRow
                {session}
                {current}
                location={queueLocation(session, repositories)}
                icon={session.agent?.icon ??
                  configIcons.get(session.configId ?? "") ??
                  ""}
                pinned={pinnedIds.has(session.id)}
                compact
                {onopen}
                {onrename}
                {onclose}
              />
            {/each}
          </ul>
        </Collapsible.Content>
      </Collapsible.Root>
    </section>
  {/each}
</div>
