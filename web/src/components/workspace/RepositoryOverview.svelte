<script lang="ts">
  import IconCopy from "~icons/tabler/copy";
  import { m } from "$lib/paraglide/messages.js";
  import { locale } from "$lib/locale";
  import { Button } from "$lib/components/ui/button";
  import { copyText } from "../../platform/clipboard";
  import { placeArchived } from "../../domains/sessions/archived";
  import { homeLayout } from "../../domains/workspace/home-worktrees";
  import type {
    ArchivedSession,
    HomeWorktrees,
    Repository,
    SessionInfo,
  } from "../../contracts/schemas";
  import { statusSummary } from "../../domains/sessions/session-status";
  import { fetchedText } from "../../domains/workspace/worktree-sync";
  import { InlineNotice } from "$lib/components/ui/inline-notice";

  // The repository panel's overview: where the repository lives, its
  // default branch, how many worktrees the home page lists, the remotes, the
  // last fetch, the sessions and when Git was read. Removing the
  // registration starts here.
  let {
    repository,
    sessions,
    records,
    home,
    remoteCount,
    registryReady,
    onunregister,
  }: {
    repository: Repository;
    sessions: readonly SessionInfo[];
    records: readonly ArchivedSession[];
    home: HomeWorktrees;
    /** Undefined until the remotes are read, or when they cannot be. */
    remoteCount: number | undefined;
    registryReady: boolean;
    onunregister: () => void;
  } = $props();
  const available = $derived(repository.availability === "available");
  const own = $derived(
    sessions.filter((s) => s.workspace?.repositoryId === repository.id),
  );
  const shown = $derived(
    homeLayout(
      home,
      repository,
      own,
      placeArchived(
        records.filter((record) => !sessions.some((s) => s.id === record.id)),
        [repository],
      ).repositories.get(repository.id)?.worktrees,
    ).shown.length,
  );
  const main = $derived(repository.worktrees.find((w) => w.kind === "main"));
  const defaultBranch = $derived(
    main
      ? (main.branch ??
          (main.head
            ? m.s8_detached_head_hash({ hash: main.head.slice(0, 8) })
            : m.s8_no_commits_yet()))
      : "—",
  );
  const checkedTime = $derived(
    new Date(repository.checkedAt).toLocaleString(locale.current),
  );
</script>

{#snippet copyable(value: string, label: string)}
  <dd class="flex min-w-0 items-start gap-1">
    <span class="min-w-0 flex-1 font-mono break-all">{value}</span>
    <Button
      size="icon-xs"
      variant="ghost"
      aria-label={label}
      title={label}
      onclick={() => void copyText(value)}
      ><IconCopy aria-hidden="true" /></Button
    >
  </dd>
{/snippet}

<div class="space-y-3">
  {#if !available}
    <InlineNotice tone="danger"
      >{m.s8_repo_unavailable_notice({
        error: repository.error || m.s8_repo_path_missing_or_unreadable(),
      })}</InlineNotice
    >
  {/if}
  <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-2 text-xs">
    <dt class="text-muted-foreground">{m.s8_label_path()}</dt>
    {@render copyable(repository.anchorPath, m.s8_copy_repo_path())}
    {#if repository.commonDir}
      <dt class="text-muted-foreground">{m.s8_label_common_dir()}</dt>
      {@render copyable(repository.commonDir, m.s8_copy_git_common_dir())}
    {/if}
    {#if available}
      <dt class="text-muted-foreground">{m.s8_label_default_branch()}</dt>
      <dd class="font-mono break-all">{defaultBranch}</dd>
      <dt class="text-muted-foreground">{m.s8_workspace_title()}</dt>
      <dd>
        {m.s8_overview_workspaces_shown({
          shown: String(shown),
          total: String(repository.worktrees.length),
        })}
      </dd>
      <dt class="text-muted-foreground">{m.s8_repo_tab_remotes()}</dt>
      <dd>
        {remoteCount === undefined
          ? "—"
          : m.s8_overview_remote_count({ count: String(remoteCount) })}
      </dd>
      <dt class="text-muted-foreground">{m.s8_label_last_fetched()}</dt>
      <dd
        title={repository.lastFetchedAt
          ? new Date(repository.lastFetchedAt).toLocaleString(locale.current)
          : undefined}
      >
        {m.s8_overview_fetched_approx({
          text: fetchedText(repository.lastFetchedAt),
        })}
      </dd>
    {/if}
    <dt class="text-muted-foreground">{m.s8_label_sessions()}</dt>
    <dd>{statusSummary(own)}</dd>
    <dt class="text-muted-foreground">Git</dt>
    <dd>
      {available
        ? m.s8_overview_git_checked_at({ time: checkedTime })
        : m.s8_overview_git_checked_unavailable({ time: checkedTime })}
    </dd>
  </dl>
  <div class="border-t pt-3">
    <Button
      size="sm"
      variant="destructive"
      disabled={!registryReady}
      onclick={onunregister}>{m.s8_unregister_repo_btn()}</Button
    >
  </div>
</div>
