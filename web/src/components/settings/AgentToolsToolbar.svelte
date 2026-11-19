<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { locale } from "$lib/locale";
  import IconRefresh from "~icons/tabler/refresh";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import type { AgentToolsReport } from "../../contracts/schemas";
  import { summarizeAgentTools } from "../../domains/settings/agent-tools";
  import type { AgentToolsViewState } from "../../hooks/settings/agent-tools-inspector.svelte";

  let {
    state,
    report,
    error,
    checkError,
    checking,
    rescanning,
    busy,
    primaryAction = true,
    onrescan,
    oncheck,
  }: {
    state: AgentToolsViewState;
    report: AgentToolsReport | undefined;
    error: string;
    checkError: string;
    checking: boolean;
    rescanning: boolean;
    busy: boolean;
    primaryAction?: boolean;
    onrescan: () => void;
    oncheck: () => void;
  } = $props();

  const summary = $derived(summarizeAgentTools(report));

  function formatTimestamp(iso: string | undefined): string {
    if (!iso) return "";
    const epoch = Date.parse(iso);
    if (Number.isNaN(epoch)) return "";
    try {
      return new Intl.DateTimeFormat(locale.current, {
        timeStyle: "short",
      }).format(epoch);
    } catch {
      return "";
    }
  }
</script>

{#if state === "unavailable"}
  <InlineNotice tone="warning">
    <p>{m.agent_tools_unavailable()}</p>
    <p class="mt-1 font-mono text-xs">{error}</p>
  </InlineNotice>
  <Button variant="outline" onclick={onrescan}>{m.s2_reload()}</Button>
{:else if state === "ready" && report}
  <div class="flex flex-wrap items-center justify-between gap-3">
    <div class="flex min-w-0 flex-wrap items-center gap-2 text-sm">
      <span class="font-medium"
        >{m.agent_tools_summary_installed({
          installed: String(summary.installedCount),
        })}</span
      >
      {#if summary.updatesCount > 0}
        <Badge variant="info"
          >{m.agent_tools_summary_updates({
            updates: String(summary.updatesCount),
          })}</Badge
        >
      {/if}
      <span class="text-xs text-muted-foreground">
        {report.remoteCheckedAt
          ? m.agent_tools_checked_remote_at({
              time: formatTimestamp(report.remoteCheckedAt),
            })
          : m.agent_tools_checked_local_at({
              time: formatTimestamp(report.checkedAt),
            })}
      </span>
    </div>
    <div class="flex shrink-0 items-center gap-2">
      <Button
        variant="outline"
        size="icon"
        aria-disabled={busy || undefined}
        tabindex={busy ? -1 : undefined}
        aria-label={m.agent_tools_rescan_aria()}
        title={m.agent_tools_rescan_aria()}
        onclick={() => {
          if (!busy) onrescan();
        }}
      >
        <IconRefresh
          class={rescanning ? "motion-safe:animate-spin" : ""}
          aria-hidden="true"
        />
      </Button>
      <Button
        variant={primaryAction ? "default" : "outline"}
        disabled={!summary.hasCheckableTargets}
        aria-disabled={busy || undefined}
        tabindex={busy ? -1 : undefined}
        onclick={() => {
          if (!busy && summary.hasCheckableTargets) oncheck();
        }}
      >
        {checking ? m.agent_tools_checking_btn() : m.agent_tools_check_btn()}
      </Button>
    </div>
  </div>
{/if}

{#if checkError}
  <InlineNotice tone="warning">
    <p>{m.agent_tools_check_error({ error: checkError })}</p>
  </InlineNotice>
{/if}
