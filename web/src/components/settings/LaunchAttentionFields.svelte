<script lang="ts">
  import { tick } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import type {
    AttentionMode,
    AttentionSettings,
    LaunchConfig,
  } from "../../contracts/schemas";
  import {
    defaultLaunchAttentionSettings,
    LAUNCH_ATTENTION_DONE_DELAY_MAX,
    LAUNCH_ATTENTION_DONE_DELAY_MIN,
    LAUNCH_ATTENTION_MIN_TURN_MAX,
    LAUNCH_ATTENTION_MIN_TURN_MIN,
    LAUNCH_ATTENTION_MODE_OPTIONS,
  } from "../../domains/settings/launch-configs";

  let {
    config,
    attention = $bindable<AttentionSettings | null>(null),
    attentionError,
  }: {
    config: LaunchConfig | null;
    attention: AttentionSettings | null;
    attentionError?: string;
  } = $props();
  let defaultAttention = $state(true);
  let attentionMode = $state<AttentionMode>(
    defaultLaunchAttentionSettings.mode,
  );
  let attentionProgramStatus = $state(
    defaultLaunchAttentionSettings.programStatus,
  );
  let attentionAgentProtocol = $state(
    defaultLaunchAttentionSettings.agentProtocol,
  );
  let attentionNotify = $state(defaultLaunchAttentionSettings.notify);
  let attentionBell = $state(defaultLaunchAttentionSettings.bell);
  let attentionMinTurnSeconds = $state<number | undefined>(
    defaultLaunchAttentionSettings.minTurnSeconds,
  );
  let attentionDoneDelaySeconds = $state<number | undefined>(
    defaultLaunchAttentionSettings.doneDelaySeconds,
  );

  $effect.pre(() => {
    defaultAttention = !config?.attention;
    attentionMode =
      config?.attention?.mode ?? defaultLaunchAttentionSettings.mode;
    attentionProgramStatus =
      config?.attention?.programStatus ??
      defaultLaunchAttentionSettings.programStatus;
    attentionAgentProtocol =
      config?.attention?.agentProtocol ??
      defaultLaunchAttentionSettings.agentProtocol;
    attentionNotify =
      config?.attention?.notify ?? defaultLaunchAttentionSettings.notify;
    attentionBell =
      config?.attention?.bell ?? defaultLaunchAttentionSettings.bell;
    attentionMinTurnSeconds =
      config?.attention?.minTurnSeconds ??
      defaultLaunchAttentionSettings.minTurnSeconds;
    attentionDoneDelaySeconds =
      config?.attention?.doneDelaySeconds ??
      defaultLaunchAttentionSettings.doneDelaySeconds;
  });
  $effect.pre(() => {
    attention = defaultAttention
      ? null
      : {
          mode: attentionMode,
          programStatus: attentionProgramStatus,
          agentProtocol: attentionAgentProtocol,
          notify: attentionNotify,
          bell: attentionBell,
          minTurnSeconds:
            attentionMode === "all"
              ? typeof attentionMinTurnSeconds === "number"
                ? attentionMinTurnSeconds
                : Number.NaN
              : 0,
          doneDelaySeconds:
            attentionMode === "all"
              ? typeof attentionDoneDelaySeconds === "number"
                ? attentionDoneDelaySeconds
                : Number.NaN
              : 0,
        };
  });
  const attentionMinTurnInvalid = $derived(
    !defaultAttention &&
      attentionMode === "all" &&
      (typeof attentionMinTurnSeconds !== "number" ||
        !Number.isInteger(attentionMinTurnSeconds) ||
        attentionMinTurnSeconds < LAUNCH_ATTENTION_MIN_TURN_MIN ||
        attentionMinTurnSeconds > LAUNCH_ATTENTION_MIN_TURN_MAX),
  );
  const attentionDoneDelayInvalid = $derived(
    !defaultAttention &&
      attentionMode === "all" &&
      (typeof attentionDoneDelaySeconds !== "number" ||
        !Number.isInteger(attentionDoneDelaySeconds) ||
        attentionDoneDelaySeconds < LAUNCH_ATTENTION_DONE_DELAY_MIN ||
        attentionDoneDelaySeconds > LAUNCH_ATTENTION_DONE_DELAY_MAX),
  );
  const attentionModeDescription = $derived(
    LAUNCH_ATTENTION_MODE_OPTIONS.find(
      (option) => option.value === attentionMode,
    )?.description ?? "",
  );
</script>

