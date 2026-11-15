<script lang="ts">
  import type { Shell } from "../../hooks/shared/shell.svelte";
  import type {
    LaunchEntry,
    LaunchScope,
  } from "../../domains/sessions/new-session-defaults";
  import type { SessionInfo } from "../../contracts/schemas";
  import type { Repositories } from "../../hooks/workspace/repositories.svelte";
  import type { LaunchConfigs } from "../../hooks/settings/launch-configs.svelte";
  import type { Archived } from "../../hooks/sessions/archived.svelte";
  import { pluginContributions } from "../../plugins/registry.svelte";
  import LazyDialog from "../shared/LazyDialog.svelte";
  import RenameSessionDialog from "../sessions/RenameSessionDialog.svelte";
  import CloseSessionDialog from "../sessions/CloseSessionDialog.svelte";
  import { m } from "$lib/paraglide/messages.js";

  let {
    shell,
    creating = $bindable(null),
    renaming = $bindable(null),
    closing = $bindable(null),
    forwarding = $bindable(false),
    repositories,
    launchConfigs,
    archived,
    onrefreshworktrees,
  }: {
    shell: Shell;
    creating: { entry?: LaunchEntry; scope?: LaunchScope } | null;
    renaming: SessionInfo | null;
    closing: SessionInfo | null;
    forwarding: boolean;
    repositories?: Repositories;
    launchConfigs?: LaunchConfigs;
    archived?: Archived;
    onrefreshworktrees: () => void;
  } = $props();
  const forward = pluginContributions.entries.forward;
</script>

{#if creating}
  {@const target = creating}
  <LazyDialog
    load={() => import("../sessions/NewSessionDialog.svelte")}
    label={m.s1_new_session_form()}
    onclose={() => (creating = null)}
  >
    {#snippet children(page)}
      <page.default
        launchConfigs={launchConfigs?.state ?? {
          status: "unavailable",
          error: m.s1_launch_configs_unavailable(),
        }}
        repositories={[...(repositories?.list ?? [])]}
        defaultCwd={shell.setup.value.cwd}
        entry={target.entry}
        scope={target.scope}
        selectedRepositoryId={repositories?.choice ?? "all"}
        intent={target.entry?.kind === "new-worktree" ? "worktree" : "session"}
        onchange={onrefreshworktrees}
        onworktreecreated={(tree) => {
          const workspace = pluginContributions.entries.workspace;
          if (!workspace || !repositories)
            throw new Error(m.s1_workspace_not_enabled());
          workspace.pinCreatedWorktree(repositories.list, tree);
        }}
        onclose={() => (creating = null)}
      />
    {/snippet}
  </LazyDialog>
{/if}
{#if forwarding && forward}
  <LazyDialog
    load={forward.dialog}
    label={m.s1_port_forwarding()}
    onclose={() => (forwarding = false)}
  >
    {#snippet children(page)}<page.default
        sessions={[...shell.sessions.sessions]}
        detection={shell.setup.value.portDetection}
        onclose={() => (forwarding = false)}
      />{/snippet}
  </LazyDialog>
{/if}
{#if renaming}<RenameSessionDialog
    session={renaming}
    onclose={() => (renaming = null)}
  />{/if}
{#if closing}
  <CloseSessionDialog
    session={closing}
    onremoved={() => {
      if (!closing) return;
      shell.drafts.remove(closing.id);
      shell.archiveSignal.emit(closing.id);
    }}
    onarchived={(session) => {
      if (archived) void archived.refresh();
      shell.archiveSignal.emit(session.id);
    }}
    onclose={() => (closing = null)}
  />
{/if}
