<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import type { ArchivedSession, SessionInfo } from "../../contracts/schemas";
  import type { ArchiveActions } from "../../domains/sessions/archived";
  import WorkspaceGroup from "../workspace/WorkspaceGroup.svelte";

  let {
    sessions,
    visibleIds,
    focused = false,
    compact = false,
    now,
    onclose,
    archived = [],
    visibleArchiveIds,
    archive,
  }: {
    // All sessions outside every registered repository.
    sessions: SessionInfo[];
    visibleIds?: ReadonlySet<string>;
    focused?: boolean;
    compact?: boolean;
    now: number;
    onclose: (session: SessionInfo) => void;
    // Archived records outside every registered repository.
    archived?: ArchivedSession[];
    // Archived records the filters keep; undefined while not filtering.
    visibleArchiveIds?: ReadonlySet<string>;
    archive?: ArchiveActions;
  } = $props();

  const visible = $derived(
    visibleIds
      ? sessions.filter((session) => visibleIds.has(session.id))
      : sessions,
  );
  const shownArchive = $derived(
    visibleArchiveIds
      ? archived.filter((record) => visibleArchiveIds.has(record.id))
      : archived,
  );

  // Shell-reported directories are navigation hints; grouping stays on the
  // immutable launch directory so a `cd` never moves a session between groups.
  const paths = $derived(
    [
      ...new Set([
        ...sessions.map((s) => s.cwd),
        ...archived.map((record) => record.cwd),
      ]),
    ].sort(),
  );
  const reported = $derived(visible.some((s) => s.reportedCwd));
</script>

<section
  class="min-w-0 space-y-8"
  aria-label={m.s3_scope_direct_sessions()}
  hidden={!visible.length && !shownArchive.length}
>
  <h2 class={focused ? "sr-only" : "font-semibold"}>
    {m.s3_scope_direct_sessions()}
  </h2>
  {#if reported}<p class="text-xs text-muted-foreground">
      {m.s3_direct_sessions_cwd_hint()}
    </p>{/if}
  {#each paths as path (path)}
    {@const pathArchive = shownArchive.filter((record) => record.cwd === path)}
    <WorkspaceGroup
      title={path.split("/").pop() || "/"}
      {path}
      sessions={sessions.filter((s) => s.cwd === path)}
      {visibleIds}
      {compact}
      hidden={Boolean(
        (visibleIds || visibleArchiveIds) &&
        !pathArchive.length &&
        !visible.some((session) => session.cwd === path),
      )}
      {now}
      {onclose}
      archived={pathArchive}
      {archive}
    />{/each}
</section>
