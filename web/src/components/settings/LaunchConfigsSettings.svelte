<script lang="ts">
  import { onMount } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { settings } from "../../domains/settings/settings.svelte";
  import { findShell } from "../../hooks/shared/shell.svelte";
  import {
    createAgentToolsInspector,
    type AgentToolsViewState,
  } from "../../hooks/settings/agent-tools-inspector.svelte";
  import { errorMessage } from "../../platform/error";
  import IconPlus from "~icons/tabler/plus";
  import IconRocket from "~icons/tabler/rocket";
  import IconTerminal from "~icons/tabler/terminal-2";
  import { AdvancedSection } from "$lib/components/ui/advanced-section";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import { Button } from "$lib/components/ui/button";
  import * as Card from "$lib/components/ui/card";
  import * as Empty from "$lib/components/ui/empty";
  import { InfoHint } from "$lib/components/ui/info-hint";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import * as Item from "$lib/components/ui/item";
  import { APIError, api, joinArgs } from "../../platform/api";
  import {
    LaunchConfigsSchema,
    LaunchPresetsSchema,
    NoContentSchema,
    type AgentToolsReport,
    type LaunchConfig,
    type LaunchConfigSummary,
    type LaunchPreset,
  } from "../../contracts/schemas";
  import {
    partitionLaunchAndAgentRows,
    type MergedLaunchRow,
  } from "../../domains/settings/agent-tools";
  import {
    LAUNCH_CONFIGS_LIMIT,
    isLaunchConfigHiddenByAgentInstall,
    type StartupShell,
  } from "../../domains/settings/launch-configs";
  import AgentToolRow from "./AgentToolRow.svelte";
  import AgentToolsToolbar from "./AgentToolsToolbar.svelte";
  import LaunchConfigDialog from "./LaunchConfigDialog.svelte";
  import LaunchPresetMenu from "./LaunchPresetMenu.svelte";

  type LaunchConfigsFixture = {
    configs?: LaunchConfigSummary[];
    presets?: LaunchPreset[];
    launchState?: "loading" | "ready" | "unavailable";
    launchError?: string;
    report?: AgentToolsReport;
    agentState?: AgentToolsViewState;
    agentError?: string;
  };

  let {
    defaultShell = "",
    fixture,
  }: {
    defaultShell?: "" | StartupShell;
    fixture?: LaunchConfigsFixture;
  } = $props();

  const shell = findShell();
  const agentInspector = createAgentToolsInspector(
    () =>
      fixture
        ? {
            report: fixture.report,
            state: fixture.agentState,
            error: fixture.agentError,
          }
        : undefined,
    () => void shell?.launchConfigs?.load(),
  );

  // Launch configs live on the server; a load failure (for example 503 when
  // the state file is corrupt or locked) only disables this card.
  let launchConfigs = $state<LaunchConfigSummary[]>([]);
  let launchPresets = $state<LaunchPreset[]>([]);
  let launchState = $state<"loading" | "ready" | "unavailable">("loading");
  let launchError = $state("");
  let launchOverridden = $state(false);
  let launchPresetBusy = $state(false);
  let launchPresetError = $state("");
  let launchDialogOpen = $state(false);
  let launchEditing = $state<string>();
  let launchDeleting = $state<LaunchConfigSummary>();
  let launchDeleteOpen = $state(false);
  let launchDeleteBusy = $state(false);
  let launchDeleteError = $state("");
  let uninstalledOpen = $state(false);

  const activeLaunch = $derived(
    launchOverridden || !fixture
      ? {
          configs: launchConfigs,
          presets: launchPresets,
          state: launchState,
          error: launchError,
        }
      : {
          configs: fixture.configs ?? [],
          presets: fixture.presets ?? [],
          state: fixture.launchState ?? "ready",
          error: fixture.launchError ?? "",
        },
  );

  const activeConfigs = $derived(activeLaunch.configs);
  const activePresets = $derived(activeLaunch.presets);
  const launchViewState = $derived(activeLaunch.state);
  const launchLoadError = $derived(activeLaunch.error);
  const launchFull = $derived(activeConfigs.length >= LAUNCH_CONFIGS_LIMIT);
  const initialLoading = $derived(
    launchViewState === "loading" || agentInspector.state === "loading",
  );

  const partitioned = $derived(
    partitionLaunchAndAgentRows(activeConfigs, agentInspector.report?.tools),
  );
  const activeRows = $derived(partitioned.active);
  const uninstalledRows = $derived(partitioned.uninstalled);

  async function loadLaunchConfigs() {
    launchOverridden = true;
    try {
      launchConfigs = await api("/launch-configs", LaunchConfigsSchema);
      launchError = "";
      launchState = "ready";
      void shell?.launchConfigs?.load();
    } catch (err) {
      if (fixture && err instanceof APIError && err.status === 404) {
        launchConfigs = [];
        launchError = "";
        launchState = "ready";
        return;
      }
      launchError = errorMessage(err);
      launchState = "unavailable";
    }
  }

  async function loadLaunchPresets() {
    launchOverridden = true;
    try {
      launchPresets = await api("/launch-configs/presets", LaunchPresetsSchema);
      launchPresetError = "";
    } catch (err) {
      launchPresets = [];
      launchPresetError = errorMessage(err);
    }
  }

  async function restoreLaunchPresets(name?: string) {
    if (launchPresetBusy) return;
    launchPresetBusy = true;
    launchPresetError = "";
    const previousIds = activeConfigs.map((item) => item.id);
    try {
      launchOverridden = true;
      launchConfigs = await api(
        "/launch-configs/presets",
        LaunchConfigsSchema,
        "POST",
        name ? { name } : {},
      );
      void shell?.launchConfigs?.load();
      const added = launchConfigs.filter(
        (item) => !previousIds.includes(item.id),
      );
      if (
        added.some((item) =>
          isLaunchConfigHiddenByAgentInstall(
            item,
            agentInspector.report?.tools,
          ),
        )
      ) {
        uninstalledOpen = true;
      }
    } catch (err) {
      launchPresetError = errorMessage(err);
    } finally {
      launchPresetBusy = false;
    }
  }

  function editLaunchConfig(id?: string) {
    launchEditing = id;
    launchDialogOpen = true;
  }

  async function handleConfigSaved(saved: LaunchConfig) {
    await loadLaunchConfigs();
    if (
      isLaunchConfigHiddenByAgentInstall(saved, agentInspector.report?.tools)
    ) {
      uninstalledOpen = true;
    }
  }

  async function deleteLaunchConfig() {
    if (!launchDeleting) return;
    launchDeleteBusy = true;
    launchDeleteError = "";
    try {
      await api(
        `/launch-configs/${launchDeleting.id}`,
        NoContentSchema,
        "DELETE",
      );
      launchDeleteOpen = false;
      await loadLaunchConfigs();
    } catch (err) {
      launchDeleteError = errorMessage(err);
    } finally {
      launchDeleteBusy = false;
    }
  }

  onMount(() => {
    if (!fixture) {
      void loadLaunchConfigs();
      void loadLaunchPresets();
      void agentInspector.load(false);
    }
  });
