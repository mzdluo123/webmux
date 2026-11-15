<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Label } from "$lib/components/ui/label";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import { Switch } from "$lib/components/ui/switch";
  import type { ForwardOptions } from "../../contracts/schemas";

  // The compatibility options of one forward. New forwards start with every
  // rewrite on, so a Vite-like dev server works through the gateway as is.
  let {
    id,
    port,
    options = $bindable(),
  }: {
    /** Prefix for the field IDs; unique per form. */
    id: string;
    /** The forwarded port, or null while the add form has none. */
    port: number | null;
    options: ForwardOptions;
  } = $props();
  const base = $derived(port ? `/forward/${port}/` : "/forward/{port}/");
  const portToken = $derived(port ? String(port) : "{port}");
  const switches = $derived([
    {
      key: "rewriteHost",
      label: m.s8_fwd_opt_rewrite_host(),
      help: m.s8_fwd_opt_rewrite_host_help({ port: portToken }),
    },
    {
      key: "rewriteOrigin",
      label: m.s8_fwd_opt_rewrite_origin(),
      help: m.s8_fwd_opt_rewrite_origin_help({ port: portToken }),
    },
    {
      key: "forwardedHeaders",
      label: m.s8_fwd_opt_forwarded_headers(),
      help: m.s8_fwd_opt_forwarded_headers_help(),
    },
    {
      key: "rewriteResponse",
      label: m.s8_fwd_opt_rewrite_response(),
      help: m.s8_fwd_opt_rewrite_response_help(),
    },
  ] as const);
</script>

<div class="space-y-4">
  <div class="space-y-2">
    <p class="text-sm font-medium" id={`${id}-prefix-label`}>
      {m.s8_fwd_prefix_label()}
    </p>
    <SegmentedControl.Root
      size="form"
      bind:value={options.prefix}
      aria-labelledby={`${id}-prefix-label`}
      aria-describedby={`${id}-prefix-help`}
    >
      <SegmentedControl.Item value="keep"
        >{m.s8_fwd_prefix_keep()}</SegmentedControl.Item
      >
      <SegmentedControl.Item value="strip"
        >{m.s8_fwd_prefix_strip()}</SegmentedControl.Item
      >
    </SegmentedControl.Root>
    <p id={`${id}-prefix-help`} class="text-xs text-muted-foreground">
      {#if options.prefix === "keep"}
        {m.s8_fwd_prefix_keep_before()}<code class="font-mono">{base}…</code
        >{m.s8_fwd_prefix_keep_mid1()}<code class="font-mono"
          >--base {base}</code
        >{m.s8_fwd_prefix_keep_mid2()}<code class="font-mono">basePath</code
        >{m.s8_fwd_prefix_keep_after()}
      {:else}
        {m.s8_fwd_prefix_strip_before()}<code class="font-mono"
          >{base.slice(0, -1)}</code
        >{m.s8_fwd_prefix_strip_after()}
      {/if}
    </p>
  </div>
  {#each switches as item (item.key)}
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0 space-y-0.5">
        <Label for={`${id}-${item.key}`}>{item.label}</Label>
        <p
          id={`${id}-${item.key}-help`}
          class="text-xs text-muted-foreground wrap-anywhere"
        >
          {item.help}
        </p>
      </div>
      <Switch
        id={`${id}-${item.key}`}
        bind:checked={options[item.key]}
        aria-describedby={`${id}-${item.key}-help`}
      />
    </div>
  {/each}
</div>
