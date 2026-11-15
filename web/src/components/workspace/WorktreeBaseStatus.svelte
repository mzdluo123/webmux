<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { locale } from "$lib/locale";

  let {
    refreshing,
    baseSource,
    upstream,
    baseSubject,
    lastRefreshedAt,
    startBranch,
    refreshWarning,
  }: {
    refreshing: boolean;
    baseSource: "local" | "remote" | "";
    upstream: string;
    baseSubject: string;
    lastRefreshedAt: Date | null;
    startBranch: string;
    refreshWarning: string;
  } = $props();
</script>

{#if refreshing}
  <p
    role="status"
    aria-label={m.s8_upstream_refresh_status_aria()}
    class="text-xs text-muted-foreground"
  >
    {m.s8_fetching_upstream_commits_status({ branch: startBranch })}
  </p>
{:else if baseSource === "remote" && upstream}
  <p
    role="status"
    aria-label={m.s8_upstream_refresh_status_aria()}
    class="break-all text-xs text-muted-foreground"
  >
    {m.s8_tip_remote_latest()} · {upstream} · {baseSubject ||
      m.s8_untitled_commit()}{#if lastRefreshedAt}
      · {m.s8_last_refreshed_at({
        time: lastRefreshedAt.toLocaleTimeString(locale.current),
      })}{/if}
  </p>
{:else if baseSource === "local" && startBranch}
  <p
    role="status"
    aria-label={m.s8_upstream_refresh_status_aria()}
    class="break-all text-xs text-muted-foreground"
  >
    {m.s8_local_branch_prefix({ branch: startBranch })}
  </p>
{/if}
{#if refreshWarning}<p
    role="status"
    aria-label={m.s8_upstream_refresh_warning_aria()}
    class="text-xs text-warning"
  >
    {refreshWarning}
  </p>{/if}
