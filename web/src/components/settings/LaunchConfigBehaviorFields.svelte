<script lang="ts">
  import { getPluginContributions } from "../../plugins/context";
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
  const providers = getPluginContributions().creation;

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
    输出预览 <span class="font-normal text-muted-foreground">可选</span>
  </legend>
  <div class="flex items-center gap-2">
    <Checkbox
      id="launch-config-preview-default"
      bind:checked={defaultPreview}
      aria-describedby="launch-config-preview-help"
    />
    <Label for="launch-config-preview-default"
      >跟随全局默认<span class="sr-only">（输出预览）</span></Label
    >
  </div>
  {#if defaultPreview}
    <p id="launch-config-preview-help" class="text-xs text-muted-foreground">
      使用设置页「会话输出预览」中的行数与跳过末尾行数。
    </p>
  {:else}
    <div class="grid min-w-0 gap-4 sm:grid-cols-2">
      <div class="grid gap-2">
        <Label for="launch-config-preview-lines">显示行数</Label>
        <Input
          id="launch-config-preview-lines"
          type="number"
          min={LAUNCH_PREVIEW_LINES_MIN}
          max={LAUNCH_PREVIEW_LINES_MAX}
          step={1}
          bind:value={previewLines}
          aria-invalid={previewLinesInvalid}
          aria-describedby={previewError
            ? "launch-config-preview-help launch-config-preview-error"
            : "launch-config-preview-help"}
        />
      </div>
      <div class="grid gap-2">
        <Label for="launch-config-preview-skip">跳过末尾行数</Label>
        <Input
          id="launch-config-preview-skip"
          type="number"
          min={LAUNCH_PREVIEW_SKIP_MIN}
          max={LAUNCH_PREVIEW_SKIP_MAX}
          step={1}
          bind:value={previewSkip}
          aria-invalid={previewSkipInvalid}
          aria-describedby={previewError
            ? "launch-config-preview-help launch-config-preview-error"
            : "launch-config-preview-help"}
        />
      </div>
    </div>
    {#if previewError}<p
        id="launch-config-preview-error"
        role="alert"
        class="text-xs text-destructive"
      >
        {previewError}
      </p>{/if}
    <p id="launch-config-preview-help" class="text-xs text-muted-foreground">
      不同 agent
      的底部状态栏高度不同；按此配置启动的会话在首页使用这些值。修改配置后回到首页生效。
    </p>
  {/if}
</fieldset>
{#if providers.LaunchAttentionFields}
  <providers.LaunchAttentionFields {config} bind:attention {attentionError} />
{/if}
