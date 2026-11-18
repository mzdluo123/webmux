<script lang="ts">
  import { errorMessage } from "../../platform/error";
  import { untrack } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import EnvRowsEditor from "./EnvRowsEditor.svelte";
  import LaunchConfigBehaviorFields from "./LaunchConfigBehaviorFields.svelte";
  import { Button } from "$lib/components/ui/button";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import { Textarea } from "$lib/components/ui/textarea";
  import * as Dialog from "$lib/components/ui/dialog";
  import * as Select from "$lib/components/ui/select";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import LaunchConfigIcon from "./LaunchConfigIcon.svelte";
  import { api, APIError } from "../../platform/api";
  import { featureEnabled } from "../../platform/feature-host.svelte";
  import { getFeatureContributions } from "../../features/context";
  import {
    LaunchConfigSchema,
    type AttentionSettings,
    type LaunchConfig,
    type PreviewSettings,
  } from "../../contracts/schemas";
  import {
    LAUNCH_ENV_MAX,
    LAUNCH_ICON_OPTIONS,
    launchConfigIcon,
    launchCommandDraft,
    validateLaunchDraft,
    type EnvRow,
    type StartupShell,
  } from "../../domains/settings/launch-configs";

  let {
    open = $bindable(false),
    configId,
    defaultShell = "",
    onsaved,
  }: {
    open: boolean;
    /** Config being edited; undefined creates a new one. */
    configId: string | undefined;
    /** Startup file shell to preselect; "" falls back to Bash. */
    defaultShell?: "" | StartupShell;
    onsaved: (config: LaunchConfig) => void;
  } = $props();
  const providers = getFeatureContributions().creation;
  const chatEnabled = featureEnabled("chat") && !!providers.LaunchChatFields;
  const notificationsEnabled =
    featureEnabled("notifications") && !!providers.LaunchAttentionFields;

  let loadedConfig = $state<LaunchConfig | null>(null);
  let name = $state("");
  let icon = $state("auto");
  let command = $state("");
  let loadStartup = $state(false);
  let startupShell = $state<StartupShell>("bash");
  let rows = $state<EnvRow[]>([]);
  let preview = $state<PreviewSettings | null>(null);
  let attention = $state<AttentionSettings | null>(null);
  let nextKey = 0;
  let loading = $state(false);
  let loadError = $state("");
  let error = $state("");
  let busy = $state(false);
  let hybrid = $state(false);
  const draft = $derived(
    validateLaunchDraft(
      name,
      command,
      rows,
      loadStartup ? startupShell : null,
      preview,
      attention,
    ),
  );
  const iconOptions = $derived([
    { value: "auto", label: m.s2_icon_auto() },
    ...LAUNCH_ICON_OPTIONS.map((option) => ({
      value: option.value,
      label: option.label,
    })),
  ]);
  const startupShellOptions = $derived<
    { value: StartupShell; label: string }[]
  >([
    { value: "bash", label: m.s2_startup_shell_bash() },
    { value: "zsh", label: m.s2_startup_shell_zsh() },
  ]);
  const autoIcon = $derived(
    draft.ok ? launchConfigIcon(draft.value) : "terminal",
  );

  function row(name = "", value = ""): EnvRow {
    return { key: nextKey++, name, value };
  }

  // Each opening starts from the server copy, so cancel/Esc discards edits.
  // Editing never saves from a summary: the full config (with env values) must
  // load first, otherwise a whole-replacement PUT would drop the variables.
  $effect.pre(() => {
    if (!open) return;
    const controller = new AbortController();
    untrack(() => {
      loadedConfig = null;
      name = "";
      icon = "auto";
      command = "";
      loadStartup = false;
      startupShell = defaultShell === "zsh" ? "zsh" : "bash";
      rows = [];
      preview = null;
      attention = null;
      error = "";
      loadError = "";
      busy = false;
      hybrid = false;
      loading = !!configId;
      if (configId)
        api(
          `/launch-configs/${configId}`,
          LaunchConfigSchema,
          "GET",
          undefined,
          controller.signal,
        )
          .then((config) => {
            loadedConfig = config;
            name = config.name;
            icon = config.icon || "auto";
            const restored = launchCommandDraft(config.cmd);
            command = restored.command;
            loadStartup = restored.startupShell !== null;
            startupShell =
              restored.startupShell ??
              (defaultShell === "zsh" ? "zsh" : "bash");
            rows = Object.entries(config.env).map(([key, value]) =>
              row(key, value),
            );
            hybrid = config.chat;
          })
          .catch((err: unknown) => {
            if (!controller.signal.aborted)
              loadError = m.s2_launch_err_load({ error: errorMessage(err) });
          })
          .finally(() => {
            if (!controller.signal.aborted) loading = false;
          });
    });
    return () => controller.abort();
  });

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (!draft.ok || busy || loading || loadError) return;
    busy = true;
    error = "";
    try {
      const { attention: draftAttention, ...launch } = draft.value;
      const saved = await api(
        configId ? `/launch-configs/${configId}` : "/launch-configs",
        LaunchConfigSchema,
        configId ? "PUT" : "POST",
        {
          ...launch,
          ...(featureEnabled("chat")
            ? { chat: chatEnabled ? hybrid : (loadedConfig?.chat ?? false) }
            : {}),
          ...(featureEnabled("notifications")
            ? {
                attention: notificationsEnabled
                  ? draftAttention
                  : (loadedConfig?.attention ?? null),
              }
            : {}),
          ...(icon === "auto" ? {} : { icon }),
        },
      );
      onsaved(saved);
      open = false;
    } catch (err) {
      error =
        err instanceof APIError && err.status === 409
          ? m.s2_launch_err_conflict({ name: draft.value.name })
          : errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog.Root bind:open>
  <Dialog.Content
    class="flex max-h-[90dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-xl"
  >
    <form class="flex min-h-0 flex-1 flex-col" onsubmit={submit}>
      <div class="shrink-0 border-b px-5 py-4 pr-12">
        <Dialog.Header>
          <Dialog.Title
            >{configId
              ? m.s2_launch_edit_title()
              : m.s2_launch_new_title()}</Dialog.Title
          >
          <Dialog.Description>{m.s2_launch_dialog_desc()}</Dialog.Description>
        </Dialog.Header>
      </div>
      <div
        class="grid min-h-0 min-w-0 flex-1 gap-4 overflow-y-auto overscroll-contain p-5"
        data-testid="launch-config-scroll"
      >
        {#if loading}
          <p role="status" class="text-sm text-muted-foreground">
            {m.s2_launch_loading()}
          </p>
        {:else if loadError}
          <InlineNotice tone="danger">{loadError}</InlineNotice>
        {:else}
          <fieldset disabled={busy} class="grid min-w-0 gap-4">
            <div class="grid gap-2">
              <Label for="launch-config-name">{m.s2_name()}</Label>
              <Input
                id="launch-config-name"
                bind:value={name}
                placeholder={m.s2_launch_name_placeholder()}
                autocomplete="off"
                aria-invalid={!!name && !!draft.errors.name}
                aria-describedby={name && draft.errors.name
                  ? "launch-config-name-error"
                  : undefined}
              />
              {#if name && draft.errors.name}<p
                  id="launch-config-name-error"
                  class="text-xs text-destructive"
                >
                  {draft.errors.name}
                </p>{/if}
            </div>
            <div class="grid gap-2">
              <Label for="launch-config-icon">{m.s2_launch_icon()}</Label>
              <Select.Root type="single" bind:value={icon} items={iconOptions}>
                <Select.Trigger
                  id="launch-config-icon"
                  aria-label={m.s2_launch_icon()}
                >
                  <span class="flex items-center gap-2">
                    <LaunchConfigIcon
                      icon={icon === "auto" ? autoIcon : icon}
                    />
                    {iconOptions.find((option) => option.value === icon)
                      ?.label ?? m.s2_icon_terminal()}
                  </span>
                </Select.Trigger>
                <Select.Content>
                  {#each iconOptions as option (option.value)}
                    <Select.Item value={option.value} label={option.label}>
                      <LaunchConfigIcon
                        icon={option.value === "auto" ? autoIcon : option.value}
                      />
                      {option.label}
                    </Select.Item>
                  {/each}
                </Select.Content>
              </Select.Root>
              <p class="text-xs text-muted-foreground">
                {m.s2_launch_icon_help()}
              </p>
            </div>
            <div class="grid gap-2">
              <Label for="launch-config-command">{m.s2_launch_cmd()}</Label>
              <Textarea
                id="launch-config-command"
                bind:value={command}
                rows={3}
                placeholder={m.s2_launch_cmd_placeholder()}
                mono
                class="max-h-[30dvh]"
                autocapitalize="off"
                autocorrect="off"
                spellcheck={false}
                aria-invalid={!!command.trim() && !!draft.errors.command}
                aria-describedby="launch-config-command-help"
              />
              {#if command.trim() && draft.errors.command}<p
                  class="text-xs text-destructive"
                >
                  {draft.errors.command}
                </p>{/if}
              <p
                id="launch-config-command-help"
                class="text-xs text-muted-foreground"
              >
                {m.s2_launch_cmd_help()}
              </p>
              <div class="flex items-center gap-2">
                <Checkbox
                  id="launch-config-startup"
                  bind:checked={loadStartup}
                  aria-describedby="launch-config-startup-help"
                />
                <Label for="launch-config-startup"
                  >{m.s2_launch_load_startup()}</Label
                >
              </div>
              {#if loadStartup}
                <div class="grid gap-2">
                  <Label for="launch-config-startup-shell"
                    >{m.s2_launch_startup_file()}</Label
                  >
                  <Select.Root
                    type="single"
                    value={startupShell}
                    onValueChange={(value) => {
                      if (value === "bash" || value === "zsh")
                        startupShell = value;
                    }}
                    items={startupShellOptions}
                  >
                    <Select.Trigger
                      id="launch-config-startup-shell"
                      aria-label={m.s2_launch_startup_shell_aria()}
                    >
                      {startupShellOptions.find(
                        (option) => option.value === startupShell,
                      )?.label}
                    </Select.Trigger>
                    <Select.Content>
                      {#each startupShellOptions as option (option.value)}
                        <Select.Item value={option.value} label={option.label}>
                          {option.label}
                        </Select.Item>
                      {/each}
                    </Select.Content>
                  </Select.Root>
                </div>
              {/if}
              <p
                id="launch-config-startup-help"
                class="text-xs text-muted-foreground"
              >
                {m.s2_launch_startup_help()}
              </p>
              {#if providers.LaunchChatFields}
                <providers.LaunchChatFields
                  config={loadedConfig}
                  cmd={draft.ok ? draft.value.cmd : draft.cmd}
                  bind:hybrid
                  disabled={busy}
                />
              {/if}
            </div>
            <EnvRowsEditor
              bind:rows
              errors={draft.errors.rows}
              error={draft.errors.env}
              max={LAUNCH_ENV_MAX}
              makeRow={row}
            >
              {#snippet help()}{m.s2_launch_env_help()}{/snippet}
            </EnvRowsEditor>
            <LaunchConfigBehaviorFields
              config={loadedConfig}
              bind:preview
              bind:attention
              previewError={draft.errors.preview}
              attentionError={draft.errors.attention}
            />
            {#if error}<InlineNotice tone="danger">{error}</InlineNotice>{/if}
          </fieldset>
        {/if}
      </div>
      <div class="shrink-0 border-t bg-muted/30 px-5 py-3">
        <Dialog.Footer>
          <Button
            type="button"
            variant="outline"
            onclick={() => {
              open = false;
            }}>{m.s2_cancel()}</Button
          >
          {#if !loadError}<Button
              type="submit"
              disabled={loading || busy || !draft.ok}
              >{busy ? m.s2_saving() : m.s2_save()}</Button
            >{/if}
        </Dialog.Footer>
      </div>
    </form>
  </Dialog.Content>
</Dialog.Root>
