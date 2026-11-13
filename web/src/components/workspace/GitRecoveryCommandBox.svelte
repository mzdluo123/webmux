<script lang="ts">
  import IconCheck from "~icons/tabler/check";
  import IconCopy from "~icons/tabler/copy";
  import { Button } from "$lib/components/ui/button";
  import { copyText } from "../../platform/clipboard";

  let {
    label,
    copyLabel,
    command,
    testId,
  }: {
    label: string;
    copyLabel: string;
    command: string;
    testId?: string;
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

<div
  class="space-y-1.5 rounded-lg border border-git-border bg-git-surface p-3 text-xs"
>
  <div class="flex items-center justify-between gap-2">
    <span class="font-semibold text-foreground">{label}</span>
    <Button
      size="icon-xs"
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
  <code class="block font-mono break-all text-foreground" data-testid={testId}
    >{command}</code
  >
</div>
