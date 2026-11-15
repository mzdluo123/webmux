<script lang="ts">
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import { joinArgs } from "../../platform/api";
  import {
    pluginAvailable,
    pluginEnabled,
  } from "../../platform/plugin-host.svelte";
  import { getPluginContributions } from "../../plugins/context";
  import {
    launchConfigIcon,
    type LaunchConfigsState,
  } from "../../domains/settings/launch-configs";
  import type { LaunchSource } from "../../domains/sessions/new-session-defaults";
  import type { SessionView } from "../../contracts/schemas";
  import * as Command from "$lib/components/ui/command";
  import CustomCommandField from "./CustomCommandField.svelte";
  import { Label } from "$lib/components/ui/label";
  import ChoiceDetails from "../shared/ChoiceDetails.svelte";
  import LaunchConfigIcon from "../settings/LaunchConfigIcon.svelte";
  import PickerField from "../shared/PickerField.svelte";

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
  const providers = getPluginContributions().creation;
  const chatEnabled = $derived(
    pluginAvailable("chat.acp") && !!providers.LaunchStartSideFields,
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

  function choose(next: LaunchSource, id = configId) {
    source = next;
    configId = id;
    setOpen(false);
    onchoose();
  }
</script>

<div class="space-y-2">
  <Label for="session-launch">运行</Label>
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
        <ChoiceDetails primary="自定义命令" />
      {:else if launchConfigs.status === "loading"}
        <span class="text-sm text-muted-foreground">正在读取启动配置…</span>
      {:else}
        <span class="text-sm text-muted-foreground">选择启动方式</span>
      {/if}
    {/snippet}
    <Command.Root label="启动方式" class="h-auto min-w-0">
      <Command.Input
        bind:value={search}
        placeholder="搜索启动配置…"
        aria-label="搜索启动方式"
        autocomplete="off"
        autocapitalize="off"
        autocorrect="off"
        spellcheck={false}
      />
      <Command.List
        aria-label="启动方式列表"
        class="max-h-[min(20rem,50dvh)] p-1"
      >
        <Command.Empty>没有匹配的启动配置</Command.Empty>
        <Command.Group heading="启动配置" value="configs">
          {#if launchConfigs.status === "loading"}
            <p role="status" class="px-2 py-1.5 text-xs text-muted-foreground">
              正在读取启动配置…
            </p>
          {:else if launchConfigs.status === "unavailable"}
            <p class="px-2 py-1.5 text-xs text-destructive wrap-anywhere">
              启动配置暂不可用：{launchConfigs.error}
            </p>
          {:else if !configs.length}
            <p class="px-2 py-1.5 text-xs text-muted-foreground">
              还没有启动配置。<a
                class="text-foreground underline underline-offset-2"
                href="/settings#launch-configs">前往设置创建启动配置</a
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
                  >{pluginEnabled("chat")
                    ? "聊天功能暂不可用"
                    : "聊天插件未启用"}</span
                >
              {/if}
            </Command.Item>
          {/each}
        </Command.Group>
        <Command.Group heading="其他" value="other">
          <Command.Item
            value="custom"
            keywords={["自定义命令", "custom"]}
            data-checked={source === "custom"}
            onSelect={() => choose("custom")}
          >
            <IconTerminal2 class="text-muted-foreground" aria-hidden="true" />
            <span class="text-sm">自定义命令…</span>
          </Command.Item>
        </Command.Group>
      </Command.List>
    </Command.Root>
  </PickerField>
  {#if source === "unset" && launchConfigs.status === "unavailable"}
    <p role="alert" class="text-sm text-destructive">
      启动配置存储暂不可用：{launchConfigs.error}。可重新选择自定义命令。
    </p>
  {:else if source === "unset" && launchConfigs.status === "ready"}
    <p role="status" class="text-sm text-muted-foreground">
      原启动配置已删除或当前不可用。请重新选择启动方式，不会推测旧配置的命令或环境变量。
    </p>
  {/if}
  {#if source === "custom"}
    <CustomCommandField bind:command {disabled} />
  {:else if chosen?.chat && chatEnabled && providers.LaunchStartSideFields}
    <providers.LaunchStartSideFields
      bind:startSide
      support={chosen.chatSupport}
      {disabled}
    />
  {/if}
</div>
