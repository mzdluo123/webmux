<script lang="ts">
  import {
    settings,
    changeSettings,
    reloadSettings,
  } from "../../domains/settings/settings.svelte";
  import {
    terminalFont,
    minFontSize,
    maxFontSize,
  } from "../../hooks/terminal/terminal-font-preference.svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import * as Card from "$lib/components/ui/card";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import * as m from "$lib/paraglide/messages.js";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import LocaleChoice from "../shared/LocaleChoice.svelte";
  import { tvMode } from "../../hooks/shared/tv-mode.svelte";
  import { headsetMode } from "../../hooks/shared/headset-mode.svelte";
  let fontDraft = $state<number | undefined>(undefined);
  const fontInput = $derived(fontDraft ?? terminalFont.size);
  const validFont = $derived(
    Number.isInteger(fontInput) &&
      fontInput >= minFontSize &&
      fontInput <= maxFontSize,
  );
</script>

<Card.Root flat id="appearance" class="scroll-mt-4">
  <Card.Header>
    <Card.Title>{m.s2_appearance_title()}</Card.Title>
    <Card.Description>{m.s2_appearance_desc()}</Card.Description>
  </Card.Header>
  <Card.Content class="space-y-6">
    <div class="space-y-2">
      <p class="text-sm font-medium">{m.locale_label()}</p>
      <LocaleChoice />
    </div>
    <div class="flex items-center justify-between gap-4">
      <div class="space-y-1">
        <Label for="tv-mode">{m.s2_tv_mode()}</Label>
        <p class="text-sm text-muted-foreground">
          {m.s2_tv_mode_desc()}
        </p>
      </div>
      <Switch
        id="tv-mode"
        bind:checked={tvMode.enabled}
        aria-label={m.s2_tv_mode()}
      />
    </div>
    <div class="flex items-center justify-between gap-4">
      <div class="space-y-1">
        <Label for="headset-mode">{m.s2_headset_mode()}</Label>
        <p class="text-sm text-muted-foreground">
          {m.s2_headset_mode_desc()}
        </p>
      </div>
      <Switch
        id="headset-mode"
        bind:checked={headsetMode.enabled}
        aria-label={m.s2_headset_mode()}
      />
    </div>
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p role="status" class="text-sm text-muted-foreground">
        {settings.saving
          ? m.s2_saving()
          : !settings.ready
            ? m.s2_settings_unavailable()
            : settings.connected
              ? m.s2_settings_synced()
              : m.s2_settings_reconnecting()}
      </p>
      <Button variant="outline" size="sm" onclick={() => void reloadSettings()}
        >{m.s2_reload()}</Button
      >
    </div>
    {#if settings.error}<InlineNotice tone="danger"
        >{settings.error}</InlineNotice
      >{/if}
    <div class="space-y-2">
      <Label for="settings-font">{m.s2_terminal_font_size()}</Label>
      <Input
        id="settings-font"
        type="number"
        min={minFontSize}
        max={maxFontSize}
        step={1}
        value={fontInput}
        oninput={(event) => {
          fontDraft = event.currentTarget.valueAsNumber;
          if (validFont) terminalFont.set(fontDraft);
        }}
        onblur={() => (fontDraft = undefined)}
        aria-invalid={!validFont}
      />
      {#if !validFont}<p role="alert" class="text-sm text-destructive">
          {m.s2_terminal_font_range_err({
            min: String(minFontSize),
            max: String(maxFontSize),
          })}
        </p>{/if}
      <Button
        variant="outline"
        size="sm"
        onclick={() => {
          fontDraft = undefined;
          terminalFont.reset();
        }}>{m.s2_reset_default_font()}</Button
      >
    </div>
    <fieldset disabled={!settings.ready} class="space-y-6">
      <div class="space-y-2">
        <p class="text-sm font-medium">{m.s2_theme()}</p>
        <SegmentedControl.Root
          value={settings.values.theme ?? "system"}
          onValueChange={(value) =>
            changeSettings({
              theme: value === "light" || value === "dark" ? value : null,
            })}
          disabled={!settings.ready}
          aria-label={m.s2_theme()}
        >
          <SegmentedControl.Item value="system"
            >{m.s2_theme_system()}</SegmentedControl.Item
          >
          <SegmentedControl.Item value="light"
            >{m.s2_theme_light()}</SegmentedControl.Item
          >
          <SegmentedControl.Item value="dark"
            >{m.s2_theme_dark()}</SegmentedControl.Item
          >
        </SegmentedControl.Root>
      </div>
      <div class="flex items-center justify-between gap-3">
        <Label for="settings-softkeys">{m.s2_force_softkeys()}</Label>
        <Switch
          id="settings-softkeys"
          checked={settings.values.forceSoftKeys ?? false}
          onCheckedChange={(value) => changeSettings({ forceSoftKeys: value })}
        />
      </div>
      <Button
        variant="outline"
        size="sm"
        onclick={() => changeSettings({ forceSoftKeys: null })}
        >{m.s2_reset_default_toolbar()}</Button
      >
    </fieldset>
  </Card.Content>
</Card.Root>
