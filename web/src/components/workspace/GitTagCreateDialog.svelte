<script lang="ts">
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import { Textarea } from "$lib/components/ui/textarea";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
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
  title="创建本地标签 (Create Tag)"
  description="在选定提交上创建附注标签（含标签者与说明对象）或轻量标签。不允许同名强制覆盖（-f）。"
  drawerDescription="填写标签名称与类型后创建本地标签。"
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="space-y-3" data-testid="git-tag-create-dialog">
    <div class="rounded-lg border bg-background p-3 text-xs">
      <span class="text-muted-foreground">目标提交：</span>
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
      aria-label="标签类型"
    >
      <SegmentedControl.Item value="annotated">
        附注标签 (Annotated)
      </SegmentedControl.Item>
      <SegmentedControl.Item value="lightweight">
        轻量标签 (Lightweight)
      </SegmentedControl.Item>
    </SegmentedControl.Root>

    <div class="space-y-1.5">
      <Label for="git-tag-name-input">标签名称</Label>
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
        <Label for="git-tag-message-input">附注标签说明（必填）</Label>
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
      confirmLabel="创建本地标签"
      submittingLabel="正在创建标签…"
      confirmTestId="git-tag-create-confirm-btn"
      {onConfirm}
      {onClose}
    />
  </div>
</ResponsiveDialog>
