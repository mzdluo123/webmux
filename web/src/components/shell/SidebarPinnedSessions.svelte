<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import type { SessionInfo } from "../../contracts/schemas";
  import { queueLocation } from "../../domains/workspace/session-queue";
  import { getShell, openSession } from "../../hooks/shared/shell.svelte";
  import SidebarSessionRow from "./SidebarSessionRow.svelte";

  let {
    sessions,
    current,
    configIcons,
    showLocation,
  }: {
    sessions: readonly SessionInfo[];
    current: string;
    configIcons: ReadonlyMap<string, string>;
    showLocation: boolean;
  } = $props();
  const shell = getShell();
</script>

<section class="mb-3 border-b pb-3" aria-label={m.s1_pinned_sessions()}>
  <h2 class="px-3 pb-1 text-xs text-muted-foreground">{m.s1_pinned()}</h2>
  <ul class="space-y-0.5">
    {#each sessions as session (session.id)}
      <SidebarSessionRow
        {session}
        {current}
        icon={session.agent?.icon ??
          configIcons.get(session.configId ?? "") ??
          ""}
        pinned
        location={showLocation
          ? queueLocation(session, shell.repositories?.list ?? [])
          : undefined}
        onopen={openSession}
        onrename={shell.rename}
        onclose={shell.close}
      />
    {/each}
  </ul>
</section>
