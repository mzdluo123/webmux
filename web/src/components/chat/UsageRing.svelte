<script lang="ts">
  import { useUsageMotion } from "../../hooks/chat/usage-motion.svelte";
  import { Button } from "$lib/components/ui/button";
  import * as Popover from "$lib/components/ui/popover";
  import { locale } from "$lib/locale";
  import { m } from "$lib/paraglide/messages.js";
  import type { ChatUsage } from "../../contracts/schemas";

  // Context-window use from the agent's usage updates, with its cost; a
  // click opens the exact token counts.
  let { usage }: { usage: ChatUsage } = $props();
  const ratio = $derived(
    usage.size > 0 ? Math.min(1, usage.used / usage.size) : 0,
  );
  const percent = $derived(Math.round(ratio * 100));
  const circumference = 2 * Math.PI * 15;
  let ring = $state<SVGCircleElement | null>(null);
  let bar = $state<HTMLDivElement | null>(null);
  useUsageMotion(
    () => ring,
    "strokeDasharray",
    () => `${ratio * circumference}px, ${circumference}px`,
  );
  useUsageMotion(
    () => bar,
    "width",
    () => `${percent}%`,
  );
  const kilo = (value: number) =>
    value >= 1000 ? `${(value / 1000).toFixed(1)}k` : String(value);
  const exact = (value: number) => value.toLocaleString(locale.current);
  const money = $derived(
    usage.cost === undefined
      ? ""
      : `${usage.currency && usage.currency !== "USD" ? `${usage.currency} ` : "$"}${usage.cost.toFixed(2)}`,
  );
  const label = $derived(
    money
      ? m.s3_usage_ring_label_cost({
          percent: String(percent),
          used: kilo(usage.used),
          size: kilo(usage.size),
          cost: money,
        })
      : m.s3_usage_ring_label({
          percent: String(percent),
          used: kilo(usage.used),
          size: kilo(usage.size),
        }),
  );
  const remaining = $derived(Math.max(0, usage.size - usage.used));
  const rows = $derived<Array<[string, string]>>([
    [m.s3_usage_used(), `${exact(usage.used)} tokens`],
    [
      m.s3_context_window(),
      usage.size > 0 ? `${exact(usage.size)} tokens` : m.s3_usage_unknown(),
    ],
    [
      m.s3_usage_remaining(),
      usage.size > 0 ? `${exact(remaining)} tokens` : m.s3_usage_unknown(),
    ],
    ...(money
      ? ([[m.s3_usage_session_cost(), money]] as Array<[string, string]>)
      : []),
  ]);
</script>

<Popover.Root>
  <Popover.Trigger>
    {#snippet child({ props })}<Button
        {...props}
        variant="quiet"
        size="xs"
        class="shrink-0"
        aria-label={label}
        title={label}
      >
        <svg
          viewBox="0 0 36 36"
          class="size-4 shrink-0 -rotate-90"
          aria-hidden="true"
        >
          <circle
            cx="18"
            cy="18"
            r="15"
            fill="none"
            class="stroke-muted"
            stroke-width="4"
          />
          <circle
            bind:this={ring}
            cx="18"
            cy="18"
            r="15"
            fill="none"
            class={ratio > 0.8 ? "stroke-warning" : "stroke-primary"}
            stroke-width="4"
            stroke-linecap="round"
            stroke-dasharray={`${ratio * circumference} ${circumference}`}
          />
        </svg>
        <span class="tabular-nums max-sm:hidden">{percent}%</span>
      </Button>{/snippet}
  </Popover.Trigger>
  <Popover.Content
    role="dialog"
    aria-label={m.s3_usage_dialog_title()}
    align="end"
    side="top"
    collisionPadding={16}
    class="w-64 gap-3"
  >
    <Popover.Header>
      <Popover.Title>{m.s3_usage_dialog_title()}</Popover.Title>
      <Popover.Description>{m.s3_usage_dialog_desc()}</Popover.Description>
    </Popover.Header>
    <div class="space-y-1.5">
      <div class="flex items-baseline justify-between text-xs">
        <span class="text-muted-foreground">{m.s3_usage_context()}</span>
        <span class="font-medium tabular-nums">{percent}%</span>
      </div>
      <div
        class="h-1.5 overflow-hidden rounded-full bg-muted"
        role="progressbar"
        aria-label={m.s3_usage_context_used()}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={percent}
      >
        <div
          bind:this={bar}
          class={[
            "h-full w-(--usage) rounded-full",
            ratio > 0.8 ? "bg-warning" : "bg-primary",
          ]}
          style:--usage={`${percent}%`}
        ></div>
      </div>
    </div>
    <dl class="grid grid-cols-2 gap-x-3 gap-y-1.5 text-xs">
      {#each rows as [name, value] (name)}
        <dt class="text-muted-foreground">{name}</dt>
        <dd class="text-right font-medium tabular-nums">{value}</dd>
      {/each}
    </dl>
  </Popover.Content>
</Popover.Root>
