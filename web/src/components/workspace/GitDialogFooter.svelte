<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    dialogError = "",
    submitting = false,
    canConfirm = true,
    wrap = false,
    confirmLabel,
    submittingLabel: rawSubmittingLabel,
    confirmVariant = "default",
    confirmTestId,
    onConfirm,
    onClose,
  }: {
    dialogError?: string;
    submitting?: boolean;
    canConfirm?: boolean;
    wrap?: boolean;
    confirmLabel: string;
    submittingLabel?: string;
    confirmVariant?: "default" | "destructive";
    confirmTestId?: string;
    onConfirm: () => void;
    onClose: () => void;
  } = $props();

  const submittingLabel = $derived(
    rawSubmittingLabel ?? m.s6_processing_ellipsis(),
  );
</script>

{#if dialogError}
  <InlineNotice tone="danger">{dialogError}</InlineNotice>
{/if}

<div
  class="flex min-w-0 max-w-full flex-wrap items-center justify-end gap-2 pt-1"
>
  <Button variant="outline" {wrap} disabled={submitting} onclick={onClose}>
    {m.s6_cancel()}
  </Button>
  <Button
    variant={confirmVariant}
    {wrap}
    disabled={submitting || !canConfirm}
    onclick={onConfirm}
    data-testid={confirmTestId}
  >
    {submitting ? submittingLabel : confirmLabel}
  </Button>
</div>