<fieldset id="launch-config-attention-fieldset" class="grid min-w-0 gap-2">
  <legend class="mb-2 text-sm font-medium">
    {m.s2_launch_attn_legend()}
    <span class="font-normal text-muted-foreground">{m.s2_optional()}</span>
  </legend>
  <div class="flex items-center gap-2">
    <Checkbox
      id="launch-config-attention-default"
      bind:checked={defaultAttention}
      onCheckedChange={(checked) => {
        if (!checked) {
          void tick().then(() => {
            document
              .getElementById("launch-config-attention-fieldset")
              ?.scrollIntoView({
                block: "nearest",
                behavior: "smooth",
              });
          });
        }
      }}
      aria-describedby="launch-config-attention-help"
    />
    <Label for="launch-config-attention-default"
      >{m.s2_follow_global_default()}<span class="sr-only"
        >{m.s2_launch_attn_default_sr()}</span
      ></Label
    >
  </div>
  {#if defaultAttention}
    <p id="launch-config-attention-help" class="text-xs text-muted-foreground">
      {m.s2_launch_attn_default_help()}
    </p>
  {:else}
    <div class="grid min-w-0 gap-3 pt-1">
      <div class="grid gap-1.5">
        <Label for="launch-config-attention-mode"
          >{m.s2_launch_attn_scope()}</Label
        >
        <SegmentedControl
          id="launch-config-attention-mode"
          aria-label={m.s2_launch_attn_scope()}
          size="form"
          wrap
          bind:value={attentionMode}
        >
          {#each LAUNCH_ATTENTION_MODE_OPTIONS as option (option.value)}
            <SegmentedControlItem value={option.value}>
              {option.label}
            </SegmentedControlItem>
          {/each}
        </SegmentedControl>
        <p class="text-xs text-muted-foreground">
          {attentionModeDescription}
        </p>
      </div>
      {#if attentionMode !== "off"}
        <div class="grid gap-2">
          <span class="text-xs font-medium text-foreground"
            >{m.s2_launch_attn_sources()}</span
          >
          <div class="grid gap-2 sm:grid-cols-2">
            <div class="flex items-center gap-2">
              <Checkbox
                id="launch-config-attention-program-status"
                bind:checked={attentionProgramStatus}
              />
              <Label for="launch-config-attention-program-status"
                >{m.s2_launch_attn_osc7501()}</Label
              >
            </div>
            <div class="flex items-center gap-2">
              <Checkbox
                id="launch-config-attention-agent-protocol"
                bind:checked={attentionAgentProtocol}
              />
              <Label for="launch-config-attention-agent-protocol"
                >{m.s2_launch_attn_warp()}</Label
              >
            </div>
            <div class="flex items-center gap-2">
              <Checkbox
                id="launch-config-attention-notify"
                bind:checked={attentionNotify}
              />
              <Label for="launch-config-attention-notify"
                >{m.s2_launch_attn_osc_notify()}</Label
              >
            </div>
            <div class="flex items-center gap-2">
              <Checkbox
                id="launch-config-attention-bell"
                bind:checked={attentionBell}
              />
              <Label for="launch-config-attention-bell"
                >{m.s2_launch_attn_bell()}</Label
              >
            </div>
          </div>
          <p
            id="launch-config-attention-help"
            class="text-xs text-muted-foreground"
          >
            {m.s2_launch_attn_priority_help()}
          </p>
        </div>
      {/if}
      {#if attentionMode === "all"}
        <div class="grid min-w-0 gap-4 sm:grid-cols-2">
          <div class="grid gap-2">
            <Label for="launch-config-attention-min-turn"
              >{m.s2_launch_attn_min_turn()}</Label
            >
            <Input
              id="launch-config-attention-min-turn"
              type="number"
              min={LAUNCH_ATTENTION_MIN_TURN_MIN}
              max={LAUNCH_ATTENTION_MIN_TURN_MAX}
              step={1}
              bind:value={attentionMinTurnSeconds}
              aria-invalid={attentionMinTurnInvalid}
              aria-describedby={attentionError
                ? "launch-config-attention-threshold-help launch-config-attention-error"
                : "launch-config-attention-threshold-help"}
            />
          </div>
          <div class="grid gap-2">
            <Label for="launch-config-attention-done-delay"
              >{m.s2_launch_attn_done_delay()}</Label
            >
            <Input
              id="launch-config-attention-done-delay"
              type="number"
              min={LAUNCH_ATTENTION_DONE_DELAY_MIN}
              max={LAUNCH_ATTENTION_DONE_DELAY_MAX}
              step={1}
              bind:value={attentionDoneDelaySeconds}
              aria-invalid={attentionDoneDelayInvalid}
              aria-describedby={attentionError
                ? "launch-config-attention-threshold-help launch-config-attention-error"
                : "launch-config-attention-threshold-help"}
            />
          </div>
        </div>
        {#if attentionError}<p
            id="launch-config-attention-error"
            role="alert"
            class="text-xs text-destructive"
          >
            {attentionError}
          </p>{/if}
        <p
          id="launch-config-attention-threshold-help"
          class="text-xs text-muted-foreground"
        >
          {m.s2_launch_attn_threshold_help()}
        </p>
      {/if}
    </div>
  {/if}
</fieldset>
