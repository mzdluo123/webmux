<script lang="ts">
  import IconLayoutList from "~icons/tabler/layout-list";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import type { Repository, SessionInfo } from "../../contracts/schemas";
  import {
    queueGroup,
    queueLocation,
  } from "../../domains/workspace/session-queue";
  import SidebarLiveSessionRow from "./SidebarLiveSessionRow.svelte";

  type Group = "attention" | "pinned" | "recent";
  type HeldRow = { id: string; group: Group };

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

  let focused = $state<HeldRow | null>(null);
  let menu = $state<HeldRow | null>(null);
  const pinnedIds = $derived(new Set(pinned.map((session) => session.id)));
  const groups = $derived.by(() => {
    const byGroup: Record<Group, SessionInfo[]> = {
      attention: [],
      pinned: [],
      recent: [],
    };
    const sorted = [...sessions].sort(
      (a, b) =>
        Date.parse(b.createdAt) - Date.parse(a.createdAt) ||
        a.id.localeCompare(b.id),
    );
    for (const session of sorted) {
      const group =
        [menu, focused].find((row) => row?.id === session.id)?.group ??
        (queueGroup(session) === "attention"
          ? "attention"
          : pinnedIds.has(session.id)
            ? "pinned"
            : "recent");
      byGroup[group].push(session);
    }
    const order = new Map(pinned.map((session, index) => [session.id, index]));
    byGroup.pinned.sort(
      (a, b) =>
        (order.get(a.id) ?? pinned.length) - (order.get(b.id) ?? pinned.length),
    );
    return byGroup;
  });
</script>

{#snippet rows(group: Group, label: string)}
  <ul class="space-y-0.5" aria-label={label}>
    {#each groups[group] as session (session.id)}
      <SidebarLiveSessionRow
        {session}
        {current}
        location={queueLocation(session, repositories)}
        icon={session.agent?.icon ??
          configIcons.get(session.configId ?? "") ??
          ""}
        pinned={pinnedIds.has(session.id)}
        {onopen}
        {onrename}
        {onclose}
        onfocusrow={() => (focused = { id: session.id, group })}
        onblurrow={() => {
          if (focused?.id === session.id) focused = null;
        }}
        onmenuopen={(open) => {
          if (open) menu = { id: session.id, group };
          else if (menu?.id === session.id) menu = null;
        }}
      />
    {/each}
  </ul>
{/snippet}

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
  {#if groups.attention.length || attentionCurrent}
    <section aria-label={m.s8_queue_attention()}>
      <Button
        variant="ghost"
        size="sm-touch"
        class="w-full justify-between"
        aria-current={attentionCurrent ? "page" : undefined}
        onclick={onattention}
      >
        {m.s8_queue_attention_view()}
        <span class="font-mono tabular-nums">{groups.attention.length}</span>
      </Button>
      {@render rows("attention", m.s8_queue_attention())}
    </section>
  {/if}
  {#if groups.pinned.length}
    <section aria-label={m.s1_pinned_sessions()}>
      <h2 class="px-3 pb-1 text-xs text-muted-foreground">{m.s1_pinned()}</h2>
      {@render rows("pinned", m.s1_pinned_sessions())}
    </section>
  {/if}
  {#if groups.recent.length}
    <section aria-label={m.s3_history_recent()}>
      <h2 class="px-3 pb-1 text-xs text-muted-foreground">
        {m.s3_history_recent()}
      </h2>
      {@render rows("recent", m.s3_history_recent())}
    </section>
  {/if}
</div>
