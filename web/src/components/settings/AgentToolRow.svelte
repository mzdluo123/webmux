<script lang="ts">
  import { onDestroy } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import IconCheck from "~icons/tabler/check";
  import IconCopy from "~icons/tabler/copy";
  import IconExternalLink from "~icons/tabler/external-link";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import * as Item from "$lib/components/ui/item";
  import { ListButton } from "$lib/components/ui/list-button";
  import {
    launchConfigIcon,
    launchConfigSummary,
  } from "../../domains/settings/launch-configs";
  import { copyText } from "../../platform/clipboard";
  import type {
    AgentConnectorStatus,
    AgentToolStatus,
    AgentUpdateStatus,
    LaunchConfigSummary,
  } from "../../contracts/schemas";
  import LaunchConfigIcon from "./LaunchConfigIcon.svelte";
  import SettingsRowMenu from "./SettingsRowMenu.svelte";

  let {
    config,
    tool,
    showToolDetails = true,
    compact = false,
    settingsReady = true,
    onedit,
    ondelete,
  }: {
    config?: LaunchConfigSummary;
    tool?: AgentToolStatus;
    showToolDetails?: boolean;
    compact?: boolean;
    settingsReady?: boolean;
    onedit?: () => void;
    ondelete?: () => void;
  } = $props();

  let copiedKey = $state<string | null>(null);
  let failedKey = $state<string | null>(null);
  let copyTimer: ReturnType<typeof setTimeout> | undefined;

  async function handleCopy(key: string, command: string) {
    const ok = await copyText(command);
    clearTimeout(copyTimer);
    if (ok) {
      copiedKey = key;
      failedKey = null;
      copyTimer = setTimeout(() => {
        copiedKey = null;
      }, 2000);
    } else {
      copiedKey = null;
      failedKey = key;
      copyTimer = setTimeout(() => {
        failedKey = null;
      }, 2500);
    }
  }

  function copyActionLabel(
    key: string,
    title: string,
    isUpgrade: boolean,
  ): string {
    if (copiedKey === key) return m.agent_tools_copied_cmd();
    if (failedKey === key) return m.s3_copy_failed_manual();
    return isUpgrade
      ? m.agent_tools_copy_upgrade({ title })
      : m.agent_tools_copy_install({ title });
  }

  function primaryCommand(
    entry: AgentToolStatus | AgentConnectorStatus,
  ): { command: string; isUpgrade: boolean } | null {
    if (!entry.installed) {
      return entry.installCommand
        ? { command: entry.installCommand, isUpgrade: false }
        : null;
    }
    if (entry.upgradeCommand) {
      return { command: entry.upgradeCommand, isUpgrade: true };
    }
    return null;
  }

  function statusText(
    status: AgentUpdateStatus,
    latestVersion: string | undefined,
  ): string {
    switch (status) {
      case "update_available":
        return m.agent_tools_status_update_available({
          latest: latestVersion ?? "",
        });
      case "up_to_date":
        return m.agent_tools_status_up_to_date();
      case "source_unavailable":
        return m.agent_tools_status_source_unavailable();
      case "check_failed":
        return m.agent_tools_status_check_failed();
      case "not_installed":
        return m.agent_tools_status_not_installed();
      default:
        return m.agent_tools_status_unchecked();
    }
  }

  const iconKey = $derived(
    config ? launchConfigIcon(config) : (tool?.icon ?? "terminal"),
  );
  const showToolCode = $derived(
    Boolean(
      tool &&
      showToolDetails &&
      (compact
        ? Boolean(tool.installCommand)
        : !tool.installed ||
          tool.updateStatus === "update_available" ||
          Boolean(tool.belowVerified)),
    ),
  );
  const showConnector = $derived(
    Boolean(tool && showToolDetails && !compact && tool.connector),
  );
  const hasToolBadges = $derived(
    Boolean(tool && (showToolDetails || !tool.installed)),
  );

  onDestroy(() => {
    clearTimeout(copyTimer);
  });
</script>

