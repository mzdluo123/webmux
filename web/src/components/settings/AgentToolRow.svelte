<script lang="ts">
  import { onDestroy } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import IconCheck from "~icons/tabler/check";
  import IconCopy from "~icons/tabler/copy";
  import IconExternalLink from "~icons/tabler/external-link";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Item from "$lib/components/ui/item";
  import LaunchConfigIcon from "./LaunchConfigIcon.svelte";
  import { copyText } from "../../platform/clipboard";
  import type {
    AgentConnectorStatus,
    AgentToolStatus,
    AgentUpdateStatus,
  } from "../../contracts/schemas";

  let {
    tool,
    compact = false,
  }: {
    tool: AgentToolStatus;
    compact?: boolean;
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

  const showToolCode = $derived(
    compact
      ? Boolean(tool.installCommand)
      : !tool.installed ||
          tool.updateStatus === "update_available" ||
          Boolean(tool.belowVerified),
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
    size="icon-touch-sm"
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

{#snippet commandInline(
  key: string,
  title: string,
  entry: AgentToolStatus | AgentConnectorStatus,
  showCodePill: boolean,
)}
  {@const action = primaryCommand(entry)}
  {#if action && showCodePill}
    <div class="mt-1.5 flex min-w-0 items-center gap-1.5">
      <code
        title={action.command}
        class="min-w-0 flex-1 truncate rounded-sm bg-muted px-2 py-1 font-mono text-xs text-foreground select-all"
        >{action.command}</code
      >
      {@render copyIconButton(key, title, action.command, action.isUpgrade)}
    </div>
  {/if}
{/snippet}

{#snippet rowTrailingActions(
  key: string,
  title: string,
  entry: AgentToolStatus | AgentConnectorStatus,
  hasCodePill: boolean,
)}
  {@const action = primaryCommand(entry)}
  <div class="flex shrink-0 items-center gap-0.5">
    {#if action && !hasCodePill}
      {@render copyIconButton(key, title, action.command, action.isUpgrade)}
    {/if}
    {#if entry.docsUrl}
      <Button
        variant="ghost"
        size="icon-touch-sm"
        href={entry.docsUrl}
        target="_blank"
        rel="noreferrer noopener"
        aria-label={m.agent_tools_docs_aria({ title })}
        title={m.agent_tools_docs_aria({ title })}
      >
        <IconExternalLink aria-hidden="true" />
      </Button>
    {/if}
  </div>
{/snippet}

<Item.Root variant="divided">
  <div class="min-w-0 flex-1 space-y-1.5">
    <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
      <div class="flex min-w-0 flex-wrap items-center gap-2">
        <span
          class={compact
            ? "grid size-8 shrink-0 place-items-center rounded-md bg-muted/60 opacity-75"
            : "grid size-8 shrink-0 place-items-center rounded-md bg-muted/60"}
        >
          <LaunchConfigIcon icon={tool.icon ?? "terminal"} size="sm" />
        </span>
        <span class="text-sm font-medium text-foreground">{tool.title}</span>
        {#if compact}
          <span class="font-mono text-xs text-muted-foreground"
            >{tool.executable}</span
          >
        {/if}
        <Badge variant="outline"
          >{tool.kind === "hybrid"
            ? m.agent_tools_kind_hybrid()
            : m.agent_tools_kind_terminal()}</Badge
        >
        {#if !compact}
          {#if tool.installed}
            <span class="font-mono text-xs text-foreground"
              >{tool.installedVersion || m.agent_tools_version_unknown()}</span
            >
          {:else}
            <Badge variant="warning"
              >{m.agent_tools_status_not_installed()}</Badge
            >
          {/if}
          {@render statusDisplay(tool)}
          {#if tool.installed && tool.connector && !tool.connector.installed}
            <Badge variant="warning"
              >{m.agent_tools_connector_missing_badge()}</Badge
            >
          {/if}
        {/if}
      </div>
      {@render rowTrailingActions(
        `tool:${tool.id}`,
        tool.title,
        tool,
        showToolCode,
      )}
    </div>

    <div class="min-w-0 space-y-1.5 sm:pl-10">
      {#if !compact && tool.path}
        <p
          title={tool.path}
          class="truncate font-mono text-xs text-muted-foreground"
        >
          {tool.path}
        </p>
      {/if}

      {@render commandInline(`tool:${tool.id}`, tool.title, tool, showToolCode)}

      {#if !compact && tool.connector}
        {@const conn = tool.connector}
        {@const showConnCode =
          !conn.installed ||
          conn.updateStatus === "update_available" ||
          Boolean(conn.belowVerified)}
        <div
          class="mt-2 rounded-md border border-border/70 bg-muted/30 px-2.5 py-2"
        >
          <div
            class="flex min-w-0 flex-wrap items-center justify-between gap-2"
          >
            <div class="flex min-w-0 flex-wrap items-center gap-2">
              <span class="text-xs font-medium text-foreground wrap-anywhere"
                >{m.agent_tools_connector_label({
                  executable: conn.executable,
                })}</span
              >
              {#if conn.installed}
                <span class="font-mono text-xs text-foreground"
                  >{conn.installedVersion ||
                    m.agent_tools_version_unknown()}</span
                >
                {@render statusDisplay(conn)}
              {:else}
                <Badge variant="warning"
                  >{m.agent_tools_status_not_installed()}</Badge
                >
              {/if}
            </div>
            {@render rowTrailingActions(
              `conn:${tool.id}`,
              conn.title,
              conn,
              showConnCode,
            )}
          </div>
          {#if conn.path}
            <p
              title={conn.path}
              class="mt-1 truncate font-mono text-xs text-muted-foreground"
            >
              {conn.path}
            </p>
          {/if}
          {@render commandInline(
            `conn:${tool.id}`,
            conn.title,
            conn,
            showConnCode,
          )}
        </div>
      {/if}
    </div>
  </div>
</Item.Root>
