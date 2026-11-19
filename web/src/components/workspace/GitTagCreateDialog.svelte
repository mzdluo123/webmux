<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { Textarea } from "$lib/components/ui/textarea";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";
  import { useHeightMorph } from "../../hooks/shared/height-morph.svelte";
  import GitDialogFooter from "./GitDialogFooter.svelte";

  let {
    targetOid,
    targetLabel = "",
    tagName,
    tagKind,
    tagMessage,
    submitting = false,
    dialogError = "",
    onTagNameChange,
    onTagKindChange,
    onTagMessageChange,
    onConfirm,
    onClose,
  }: {
    targetOid: string;
    targetLabel?: string;
    tagName: string;
    tagKind: "lightweight" | "annotated";
    tagMessage: string;
    submitting?: boolean;
    dialogError?: string;
    onTagNameChange: (val: string) => void;
    onTagKindChange: (val: "lightweight" | "annotated") => void;
    onTagMessageChange: (val: string) => void;
    onConfirm: () => void;
    onClose: () => void;
  } = $props();

  let messageSection = $state<HTMLDivElement | null>(null);
  const showMessage = $derived(tagKind === "annotated");
  useHeightMorph(
    () => messageSection,
    () => !showMessage,
  );

  const canConfirm = $derived(
    !submitting &&
      Boolean(tagName.trim()) &&
      Boolean(targetOid.trim()) &&
      (tagKind === "lightweight" || Boolean(tagMessage.trim())),
  );
</script>

<ResponsiveDialog
  title={m.s7_tag_create_title()}
  description={m.s7_tag_create_desc()}
  drawerDescription={m.s7_tag_create_drawer_desc()}
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="flex flex-col" data-testid="git-tag-create-dialog">
    <div class="mb-3 rounded-lg border bg-background p-3 text-xs">
      <span class="text-muted-foreground">{m.s7_tag_target_commit_label()}</span
      >
      <span class="font-mono text-foreground">{targetOid.slice(0, 12)}</span>
      {#if targetLabel}
        <span class="wrap-anywhere text-muted-foreground">
          · {targetLabel}</span
        >
      {/if}
    </div>

    <div class="mb-3">
      <SegmentedControl
        wrap
        value={tagKind}
        onValueChange={(val) => {
          if (val === "lightweight" || val === "annotated")
            onTagKindChange(val);
        }}
        aria-label={m.s7_tag_kind_aria()}
      >
        <SegmentedControlItem value="annotated">
          {m.s7_tag_kind_annotated_tab()}
        </SegmentedControlItem>
        <SegmentedControlItem value="lightweight">
          {m.s7_tag_kind_lightweight_tab()}
        </SegmentedControlItem>
      </SegmentedControl>
    </div>

    <div class="space-y-1.5">
      <Label for="git-tag-name-input">{m.s7_tag_name_label()}</Label>
      <Input
        id="git-tag-name-input"
        mono
        value={tagName}
        placeholder="v1.2.0"
        disabled={submitting}
        oninput={(e) => onTagNameChange(e.currentTarget.value)}
        data-testid="git-tag-name-input"
      />
    </div>

    <div
      bind:this={messageSection}
      inert={!showMessage}
      class:h-0={!showMessage}
      class:overflow-clip={!showMessage}
    >
      <div class="space-y-1.5 pt-3">
        <Label for="git-tag-message-input">{m.s7_tag_msg_label()}</Label>
        <Textarea
          id="git-tag-message-input"
          rows={3}
          value={tagMessage}
          placeholder="Release v1.2.0 summary…"
          disabled={submitting}
          oninput={(e) => onTagMessageChange(e.currentTarget.value)}
          data-testid="git-tag-message-input"
        />
      </div>
    </div>

    <div class="mt-3">
      <GitDialogFooter
        {dialogError}
        {submitting}
        {canConfirm}
        confirmLabel={m.s7_tag_create_confirm()}
        submittingLabel={m.s7_tag_create_submitting()}
        confirmTestId="git-tag-create-confirm-btn"
        {onConfirm}
        {onClose}
      />
    </div>
  </div>
</ResponsiveDialog>
