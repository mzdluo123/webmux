<script lang="ts">
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconLayoutList from "~icons/tabler/layout-list";
  import { Button } from "$lib/components/ui/button";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import { ListButton } from "$lib/components/ui/list-button";
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
    onall,
    onattention,
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
    onall: () => void;
    onattention: () => void;
    onopen: (session: SessionInfo) => void;
    onrename: (session: SessionInfo) => void;
    onclose: (session: SessionInfo) => void;
  } = $props();

  const pinnedIds = $derived(new Set(pinned.map((session) => session.id)));
  const attentionCount = $derived(
    sessions.filter((session) => queueGroup(session) === "attention").length,
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
    for (const session of sessionsByPin(sessions, pinned)) {
      const group =
        byRepository.get(session.workspace?.repositoryId ?? "") ?? direct;
      group.sessions.push(session);
    }
    return [...byRepository.values(), direct].filter(
      (group) => group.sessions.length,
    );
  });
</script>

<div class="space-y-3">
  <Button
    variant="ghost"
    size="sm-touch"
    class="w-full justify-start"
    aria-current={allCurrent ? "page" : undefined}
    onclick={onall}
  >
    <IconLayoutList aria-hidden="true" />{m.s8_sidebar_all_sessions()}
  </Button>
  {#if attentionCount || attentionCurrent}
    <Button
      variant="ghost"
      size="sm-touch"
      class="w-full justify-between"
      aria-current={attentionCurrent ? "page" : undefined}
      onclick={onattention}
    >
      {m.s8_queue_attention_view()}
      <span class="font-mono tabular-nums">{attentionCount}</span>
    </Button>
  {/if}
  {#each groups as group (group.id)}
    {@const open = workbenchLayout.isProjectOpen(group.id)}
    <section aria-label={group.name}>
      <Collapsible.Root
        {open}
        onOpenChange={(next) => workbenchLayout.setProjectOpen(group.id, next)}
      >
        <h2>
          <Collapsible.Trigger>
            {#snippet child({ props })}
              <ListButton {...props} variant="section-header">
                <SidebarProjectAvatar
                  name={group.name}
                  repositoryId={group.id === "direct" ? undefined : group.id}
                />
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
                  ]}
                  aria-hidden="true"
                />
              </ListButton>
            {/snippet}
          </Collapsible.Trigger>
        </h2>
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