{#snippet statusDisplay(entry: AgentToolStatus | AgentConnectorStatus)}
  {#if entry.updateStatus === "update_available"}
    <Badge variant="info"
      >{statusText(entry.updateStatus, entry.latestVersion)}</Badge
    >
  {:else if entry.updateStatus === "check_failed"}
    <Badge variant="warning" title={entry.checkError}
      >{statusText(entry.updateStatus, entry.latestVersion)}</Badge
    >
  {:else if entry.installed}
    <span class="text-xs text-muted-foreground"
      >{statusText(entry.updateStatus, entry.latestVersion)}</span
    >
  {/if}
  {#if entry.belowVerified && entry.verifiedVersion}
    <Badge
      variant="warning"
      title={m.agent_tools_below_verified_hint({
        installed: entry.installedVersion ?? "",
        verified: entry.verifiedVersion,
      })}
      >{m.agent_tools_below_verified({
        verified: entry.verifiedVersion,
      })}</Badge
    >
  {/if}
{/snippet}

{#snippet copyIconButton(
  key: string,
  title: string,
  command: string,
  isUpgrade: boolean,
)}
  <Button
    variant="ghost"
    size="icon"
    aria-label={copyActionLabel(key, title, isUpgrade)}
    title={copyActionLabel(key, title, isUpgrade)}
    onclick={() => void handleCopy(key, command)}
  >
    {#if copiedKey === key}
      <IconCheck aria-hidden="true" />
    {:else}
      <IconCopy aria-hidden="true" />
    {/if}
  </Button>
{/snippet}

{#snippet docsIconButton(title: string, docsUrl: string)}
  <Button
    variant="ghost"
    size="icon"
    href={docsUrl}
    target="_blank"
    rel="noreferrer noopener"
    aria-label={m.agent_tools_docs_aria({ title })}
    title={m.agent_tools_docs_aria({ title })}
  >
    <IconExternalLink aria-hidden="true" />
  </Button>
{/snippet}

{#snippet commandInline(
  key: string,
  title: string,
  entry: AgentToolStatus | AgentConnectorStatus,
  showCodePill: boolean,
)}
  {@const action = primaryCommand(entry)}
  {#if action && showCodePill}
    <div class="flex min-w-0 items-center gap-1.5">
      <code
        title={action.command}
        class="min-w-0 flex-1 truncate rounded-sm bg-muted px-2 py-1 font-mono text-xs text-foreground select-all"
        >{action.command}</code
      >
      {@render copyIconButton(key, title, action.command, action.isUpgrade)}
    </div>
  {/if}
{/snippet}

{#snippet toolMetaBadges(entry: AgentToolStatus)}
  {#if compact}
    <span class="font-mono text-xs text-muted-foreground"
      >{entry.executable}</span
    >
  {/if}
  {#if !config}
    <Badge variant="outline"
      >{entry.kind === "hybrid"
        ? m.agent_tools_kind_hybrid()
        : m.agent_tools_kind_terminal()}</Badge
    >
  {/if}
  {#if entry.installed}
    <span class="font-mono text-xs text-foreground"
      >{entry.installedVersion || m.agent_tools_version_unknown()}</span
    >
    {@render statusDisplay(entry)}
    {#if entry.connector && !entry.connector.installed}
      <Badge variant="warning">{m.agent_tools_connector_missing_badge()}</Badge>
    {/if}
  {:else if config}
    <Badge variant="outline">{m.s2_launch_hidden_uninstalled()}</Badge>
  {:else if !compact}
    <Badge variant="warning">{m.agent_tools_status_not_installed()}</Badge>
  {/if}
{/snippet}

{#snippet connectorCard(parentId: string, conn: AgentConnectorStatus)}
  {@const showConnCode =
    !conn.installed ||
    conn.updateStatus === "update_available" ||
    Boolean(conn.belowVerified)}
  {@const connAction = primaryCommand(conn)}
  <div
    class="rounded-md border border-border/70 bg-muted/30 px-2.5 py-2 space-y-1.5"
  >
    <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
      <div class="flex min-w-0 flex-wrap items-center gap-2">
        <span class="text-xs font-medium text-foreground wrap-anywhere"
          >{m.agent_tools_connector_label({
            executable: conn.executable,
          })}</span
        >
        {#if conn.installed}
          <span class="font-mono text-xs text-foreground"
            >{conn.installedVersion || m.agent_tools_version_unknown()}</span
          >
          {@render statusDisplay(conn)}
        {:else}
          <Badge variant="warning">{m.agent_tools_status_not_installed()}</Badge
          >
        {/if}
      </div>
      <div class="flex shrink-0 items-center gap-0.5">
        {#if connAction && !showConnCode}
          {@render copyIconButton(
            `conn:${parentId}`,
            conn.title,
            connAction.command,
            connAction.isUpgrade,
          )}
        {/if}
        {#if conn.docsUrl}
          {@render docsIconButton(conn.title, conn.docsUrl)}
        {/if}
      </div>
    </div>
    {#if conn.path}
      <p
        title={conn.path}
        class="truncate font-mono text-xs text-muted-foreground"
      >
        {conn.path}
      </p>
    {/if}
    {@render commandInline(`conn:${parentId}`, conn.title, conn, showConnCode)}
  </div>
{/snippet}

<Item.Root variant="divided" size="flush" class="min-h-14">
  {#if config}
    {@const toolAction = tool && showToolDetails ? primaryCommand(tool) : null}
    <div
      class={hasToolBadges
        ? "grid w-full min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-2 gap-y-1.5 py-2.5 sm:grid-cols-[minmax(11rem,1fr)_minmax(0,auto)_auto]"
        : "grid w-full min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-2 py-2.5"}
    >
      <ListButton
        variant="content"
        class="flex min-w-0 items-center"
        aria-label={m.s2_edit_item({ name: config.name })}
        onclick={onedit}
      >
        <span class="flex min-w-0 flex-1 items-center gap-3">
          <span
            class={compact
              ? "grid size-9 shrink-0 place-items-center rounded-md bg-muted/60 opacity-75"
              : "grid size-9 shrink-0 place-items-center rounded-md bg-muted/60"}
          >
            <LaunchConfigIcon icon={iconKey} />
          </span>
          <span class="flex min-w-0 flex-1 flex-col gap-0.5">
            <span class="truncate text-sm font-medium text-foreground"
              >{config.name}</span
            >
            <span
              class="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground"
            >
              <span class="min-w-0 truncate">{launchConfigSummary(config)}</span
              >
              {#if !compact && showToolDetails && tool?.path}
                <span aria-hidden="true" class="shrink-0">·</span>
                <span
                  title={tool.path}
                  class="min-w-0 flex-1 truncate font-mono">{tool.path}</span
                >
              {/if}
            </span>
          </span>
        </span>
      </ListButton>
      <div class="flex shrink-0 items-center gap-0.5 sm:order-3">
        {#if tool && toolAction && !showToolCode}
          {@render copyIconButton(
            `tool:${tool.id}`,
            tool.title,
            toolAction.command,
            toolAction.isUpgrade,
          )}
        {/if}
        <SettingsRowMenu
          label={config.name}
          disabled={!settingsReady}
          {ondelete}
        >
          {#if tool?.docsUrl}
            {@const docsUrl = tool.docsUrl}
            {@const docsTitle = tool.title}
            <DropdownMenu.Item
              onSelect={() => {
                window.open(docsUrl, "_blank", "noopener,noreferrer");
              }}
            >
              <IconExternalLink aria-hidden="true" />
              {m.agent_tools_docs_aria({ title: docsTitle })}
            </DropdownMenu.Item>
          {/if}
        </SettingsRowMenu>
      </div>
      {#if tool && hasToolBadges}
        <div
          class="col-span-2 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 sm:order-2 sm:col-span-1 sm:justify-end"
        >
          {#if showToolDetails}
            {@render toolMetaBadges(tool)}
          {:else if !tool.installed}
            <Badge variant="outline">{m.s2_launch_hidden_uninstalled()}</Badge>
          {/if}
        </div>
      {/if}
    </div>
  {:else if tool}
    {@const toolAction = primaryCommand(tool)}
    <div
      class="grid w-full min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-2 gap-y-1.5 py-2.5 sm:grid-cols-[minmax(11rem,1fr)_minmax(0,auto)_auto]"
    >
      <div class="flex min-w-0 items-center gap-3">
        <span
          class={compact
            ? "grid size-9 shrink-0 place-items-center rounded-md bg-muted/60 opacity-75"
            : "grid size-9 shrink-0 place-items-center rounded-md bg-muted/60"}
        >
          <LaunchConfigIcon icon={iconKey} />
        </span>
        <div class="flex min-w-0 flex-1 flex-col gap-0.5">
          <span class="truncate text-sm font-medium text-foreground"
            >{tool.title}</span
          >
          {#if !compact && tool.path}
            <p
              title={tool.path}
              class="truncate font-mono text-xs text-muted-foreground"
            >
              {tool.path}
            </p>
          {/if}
        </div>
      </div>
      <div class="flex shrink-0 items-center gap-0.5 sm:order-3">
        {#if toolAction && !showToolCode}
          {@render copyIconButton(
            `tool:${tool.id}`,
            tool.title,
            toolAction.command,
            toolAction.isUpgrade,
          )}
        {/if}
        {#if tool.docsUrl}
          {@render docsIconButton(tool.title, tool.docsUrl)}
        {/if}
      </div>
      <div
        class="col-span-2 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 sm:order-2 sm:col-span-1 sm:justify-end"
      >
        {@render toolMetaBadges(tool)}
      </div>
    </div>
  {/if}

  {#if tool && (showToolCode || showConnector)}
    <div class="w-full min-w-0 space-y-2 pb-2.5 sm:pl-12">
      {@render commandInline(`tool:${tool.id}`, tool.title, tool, showToolCode)}
      {#if showConnector && tool.connector}
        {@render connectorCard(tool.id, tool.connector)}
      {/if}
    </div>
  {/if}
</Item.Root>