</script>

{#snippet launchCreateActions(
  variant: "default" | "outline",
  disabled: boolean,
)}
  <Button {variant} {disabled} onclick={() => editLaunchConfig()}
    ><IconPlus aria-hidden="true" />{m.s2_launch_new_title()}</Button
  >
  <LaunchPresetMenu
    presets={activePresets}
    configs={activeConfigs}
    busy={disabled || launchPresetBusy}
    onrestore={restoreLaunchPresets}
  />
{/snippet}

{#snippet launchEmptyCard(busy: boolean, title: string)}
  <Empty.Root variant="outline" aria-busy={busy ? "true" : undefined}>
    <Empty.Header>
      <Empty.Media variant="icon"><IconRocket aria-hidden="true" /></Empty.Media
      >
      <Empty.Title role={busy ? "status" : undefined}>{title}</Empty.Title>
      <Empty.Description>{m.s2_launch_empty_desc()}</Empty.Description>
    </Empty.Header>
    <Empty.Content>
      <div class="flex flex-wrap items-center justify-center gap-2">
        {@render launchCreateActions("default", busy)}
      </div>
    </Empty.Content>
  </Empty.Root>
{/snippet}

{#snippet mergedRowItem(row: MergedLaunchRow, compact = false)}
  {#if row.kind === "config"}
    <AgentToolRow
      config={row.config}
      tool={row.tool}
      showToolDetails={row.showToolDetails}
      {compact}
      settingsReady={settings.ready}
      onedit={() => editLaunchConfig(row.config.id)}
      ondelete={() => {
        launchDeleting = row.config;
        launchDeleteError = "";
        launchDeleteOpen = true;
      }}
    />
  {:else}
    <AgentToolRow tool={row.tool} {compact} />
  {/if}
{/snippet}

<Card.Root
  flat
  role="region"
  id="launch-configs"
  class="scroll-mt-4"
  aria-labelledby="launch-configs-title"
>
  <Card.Header>
    <Card.Title id="launch-configs-title">{m.s2_launch_title()}</Card.Title>
    <Card.Description
      ><span class="flex items-center gap-1"
        >{m.s2_launch_desc()}<InfoHint label={m.s2_launch_title()}
          >{m.s2_launch_hint()}</InfoHint
        ></span
      ></Card.Description
    >
  </Card.Header>
  <Card.Content class="space-y-4">
    {#if initialLoading}
      {@render launchEmptyCard(true, m.s2_launch_loading())}
    {:else if launchViewState === "unavailable"}
      <InlineNotice tone="danger">
        <p>{m.s2_launch_unavailable()}</p>
        <p class="mt-1 font-mono text-xs">{launchLoadError}</p>
      </InlineNotice>
      <Button
        variant="outline"
        onclick={() => {
          void loadLaunchConfigs();
          void loadLaunchPresets();
          void agentInspector.load(true);
        }}>{m.s2_reload()}</Button
      >
    {:else}
      <AgentToolsToolbar
        state={agentInspector.state}
        report={agentInspector.report}
        error={agentInspector.error}
        checkError={agentInspector.checkError}
        checking={agentInspector.checking}
        rescanning={agentInspector.rescanning}
        busy={agentInspector.busy}
        primaryAction={activeConfigs.length > 0 && activeRows.length > 0}
        onrescan={() => void agentInspector.load(true)}
        oncheck={() => void agentInspector.checkUpdates()}
      />

      {#if activeConfigs.length === 0}
        {@render launchEmptyCard(false, m.s2_launch_empty_title())}
        {#if activeRows.length > 0}
          <Item.Group
            variant="divided"
            aria-label={m.agent_tools_installed_list_aria()}
          >
            {#each activeRows as row (row.key)}
              {@render mergedRowItem(row)}
            {/each}
          </Item.Group>
        {/if}
      {:else if activeRows.length === 0}
        <Empty.Root variant="outline">
          <Empty.Header>
            <Empty.Media variant="icon"
              ><IconTerminal aria-hidden="true" /></Empty.Media
            >
            <Empty.Title>{m.agent_tools_empty_installed_title()}</Empty.Title>
            <Empty.Description
              >{m.agent_tools_empty_installed_desc()}</Empty.Description
            >
          </Empty.Header>
          <Empty.Content>
            <div class="flex flex-wrap items-center justify-center gap-2">
              {@render launchCreateActions("default", launchFull)}
            </div>
          </Empty.Content>
        </Empty.Root>
      {:else}
        <Item.Group variant="divided" aria-label={m.s2_launch_list_aria()}>
          {#each activeRows as row (row.key)}
            {@render mergedRowItem(row)}
          {/each}
        </Item.Group>
        <div class="flex flex-wrap items-center gap-3">
          {@render launchCreateActions("outline", launchFull)}
          {#if launchFull}<span class="text-xs text-muted-foreground"
              >{m.s2_launch_limit_hint({
                limit: String(LAUNCH_CONFIGS_LIMIT),
              })}</span
            >{/if}
        </div>
      {/if}

      {#if launchPresetError}
        <InlineNotice tone="danger">
          {activePresets.length === 0
            ? m.s2_launch_presets_unavailable({ error: launchPresetError })
            : m.s2_launch_presets_restore_err({ error: launchPresetError })}
          {#snippet actions()}
            {#if activePresets.length === 0}
              <Button
                variant="outline"
                size="sm"
                onclick={() => void loadLaunchPresets()}
                >{m.s2_launch_presets_reload()}</Button
              >
            {/if}
          {/snippet}
        </InlineNotice>
      {/if}

      {#if uninstalledRows.length > 0}
        <AdvancedSection
          title={m.agent_tools_uninstalled_toggle({
            count: String(uninstalledRows.length),
          })}
          bind:open={uninstalledOpen}
        >
          <Item.Group
            variant="divided"
            aria-label={m.agent_tools_uninstalled_list_aria()}
          >
            {#each uninstalledRows as row (row.key)}
              {@render mergedRowItem(row, true)}
            {/each}
          </Item.Group>
        </AdvancedSection>
      {/if}
    {/if}
  </Card.Content>
</Card.Root>

<LaunchConfigDialog
  bind:open={launchDialogOpen}
  configId={launchEditing}
  {defaultShell}
  onsaved={(saved) => void handleConfigSaved(saved)}
/>
<AlertDialog.Root bind:open={launchDeleteOpen}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{m.s2_launch_delete_title()}</AlertDialog.Title>
      <AlertDialog.Description
        >{m.s2_launch_delete_desc({
          name: launchDeleting?.name ?? "",
        })}</AlertDialog.Description
      >
    </AlertDialog.Header>
    {#if launchDeleting}<p
        class="overflow-x-auto rounded-md bg-muted p-3 font-mono text-xs"
      >
        {joinArgs(launchDeleting.cmd)}
      </p>{/if}
    {#if launchDeleteError}<InlineNotice tone="danger"
        >{launchDeleteError}</InlineNotice
      >{/if}
    <AlertDialog.Footer>
      <AlertDialog.Cancel disabled={launchDeleteBusy}
        >{m.s2_cancel()}</AlertDialog.Cancel
      >
      <Button
        variant="destructive"
        disabled={launchDeleteBusy}
        onclick={deleteLaunchConfig}>{m.s2_delete()}</Button
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
