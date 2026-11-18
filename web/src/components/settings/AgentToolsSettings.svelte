<script lang="ts">
  import { onMount } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { locale } from "$lib/locale";
  import IconRefresh from "~icons/tabler/refresh";
  import IconTerminal from "~icons/tabler/terminal-2";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Card from "$lib/components/ui/card";
  import * as Empty from "$lib/components/ui/empty";
  import * as Item from "$lib/components/ui/item";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { Skeleton } from "$lib/components/ui/skeleton";
  import { AdvancedSection } from "$lib/components/ui/advanced-section";
  import { InfoHint } from "$lib/components/ui/info-hint";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import AgentToolRow from "./AgentToolRow.svelte";
  import { api } from "../../platform/api";
  import { errorMessage } from "../../platform/error";
  import {
    AgentToolsReportSchema,
    type AgentToolsReport,
  } from "../../contracts/schemas";

  type AgentToolsFixture = {
    report?: AgentToolsReport;
    state?: "loading" | "ready" | "unavailable";
    error?: string;
  };

  let {
    fixture,
  }: {
    fixture?: AgentToolsFixture;
  } = $props();

  let fetched = $state<{
    report?: AgentToolsReport;
    state: "loading" | "ready" | "unavailable";
    error: string;
  }>({
    state: "loading",
    error: "",
  });
  let overridden = $state(false);
  let checkError = $state("");
  let checking = $state(false);
  let rescanning = $state(false);
  let uninstalledOpen = $state(false);

  const active = $derived(
    overridden || !fixture
      ? fetched
      : {
          report: fixture.report,
          state: fixture.state ?? (fixture.report ? "ready" : "loading"),
          error: fixture.error ?? "",
        },
  );
  const report = $derived(active.report);
  const viewState = $derived(active.state);
  const loadError = $derived(active.error);
  const busy = $derived(viewState === "loading" || checking || rescanning);

  const installedTools = $derived(
    report?.tools.filter(
      (tool) => tool.installed || Boolean(tool.connector?.installed),
    ) ?? [],
  );
  const uninstalledTools = $derived(
    report?.tools.filter(
      (tool) => !tool.installed && !tool.connector?.installed,
    ) ?? [],
  );
  const installedCount = $derived(
    report?.tools.filter((tool) => tool.installed).length ?? 0,
  );
  const updatesCount = $derived(
    (report?.tools ?? []).reduce(
      (sum, tool) =>
        sum +
        (tool.updateStatus === "update_available" ? 1 : 0) +
        (tool.connector?.updateStatus === "update_available" ? 1 : 0),
      0,
    ),
  );

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

  async function loadReport(refresh = false) {
    const previous = report;
    if (refresh && busy) return;
    if (refresh && previous) {
      fetched = { ...active };
      rescanning = true;
    } else if (checking || rescanning) {
      return;
    } else {
      fetched = { report: previous, state: "loading", error: "" };
    }
    overridden = true;
    checkError = "";
    try {
      const path = refresh ? "/agent-tools?refresh=1" : "/agent-tools";
      const next = await api(path, AgentToolsReportSchema, "GET");
      fetched = { report: next, state: "ready", error: "" };
    } catch (err) {
      const text = errorMessage(err);
      if (previous) {
        fetched = { report: previous, state: "ready", error: "" };
        checkError = text;
      } else {
        fetched = { report: undefined, state: "unavailable", error: text };
      }
    } finally {
      rescanning = false;
    }
  }

  async function checkUpdates() {
    if (busy) return;
    const previous = report;
    fetched = { ...active };
    checking = true;
    overridden = true;
    checkError = "";
    try {
      const next = await api(
        "/agent-tools/check",
        AgentToolsReportSchema,
        "POST",
        {
          refreshLocal: false,
        },
      );
      fetched = { report: next, state: "ready", error: "" };
    } catch (err) {
      fetched = {
        report: previous,
        state: previous ? "ready" : "unavailable",
        error: previous ? "" : errorMessage(err),
      };
      if (previous) {
        checkError = errorMessage(err);
      }
    } finally {
      checking = false;
    }
  }

  onMount(() => {
    if (!fixture) {
      void loadReport(false);
    }
  });
</script>

<Card.Root
  flat
  role="region"
  id="agent-tools"
  class="scroll-mt-4"
  aria-labelledby="agent-tools-title"
