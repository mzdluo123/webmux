import { tick, untrack } from "svelte";
import { SvelteMap, SvelteSet } from "svelte/reactivity";
import type { SessionInfo } from "../../contracts/schemas";
import type { XrAvailableSession } from "../../domains/terminal/xr-workspace";

/** Live-list membership and pending explicit 2D owners have different provenance. */
export function createXrSessionMembership(options: {
  live(): readonly XrAvailableSession[];
  ready(): boolean;
  panes(): readonly string[];
  info(id: string): SessionInfo | undefined;
}) {
  const confirmed = new SvelteSet<string>();
  let observed = $state.raw<string[]>([]);
  const available = $derived.by(() => {
    const current = new SvelteMap(
      options.live().map((session) => [session.id, session]),
    );
    for (const id of options.panes()) {
      if (current.has(id)) continue;
      const info = options.info(id);
      // Pending navigation owners may precede their live create event.
      // Stale active metadata cannot revive a confirmed, removed live row.
      // Mounted recovery records remain valid independently of that list.
      if ((info && info.state !== "active") || !confirmed.has(id))
        current.set(id, { id, title: info?.title ?? id });
    }
    return [...current.values()];
  });

  $effect(() => {
    const live = options.live();
    if (!options.ready()) return;
    untrack(() => {
      for (const session of live) confirmed.add(session.id);
    });
  });
  $effect(() => {
    if (!options.ready()) return;
    const valid = new SvelteSet(available.map((session) => session.id));
    untrack(() => {
      if (!observed.some((id) => !valid.has(id))) return;
      observed = observed.filter((id) => valid.has(id));
    });
  });

  return {
    get observed() {
      return observed;
    },
    get available() {
      return available;
    },
    async observe(id: string) {
      if (options.ready() && !available.some((session) => session.id === id))
        throw new Error("此会话已删除或移出可观察列表。");
      if (!observed.includes(id)) observed = [...observed, id];
      await tick();
    },
    unobserve(id: string) {
      observed = observed.filter((current) => current !== id);
    },
    clear() {
      observed = [];
    },
  };
}
