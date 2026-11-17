<script lang="ts">
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import { m } from "$lib/paraglide/messages.js";
  import Brand from "../shell/Brand.svelte";
  import SessionSwitcherTrigger from "../shell/SessionSwitcherTrigger.svelte";
  import { Button } from "$lib/components/ui/button";

  let {
    href = "/settings",
    label,
    onclick,
    switcher = false,
  }: {
    href?: string;
    label?: string;
    onclick?: () => void;
    switcher?: boolean;
  } = $props();
  const text = $derived(label ?? m.s2_back_to_settings());
</script>

<header class="app-header border-b bg-card">
  <div
    class="mx-auto flex min-h-(--app-header-height) max-w-6xl items-center justify-between gap-3 px-4 py-1 sm:px-8"
  >
    <a class="brand" href="/"><Brand /></a>
    <div class="flex items-center gap-2">
      {#if switcher}<SessionSwitcherTrigger size="icon" />{/if}
      {#if onclick}
        <Button
          variant="outline"
          class="max-sm:size-(--control-default)"
          aria-label={text}
          title={text}
          {onclick}
          ><IconArrowLeft aria-hidden="true" class="sm:hidden" /><span
            class="max-sm:hidden">{text}</span
          ></Button
        >
      {:else}
        <Button
          {href}
          variant="outline"
          class="max-sm:size-(--control-default)"
          aria-label={text}
          title={text}
          ><IconArrowLeft aria-hidden="true" class="sm:hidden" /><span
            class="max-sm:hidden">{text}</span
          ></Button
        >
      {/if}
    </div>
  </div>
</header>