>
  <Card.Header>
    <Card.Title id="agent-tools-title">{m.agent_tools_title()}</Card.Title>
    <Card.Description
      ><span class="flex items-center gap-1"
        >{m.agent_tools_desc()}<InfoHint label={m.agent_tools_title()}
          >{m.agent_tools_hint()}</InfoHint
        ></span
      ></Card.Description
    >
  </Card.Header>
  <Card.Content>
    <ScrollArea class="max-h-[min(25rem,60dvh)]">
      <div class="space-y-4">
        {#if viewState === "loading"}
          <div
            role="status"
            aria-busy="true"
            aria-label={m.agent_tools_loading()}
            class="space-y-3"
          >
            {#each [0, 1, 2] as idx (idx)}
              <div class="flex items-center gap-3 py-2">
                <Skeleton class="size-9 shrink-0" />
                <div class="min-w-0 flex-1 space-y-2">
                  <Skeleton class="h-4 w-36" />
                  <Skeleton class="h-3 w-52 max-w-full" />
                </div>
                <Skeleton class="h-5 w-16 shrink-0" />
              </div>
            {/each}
          </div>
        {:else if viewState === "unavailable"}
          <InlineNotice tone="danger">
            <p>{m.agent_tools_unavailable()}</p>
            <p class="mt-1 font-mono text-xs">{loadError}</p>
          </InlineNotice>
          <Button variant="outline" onclick={() => void loadReport(true)}
            >{m.s2_reload()}</Button
          >
        {:else if report}
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex min-w-0 flex-wrap items-center gap-2 text-sm">
              <span class="font-medium"
                >{m.agent_tools_summary_installed({
                  installed: String(installedCount),
                })}</span
              >
              {#if updatesCount > 0}
                <Badge variant="info"
                  >{m.agent_tools_summary_updates({
                    updates: String(updatesCount),
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
                size="icon-touch"
                aria-disabled={busy || undefined}
                tabindex={busy ? -1 : undefined}
                aria-label={m.agent_tools_rescan_aria()}
                title={m.agent_tools_rescan_aria()}
                onclick={() => {
                  if (busy) return;
                  void loadReport(true);
                }}
              >
                <IconRefresh
                  class={rescanning ? "motion-safe:animate-spin" : ""}
                  aria-hidden="true"
                />
              </Button>
              <Button
                variant="default"
                disabled={installedTools.length === 0}
                aria-disabled={busy || undefined}
                tabindex={busy ? -1 : undefined}
                onclick={() => {
                  if (busy || installedTools.length === 0) return;
                  void checkUpdates();
                }}
              >
                {checking
                  ? m.agent_tools_checking_btn()
                  : m.agent_tools_check_btn()}
              </Button>
            </div>
          </div>

          {#if checkError}
            <InlineNotice tone="warning">
              <p>{m.agent_tools_check_error({ error: checkError })}</p>
            </InlineNotice>
          {/if}

          {#if installedTools.length === 0}
            <Empty.Root variant="outline">
              <Empty.Header>
                <Empty.Media variant="icon"
                  ><IconTerminal aria-hidden="true" /></Empty.Media
                >
                <Empty.Title
                  >{m.agent_tools_empty_installed_title()}</Empty.Title
                >
                <Empty.Description
                  >{m.agent_tools_empty_installed_desc()}</Empty.Description
                >
              </Empty.Header>
            </Empty.Root>
          {:else}
            <Item.Group
              variant="divided"
              aria-label={m.agent_tools_installed_list_aria()}
            >
              {#each installedTools as tool (tool.id)}
                <AgentToolRow {tool} />
              {/each}
            </Item.Group>
          {/if}

          {#if uninstalledTools.length > 0}
            <AdvancedSection
              title={m.agent_tools_uninstalled_toggle({
                count: String(uninstalledTools.length),
              })}
              bind:open={uninstalledOpen}
            >
              <Item.Group
                variant="divided"
                aria-label={m.agent_tools_uninstalled_list_aria()}
              >
                {#each uninstalledTools as tool (tool.id)}
                  <AgentToolRow {tool} compact />
                {/each}
              </Item.Group>
            </AdvancedSection>
          {/if}
        {/if}
      </div>
    </ScrollArea>
  </Card.Content>
</Card.Root>
