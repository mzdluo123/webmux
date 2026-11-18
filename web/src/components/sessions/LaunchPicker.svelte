<script lang="ts">
  import { tick } from "svelte";
  import { useDockMotion } from "../../hooks/shared/dock-motion.svelte";
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import { joinArgs } from "../../platform/api";
  import {
    featureAvailable,
    featureEnabled,
  } from "../../platform/feature-host.svelte";
  import { getFeatureContributions } from "../../features/context";
  import {
    launchConfigIcon,
    type LaunchConfigsState,
  } from "../../domains/settings/launch-configs";
  import type { LaunchSource } from "../../domains/sessions/new-session-defaults";
  import type { SessionView } from "../../contracts/schemas";
  import * as Command from "$lib/components/ui/command";
  import { m } from "$lib/paraglide/messages.js";
  import CustomCommandField from "./CustomCommandField.svelte";
  import { Label } from "$lib/components/ui/label";
  import ChoiceDetails from "../shared/ChoiceDetails.svelte";
  import LaunchConfigIcon from "../settings/LaunchConfigIcon.svelte";
  import { PickerField } from "$lib/components/ui/picker-field";

  // The "what" row of the new-session dialog: saved launch configs and a
  // custom command in one list. Switching keeps both the chosen config and
  // the typed command; only the current source is submitted.
  let {
    launchConfigs,
    source = $bindable(),
    configId = $bindable(),
    command = $bindable(),
    startSide = $bindable(),
    disabled = false,
    onchoose,
  }: {
    launchConfigs: LaunchConfigsState;
    source: LaunchSource;
    configId: string;
    command: string;
    startSide: SessionView;
    disabled?: boolean;
    /** The user picked a source; defaults must no longer replace it. */
    onchoose: () => void;
  } = $props();
  let open = $state(false);
  let commandPanel = $state<HTMLDivElement | null>(null);
  let commandChange = $state(0);
  useDockMotion(
    () => commandPanel,
    () => true,
    () => String(commandChange),
  );
  const providers = getFeatureContributions().creation;
  const chatEnabled = $derived(
    featureAvailable("chat.acp") && !!providers.LaunchStartSideFields,
  );
  let search = $state("");
  // The desktop list stays mounted while closed; each opening starts unfiltered.
  function setOpen(next: boolean) {
    open = next;
    if (!next) search = "";
  }
  const configs = $derived(
    launchConfigs.status === "ready" ? launchConfigs.configs : [],
  );
  const chosen = $derived(
    source === "config"
      ? configs.find((config) => config.id === configId)
      : undefined,
  );

  async function choose(next: LaunchSource, id = configId) {
    if ((source === "custom") !== (next === "custom")) commandChange++;
    source = next;
    configId = id;
    setOpen(false);
    onchoose();
    await tick();
    document.getElementById("session-launch")?.focus({ preventScroll: true });
  }
</script>

<div>
  <div class="space-y-2">
    <Label for="session-launch">{m.s3_launch_run_label()}</Label>
    <PickerField id="session-launch" bind:open={() => open, setOpen} {disabled}>
      {#snippet trigger()}
        {#if chosen}
          <LaunchConfigIcon icon={launchConfigIcon(chosen)} />
          <ChoiceDetails
            primary={chosen.name}
            secondary={joinArgs(chosen.cmd)}
            envCount={chosen.envNames.length}
          />
        {:else if source === "custom"}
          <IconTerminal2
            class="size-4 shrink-0 text-muted-foreground"
            aria-hidden="true"
          />
          <ChoiceDetails primary={m.s3_custom_command()} />
        {:else if launchConfigs.status === "loading"}
          <span class="text-sm text-muted-foreground"
            >{m.s3_loading_launch_configs()}</span
          >
        {:else}
          <span class="text-sm text-muted-foreground"
            >{m.s3_select_launch_method()}</span
          >
        {/if}
      {/snippet}
      <Command.Root label={m.s3_launch_method_label()} class="h-auto min-w-0">
        <Command.Input
          bind:value={search}
          placeholder={m.s3_search_launch_configs_placeholder()}
          aria-label={m.s3_search_launch_method_label()}
          autocomplete="off"
          autocapitalize="off"
          autocorrect="off"
          spellcheck={false}
        />
        <Command.List
          aria-label={m.s3_launch_method_list_label()}
          class="max-h-[min(20rem,50dvh)] p-1"
        >
          <Command.Empty>{m.s3_no_matching_launch_configs()}</Command.Empty>
          <Command.Group
            heading={m.s3_launch_configs_heading()}
            value="configs"
          >
            {#if launchConfigs.status === "loading"}
              <p
                role="status"
                class="px-2 py-1.5 text-xs text-muted-foreground"
              >
                {m.s3_loading_launch_configs()}
              </p>
            {:else if launchConfigs.status === "unavailable"}
              <p class="px-2 py-1.5 text-xs text-destructive wrap-anywhere">
                {m.s3_launch_configs_unavailable({
                  error: launchConfigs.error,
                })}
              </p>
            {:else if !configs.length}
              <p class="px-2 py-1.5 text-xs text-muted-foreground">
                {m.s3_no_launch_configs_yet()}<a
                  class="text-foreground underline underline-offset-2"
                  href="/settings#launch-configs"
                  >{m.s3_go_settings_create_launch()}</a
                >
              </p>
            {/if}
            {#each configs as config (config.id)}
              <Command.Item
                value="config:{config.id}"
                keywords={[config.name, joinArgs(config.cmd)]}
                data-checked={chosen?.id === config.id}
                onSelect={() => choose("config", config.id)}
                disabled={config.chat && !chatEnabled}
              >
                <LaunchConfigIcon icon={launchConfigIcon(config)} />
                <ChoiceDetails
                  primary={config.name}
                  secondary={joinArgs(config.cmd)}
                  envCount={config.envNames.length}
                />
                {#if config.chat && !chatEnabled}
                  <span class="text-xs text-muted-foreground"
                    >{featureEnabled("chat")
                      ? m.s3_chat_temporarily_unavailable()
                      : m.s3_chat_feature_disabled()}</span
                  >
                {/if}
              </Command.Item>
            {/each}
          </Command.Group>
          <Command.Group heading={m.s3_other_heading()} value="other">
            <Command.Item
              value="custom"
              keywords={[m.s3_custom_command(), "custom"]}
              data-checked={source === "custom"}
              onSelect={() => choose("custom")}
            >
              <IconTerminal2 class="text-muted-foreground" aria-hidden="true" />
              <span class="text-sm">{m.s3_custom_command_ellipsis()}</span>
            </Command.Item>
          </Command.Group>
        </Command.List>
      </Command.Root>
    </PickerField>
    {#if source === "unset" && launchConfigs.status === "unavailable"}
      <p role="alert" class="text-sm text-destructive">
        {m.s3_launch_store_unavailable({ error: launchConfigs.error })}
      </p>
    {:else if source === "unset" && launchConfigs.status === "ready"}
      <p role="status" class="text-sm text-muted-foreground">
        {m.s3_launch_config_deleted_reselect()}
      </p>
    {/if}
  </div>
  <div bind:this={commandPanel} class="flow-root">
    {#if source === "custom"}
      <div class="pt-2"><CustomCommandField bind:command {disabled} /></div>
    {:else if chosen?.chat && chatEnabled && providers.LaunchStartSideFields}
      <div class="pt-2">
        <providers.LaunchStartSideFields
          bind:startSide
          support={chosen.chatSupport}
          {disabled}
        />
      </div>
    {/if}
  </div>
</div>
