<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { getFeatureContributions } from "../../features/context";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import {
    defaultPreviewSettings,
    previewSettings,
  } from "../../domains/settings/preview-settings.svelte";
  import type {
    AttentionSettings,
    LaunchConfig,
    PreviewSettings,
  } from "../../contracts/schemas";
  import {
    LAUNCH_PREVIEW_LINES_MAX,
    LAUNCH_PREVIEW_LINES_MIN,
    LAUNCH_PREVIEW_SKIP_MAX,
    LAUNCH_PREVIEW_SKIP_MIN,
  } from "../../domains/settings/launch-configs";

  let {
    config,
    preview = $bindable<PreviewSettings | null>(null),
    attention = $bindable<AttentionSettings | null>(null),
    previewError,
    attentionError,
  }: {
    config: LaunchConfig | null;
    preview: PreviewSettings | null;
    attention: AttentionSettings | null;
    previewError?: string;
    attentionError?: string;
  } = $props();
  const providers = getFeatureContributions().creation;

  let defaultPreview = $state(true);
  let previewLines = $state<number | undefined>(defaultPreviewSettings.lines);
  let previewSkip = $state<number | undefined>(defaultPreviewSettings.skip);

  $effect.pre(() => {
    defaultPreview = !config?.preview;
    previewLines = config?.preview?.lines ?? previewSettings.lines;
    previewSkip = config?.preview?.skip ?? previewSettings.skip;
  });

  $effect.pre(() => {
    preview = defaultPreview
      ? null
      : {
          lines: typeof previewLines === "number" ? previewLines : Number.NaN,
          skip: typeof previewSkip === "number" ? previewSkip : Number.NaN,
        };
  });

  const previewLinesInvalid = $derived(
    !defaultPreview &&
      (typeof previewLines !== "number" ||
        !Number.isInteger(previewLines) ||
        previewLines < LAUNCH_PREVIEW_LINES_MIN ||
        previewLines > LAUNCH_PREVIEW_LINES_MAX),
  );
  const previewSkipInvalid = $derived(
    !defaultPreview &&
      (typeof previewSkip !== "number" ||
        !Number.isInteger(previewSkip) ||
        previewSkip < LAUNCH_PREVIEW_SKIP_MIN ||
        previewSkip > LAUNCH_PREVIEW_SKIP_MAX),
  );
</script>

<fieldset class="grid min-w-0 gap-2">
  <legend class="mb-2 text-sm font-medium">
    {m.s2_launch_preview_legend()}
    <span class="font-normal text-muted-foreground">{m.s2_optional()}</span>
  </legend>
  <div class="flex items-center gap-2">
    <Checkbox
      id="launch-config-preview-default"
      bind:checked={defaultPreview}
      aria-describedby="launch-config-preview-help"
    />
    <Label for="launch-config-preview-default"
      >{m.s2_follow_global_default()}<span class="sr-only"
        >{m.s2_launch_preview_default_sr()}</span
      ></Label
    >
  </div>
  <div class="relative">
    {#if defaultPreview}
      <p
        id="launch-config-preview-help"
        class="absolute inset-x-0 top-0 text-xs text-muted-foreground"
      >
        {m.s2_launch_preview_default_help()}
      </p>
    {/if}
    <div
      class="grid min-w-0 grid-cols-2 gap-4"
      class:invisible={defaultPreview}
      inert={defaultPreview}
    >
      <div class="grid min-w-0 content-start gap-2">
        <Label for={defaultPreview ? undefined : "launch-config-preview-lines"}
          >{m.s2_launch_preview_lines()}</Label
        >
        <Input
          id={defaultPreview ? undefined : "launch-config-preview-lines"}
          type="number"
          min={LAUNCH_PREVIEW_LINES_MIN}
          max={LAUNCH_PREVIEW_LINES_MAX}
          step={1}
          disabled={defaultPreview}
          bind:value={previewLines}
          aria-invalid={previewLinesInvalid}
          aria-describedby={previewError
            ? "launch-config-preview-help launch-config-preview-error"
            : "launch-config-preview-help"}
        />
      </div>
      <div class="grid min-w-0 content-start gap-2">
        <Label for={defaultPreview ? undefined : "launch-config-preview-skip"}
          >{m.s2_launch_preview_skip()}</Label
        >
        <Input
          id={defaultPreview ? undefined : "launch-config-preview-skip"}
          type="number"
          min={LAUNCH_PREVIEW_SKIP_MIN}
          max={LAUNCH_PREVIEW_SKIP_MAX}
          step={1}
          disabled={defaultPreview}
          bind:value={previewSkip}
          aria-invalid={previewSkipInvalid}
          aria-describedby={previewError
            ? "launch-config-preview-help launch-config-preview-error"
            : "launch-config-preview-help"}
        />
      </div>
    </div>
  </div>
  <div class="grid text-xs">
    <p class="invisible col-start-1 row-start-1" aria-hidden="true">
      {m.s2_launch_preview_custom_help()}
    </p>
    {#if previewError && !defaultPreview}<p
        id="launch-config-preview-error"
        role="alert"
        class="col-start-1 row-start-1 text-destructive"
      >
        {previewError}
      </p>{/if}
    {#if !defaultPreview}
      <p
        id="launch-config-preview-help"
        class="col-start-1 row-start-1 text-muted-foreground"
        hidden={!!previewError}
      >
        {m.s2_launch_preview_custom_help()}
      </p>
    {/if}
  </div>
</fieldset>
{#if providers.LaunchAttentionFields}
  <providers.LaunchAttentionFields {config} bind:attention {attentionError} />
{/if}
