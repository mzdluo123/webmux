<script lang="ts">
  import IconCheck from "~icons/tabler/check";
  import IconCopy from "~icons/tabler/copy";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { copyText } from "../../platform/clipboard";

  let {
    label,
    copyLabel,
    command,
    testId,
    copyButton = $bindable(null),
    bounded = false,
  }: {
    label: string;
    copyLabel: string;
    command: string;
    testId?: string;
    copyButton?: HTMLButtonElement | null;
    bounded?: boolean;
  } = $props();

  let copied = $state(false);

  function handleCopy() {
    if (!command) return;
    void copyText(command);
    copied = true;
    setTimeout(() => {
      copied = false;
    }, 1500);
  }
</script>

<div class="space-y-1.5 rounded-lg border bg-background p-3 text-xs">
  <div class="flex items-center justify-between gap-2">
    <span class="font-semibold text-foreground">{label}</span>
    <Button
      bind:ref={copyButton}
      size="icon-sm"
      variant="ghost"
      aria-label={copyLabel}
      title={copyLabel}
      onclick={handleCopy}
    >
      {#if copied}
        <IconCheck class="text-success" aria-hidden="true" />
      {:else}
        <IconCopy aria-hidden="true" />
      {/if}
    </Button>
  </div>
  {#if bounded}
    <ScrollArea
      viewportClasses="max-h-20 overscroll-contain"
      viewportProps={{ tabindex: 0, "aria-label": label }}
    >
      {@render recoveryCommand()}
    </ScrollArea>
  {:else}
    {@render recoveryCommand()}
  {/if}
</div>

{#snippet recoveryCommand()}
  <code class="block font-mono break-all text-foreground" data-testid={testId}
    >{command}</code
  >
{/snippet}
