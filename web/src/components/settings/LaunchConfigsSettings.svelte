<script lang="ts">
  import { onMount } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { settings } from "../../domains/settings/settings.svelte";
  import { errorMessage } from "../../platform/error";
  import IconPlus from "~icons/tabler/plus";
  import IconRocket from "~icons/tabler/rocket";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import { InfoHint } from "$lib/components/ui/info-hint";
  import SettingsRowMenu from "./SettingsRowMenu.svelte";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import * as Card from "$lib/components/ui/card";
  import * as Empty from "$lib/components/ui/empty";
  import * as Item from "$lib/components/ui/item";
  import { api, joinArgs } from "../../platform/api";
  import {
    LaunchConfigsSchema,
    LaunchPresetsSchema,
    NoContentSchema,
    type LaunchConfigSummary,
    type LaunchPreset,
  } from "../../contracts/schemas";
  import {
    LAUNCH_CONFIGS_LIMIT,
    launchConfigIcon,
    launchConfigSummary,
    type StartupShell,
  } from "../../domains/settings/launch-configs";
  import LaunchConfigIcon from "./LaunchConfigIcon.svelte";
  import LaunchPresetMenu from "./LaunchPresetMenu.svelte";
  import LaunchConfigDialog from "./LaunchConfigDialog.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";

  let { defaultShell = "" }: { defaultShell?: "" | StartupShell } = $props();

  // Launch configs live on the server; a load failure (for example 503 when
  // the state file is corrupt or locked) only disables this card.
  let launchConfigs = $state<LaunchConfigSummary[]>([]);
  let launchPresets = $state<LaunchPreset[]>([]);
  let launchState = $state<"loading" | "ready" | "unavailable">("loading");
  let launchError = $state("");
  let launchPresetBusy = $state(false);
  let launchPresetError = $state("");
  let launchDialogOpen = $state(false);
  let launchEditing = $state<string>();
  let launchDeleting = $state<LaunchConfigSummary>();
  let launchDeleteOpen = $state(false);
  let launchDeleteBusy = $state(false);
  let launchDeleteError = $state("");
  const launchFull = $derived(launchConfigs.length >= LAUNCH_CONFIGS_LIMIT);

  async function loadLaunchConfigs() {
    try {
      launchConfigs = await api("/launch-configs", LaunchConfigsSchema);
      launchError = "";
      launchState = "ready";
    } catch (err) {
      launchError = errorMessage(err);
      launchState = "unavailable";
    }
  }

  async function loadLaunchPresets() {
    try {
      launchPresets = await api("/launch-configs/presets", LaunchPresetsSchema);
      launchPresetError = "";
    } catch (err) {
      // An empty list disables the menu; keep the reason so the card can
      // explain why and offer a retry.
      launchPresets = [];
      launchPresetError = errorMessage(err);
    }
  }

  async function restoreLaunchPresets(name?: string) {
    if (launchPresetBusy) return;
    launchPresetBusy = true;
    launchPresetError = "";
    try {
      launchConfigs = await api(
        "/launch-configs/presets",
        LaunchConfigsSchema,
        "POST",
        name ? { name } : {},
      );
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
    void loadLaunchConfigs();
    void loadLaunchPresets();
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
    presets={launchPresets}
    configs={launchConfigs}
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
    {#if launchState === "loading"}
      <!-- Same shape as the empty state, the usual first result, so the
         cards below do not shift when the list arrives. -->
      {@render launchEmptyCard(true, m.s2_launch_loading())}
    {:else if launchState === "unavailable"}
      <InlineNotice tone="danger">
        <p>{m.s2_launch_unavailable()}</p>
        <p class="mt-1 font-mono text-xs">{launchError}</p>
      </InlineNotice>
      <Button
        variant="outline"
        onclick={() => {
          void loadLaunchConfigs();
          void loadLaunchPresets();
        }}>{m.s2_reload()}</Button
      >
    {:else if launchConfigs.length === 0}
      {@render launchEmptyCard(false, m.s2_launch_empty_title())}
    {:else}
      <Item.Group variant="divided" aria-label={m.s2_launch_list_aria()}>
        {#each launchConfigs as config (config.id)}
          <Item.Root variant="divided" size="flush" class="min-h-14">
            <ListButton
              variant="content"
              class="flex min-w-0 flex-1 items-center"
              aria-label={m.s2_edit_item({ name: config.name })}
              onclick={() => editLaunchConfig(config.id)}
            >
              <span class="flex min-w-0 flex-1 items-center gap-3">
                <span
                  class="grid size-9 shrink-0 place-items-center rounded-md bg-muted/60"
                  ><LaunchConfigIcon icon={launchConfigIcon(config)} /></span
                >
                <span class="grid min-w-0 flex-1 gap-0.5">
                  <span class="truncate text-sm font-medium">{config.name}</span
                  >
                  <span class="truncate text-xs text-muted-foreground"
                    >{launchConfigSummary(config)}</span
                  >
                </span>
              </span>
            </ListButton>
            <SettingsRowMenu
              label={config.name}
              disabled={!settings.ready}
              ondelete={() => {
                launchDeleting = config;
                launchDeleteError = "";
                launchDeleteOpen = true;
              }}
            />
          </Item.Root>
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
    {#if launchPresetError && launchState !== "unavailable"}
      <InlineNotice tone="danger">
        <p>
          {launchPresets.length === 0
            ? m.s2_launch_presets_unavailable({ error: launchPresetError })
            : m.s2_launch_presets_restore_err({ error: launchPresetError })}
        </p>
        {#if launchPresets.length === 0}
          <Button
            variant="outline"
            size="sm"
            class="mt-2"
            onclick={() => void loadLaunchPresets()}
            >{m.s2_launch_presets_reload()}</Button
          >
        {/if}
      </InlineNotice>
    {/if}
  </Card.Content>
</Card.Root>

<LaunchConfigDialog
  bind:open={launchDialogOpen}
  configId={launchEditing}
  {defaultShell}
  onsaved={() => void loadLaunchConfigs()}
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
