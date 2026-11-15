<script lang="ts">
  import AppearanceSettings from "../components/settings/AppearanceSettings.svelte";
  import SettingsPageHeader from "../components/settings/SettingsPageHeader.svelte";
  import ShortcutsSettings from "../components/settings/ShortcutsSettings.svelte";
  import {
    settings,
    changeSettings,
    saveSettings,
  } from "../domains/settings/settings.svelte";
  import { onMount } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import InfoHint from "../components/shared/InfoHint.svelte";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import * as Card from "$lib/components/ui/card";
  import { api, navigate } from "../platform/api";
  import {
    ConfigSchema,
    PreviewSettingsSchema,
    type PendingUpdate,
  } from "../contracts/schemas";
  import type { StartupShell } from "../domains/settings/launch-configs";
  import {
    defaultPreviewSettings,
    previewSettings,
  } from "../domains/settings/preview-settings.svelte";
  import { selectPreview } from "../domains/sessions/preview";
  import PendingUpdateNotice from "../components/shared/PendingUpdateNotice.svelte";
  import { setBrowserChrome } from "../platform/browser-chrome";
  import InstallSettings from "../components/settings/InstallSettings.svelte";
  import SettingsNavigation from "../components/settings/SettingsNavigation.svelte";
  import NetworkSettings from "../components/settings/NetworkSettings.svelte";
  import { Switch } from "$lib/components/ui/switch";
  import { browserDraftStorage } from "../domains/terminal/draft-storage";
  import { getPluginContributions } from "../plugins/context";

  const pluginSections = getPluginContributions().settings;
  const launchSections = pluginSections.filter((section) =>
    ["code-reader", "quick-commands", "voice", "launch-configs"].includes(
      section.id,
    ),
  );
  const accessSections = pluginSections.filter((section) =>
    ["access", "tailscale"].includes(section.id),
  );
  const notificationSections = pluginSections.filter(
    (section) => section.id === "push",
  );
  const sections = $derived([
    { id: "appearance", label: m.s1_settings_nav_appearance() },
    ...launchSections,
    { id: "drafts", label: m.s1_settings_nav_drafts() },
    { id: "shortcuts", label: m.s1_settings_nav_shortcuts() },
    { id: "network", label: m.s1_settings_nav_network() },
    ...accessSections,
    { id: "relay", label: m.relay_title() },
    { id: "install", label: m.s1_settings_nav_install() },
    ...notificationSections,
    { id: "preview", label: m.s1_settings_nav_preview() },
  ]);

  // Only a session ID is accepted as the return target, never an arbitrary path.
  const returnSession = new URLSearchParams(location.search).get("session");
  const sessionId =
    returnSession && /^[a-z2-7]{8}$/.test(returnSession) ? returnSession : "";
  let defaultShell = $state<"" | StartupShell>("");
  let pendingUpdate = $state<PendingUpdate>();

  async function loadConfig() {
    try {
      const config = await api("/config", ConfigSchema, "GET");
      pendingUpdate = config.pendingUpdate;
      defaultShell =
        config.defaultShell === "zsh"
          ? "zsh"
          : config.defaultShell === "bash"
            ? "bash"
            : "";
    } catch {
      // Older or unavailable config responses keep the Bash fallback.
      defaultShell = "";
    }
  }

  const draftStorage = browserDraftStorage();
  let keepDrafts = $state(draftStorage.enabled);
  let savedDrafts = $state(draftStorage.count());
  let savedScratch = $state(draftStorage.scratchCount());
  let clearDraftsOpen = $state(false);
  let linesDirty = $state(false);
  let skipDirty = $state(false);
  let lines = $state<number | undefined>(previewSettings.lines);
  let skip = $state<number | undefined>(previewSettings.skip);
  const validation = $derived(PreviewSettingsSchema.safeParse({ lines, skip }));
  const example = $derived(
    [
      ...Array.from({ length: 12 }, (_, index) =>
        m.s1_preview_example_line({ line: String(index + 1) }),
      ),
      m.s1_preview_example_status_bar(),
      m.s1_preview_example_prompt(),
    ].join("\n"),
  );
  const preview = $derived(selectPreview(example, previewSettings));

  $effect(() => {
    if (!linesDirty) lines = previewSettings.lines;
    if (!skipDirty) skip = previewSettings.skip;
  });
  async function savePreview(field: "lines" | "skip") {
    const value = field === "lines" ? lines : skip;
    const parsed = PreviewSettingsSchema.safeParse({
      lines: previewSettings.lines,
      skip: previewSettings.skip,
      [field]: value,
    });
    if (!parsed.success) return;
    try {
      await saveSettings({ preview: parsed.data });
      if (field === "lines") linesDirty = false;
      else skipDirty = false;
    } catch {
      /* keep the input; shared status reports the failure */
    }
  }
  $effect(() => {
    setBrowserChrome(m.s1_chrome_settings(), false);
  });
  onMount(() => {
    void loadConfig();
  });
