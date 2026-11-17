<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import { Textarea } from "$lib/components/ui/textarea";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";
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
  <div class="space-y-3" data-testid="git-tag-create-dialog">
    <div class="rounded-lg border bg-background p-3 text-xs">
      <span class="text-muted-foreground">{m.s7_tag_target_commit_label()}</span
      >
      <span class="font-mono text-foreground">{targetOid.slice(0, 12)}</span>
      {#if targetLabel}
        <span class="text-muted-foreground"> · {targetLabel}</span>
      {/if}
    </div>

    <SegmentedControl.Root
      wrap
      value={tagKind}
      onValueChange={(val) => {
        if (val === "lightweight" || val === "annotated") onTagKindChange(val);
      }}
      aria-label={m.s7_tag_kind_aria()}
    >
      <SegmentedControl.Item value="annotated">
        {m.s7_tag_kind_annotated_tab()}
      </SegmentedControl.Item>
      <SegmentedControl.Item value="lightweight">
        {m.s7_tag_kind_lightweight_tab()}
      </SegmentedControl.Item>
    </SegmentedControl.Root>

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

    {#if tagKind === "annotated"}
      <div class="space-y-1.5">
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
    {/if}

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
</ResponsiveDialog>