</script>

<div class="safe-area-page min-h-dvh">
  <SettingsPageHeader
    switcher
    href="/"
    label={sessionId ? m.s1_back_to_terminal() : m.s1_back_to_workspace()}
    onclick={sessionId ? () => navigate(`/s/${sessionId}`) : undefined}
  />
  <div class="settings-layout mx-auto max-w-6xl px-4 sm:px-8">
    <SettingsNavigation {sections} />
    <main
      id="settings-content"
      class="settings-content min-w-0 space-y-6 py-8 sm:py-10"
    >
      <div>
        <h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">
          {m.s1_settings()}
        </h1>
        <p class="mt-2 flex items-center gap-1 text-sm text-muted-foreground">
          {m.s1_settings_sync_summary()}<InfoHint label={m.s1_settings()}
            >{m.s1_settings_sync_hint()}</InfoHint
          >
        </p>
      </div>
      {#if pendingUpdate}<PendingUpdateNotice update={pendingUpdate} />{/if}
      <AppearanceSettings />
      {#each launchSections as section (section.id)}
        {#if section.id === "launch-configs"}
          <section.Component {defaultShell} />
        {:else}
          <section.Component />
        {/if}
      {/each}
      <Card.Root flat id="drafts" class="scroll-mt-4">
        <Card.Header>
          <Card.Title>{m.s1_settings_nav_drafts()}</Card.Title>
          <Card.Description>{m.s1_drafts_card_desc()}</Card.Description>
        </Card.Header>
        <Card.Content class="space-y-4">
          <div class="flex items-center justify-between gap-4">
            <Label for="keep-drafts">{m.s1_keep_local_drafts()}</Label>
            <Switch
              id="keep-drafts"
              checked={keepDrafts}
              onCheckedChange={(checked) => {
                draftStorage.setEnabled(checked);
                keepDrafts = draftStorage.enabled;
                savedDrafts = draftStorage.count();
                savedScratch = draftStorage.scratchCount();
              }}
            />
          </div>
          <p class="text-xs text-muted-foreground">
            {m.s1_keep_local_drafts_help()}
          </p>
          <div class="flex flex-wrap items-center gap-3">
            <span class="text-sm" role="status"
              >{m.s1_saved_drafts_summary({
                drafts: String(savedDrafts),
                scratch: String(savedScratch),
              })}</span
            >
            <Button
              variant="outline"
              disabled={!savedDrafts && !savedScratch}
              onclick={() => (clearDraftsOpen = true)}
              >{m.s1_clear_local_drafts()}</Button
            >
          </div>
        </Card.Content>
      </Card.Root>
      <ShortcutsSettings />
      <NetworkSettings />
      {#each accessSections as section (section.id)}
        <section.Component />
      {/each}
      <Card.Root flat id="relay" class="scroll-mt-4">
        <Card.Header>
          <Card.Title>{m.relay_title()}</Card.Title>
          <Card.Description>
            {m.relay_settings_summary()}
          </Card.Description>
        </Card.Header>
        <Card.Content>
          <Button href="/settings/relay" variant="outline"
            >{m.relay_manage()}</Button
          >
        </Card.Content>
      </Card.Root>
      <InstallSettings />
      {#each notificationSections as section (section.id)}
        <section.Component />
      {/each}
      <Card.Root flat id="preview">
        <Card.Header>
          <Card.Title>{m.s1_session_output_preview()}</Card.Title>
          <Card.Description>{m.s1_preview_card_desc()}</Card.Description>
        </Card.Header>
        <Card.Content class="space-y-6">
          <div class="grid gap-5 sm:grid-cols-2">
            <div class="space-y-2">
              <Label for="preview-lines">{m.s1_preview_lines_label()}</Label>
              <Input
                id="preview-lines"
                type="number"
                min={1}
                max={50}
                step={1}
                bind:value={lines}
                oninput={() => {
                  linesDirty = true;
                }}
                onchange={() => void savePreview("lines")}
                disabled={!settings.ready}
                aria-invalid={!validation.success}
                aria-describedby="preview-lines-help"
              />
              <p id="preview-lines-help" class="text-xs text-muted-foreground">
                {m.s1_preview_lines_help()}
              </p>
            </div>
            <div class="space-y-2">
              <Label for="preview-skip">{m.s1_preview_skip_label()}</Label>
              <Input
                id="preview-skip"
                type="number"
                min={0}
                max={50}
                step={1}
                bind:value={skip}
                oninput={() => {
                  skipDirty = true;
                }}
                onchange={() => void savePreview("skip")}
                disabled={!settings.ready}
                aria-invalid={!validation.success}
                aria-describedby="preview-skip-help"
              />
              <p id="preview-skip-help" class="text-xs text-muted-foreground">
                {m.s1_preview_skip_help()}
              </p>
            </div>
          </div>
          {#if !validation.success}
            <p role="alert" class="text-sm text-destructive">
              {m.s1_preview_validation_error()}
            </p>
          {/if}
          <div class="space-y-2">
            <p class="text-sm">
              {m.s1_preview_current_summary({
                skip: String(previewSettings.skip),
                lines: String(previewSettings.lines),
              })}
            </p>
            <p class="text-xs text-muted-foreground">
              {m.s1_preview_example_caption()}
            </p>
            <pre
              aria-label={m.s1_preview_example_aria()}
              style:--preview-lines={previewSettings.lines}
              class="h-preview min-w-0 overflow-hidden bg-muted p-2 font-mono text-xs leading-5 whitespace-pre text-foreground dark:bg-background">{preview ||
                m.s1_preview_empty_placeholder()}</pre>
          </div>
          <Button
            variant="outline"
            onclick={() => {
              linesDirty = false;
              skipDirty = false;
              changeSettings({ preview: null });
            }}
            >{m.s1_preview_restore_defaults({
              lines: String(defaultPreviewSettings.lines),
              skip: String(defaultPreviewSettings.skip),
            })}</Button
          >
        </Card.Content>
      </Card.Root>
    </main>
  </div>
</div>

<AlertDialog.Root bind:open={clearDraftsOpen}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{m.s1_clear_drafts_confirm_title()}</AlertDialog.Title>
      <AlertDialog.Description
        >{m.s1_clear_drafts_confirm_desc()}</AlertDialog.Description
      >
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>{m.s1_cancel()}</AlertDialog.Cancel>
      <AlertDialog.Action
        variant="destructive"
        onclick={() => {
          draftStorage.clear();
          savedDrafts = draftStorage.count();
          savedScratch = draftStorage.scratchCount();
          clearDraftsOpen = false;
        }}>{m.s1_confirm_clear()}</AlertDialog.Action
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>

<style>
  .settings-layout {
    display: grid;
    min-width: 0;
  }
  .settings-content > :global([id]) {
    scroll-margin-top: 6rem;
  }
  @media (min-width: 1024px) {
    .settings-layout {
      grid-template-columns: 12rem minmax(0, 1fr);
      gap: 2rem;
    }
    .settings-content > :global([id]) {
      scroll-margin-top: 1rem;
    }
  }
  @media (display-mode: standalone) {
    .settings-content > :global([id]) {
      scroll-margin-top: calc(9rem + env(safe-area-inset-top));
    }
  }
</style>
