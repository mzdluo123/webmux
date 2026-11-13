<script lang="ts">
  import { ItemGroup } from "$lib/components/ui/item";
  import { Badge } from "$lib/components/ui/badge";
  import type { GitBranchItem } from "../../contracts/git-workbench-schemas";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import GitDialogFooter from "./GitDialogFooter.svelte";
  import GitRecoveryCommandBox from "./GitRecoveryCommandBox.svelte";

  let {
    target,
    submitting = false,
    dialogError = "",
    onConfirm,
    onClose,
  }: {
    target: GitBranchItem;
    submitting?: boolean;
    dialogError?: string;
    onConfirm: () => void;
    onClose: () => void;
  } = $props();

  const recoveryCommand = $derived(`git tag ${target.name} ${target.head}`);
</script>

<ResponsiveDialog
  title={`确认删除本地标签 · ${target.name}`}
  description="仅删除本仓库 refs/tags 下的本地标签，不会自动删除远端同名标签。"
  drawerDescription="核对标签指向提交与恢复命令后确认删除。"
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="space-y-3" data-testid="git-tag-delete-dialog">
    <ItemGroup variant="panel" class="space-y-1.5">
      <div class="flex flex-wrap items-center gap-1.5">
        <Badge variant="destructive">
          <span class="font-mono">{target.name}</span>
        </Badge>
        <Badge variant="outline">
          {target.tagKind === "annotated" ? "附注标签" : "轻量标签"}
        </Badge>
      </div>
      <p class="font-mono break-all text-muted-foreground">
        目标提交 OID：{target.head}
      </p>
      {#if target.tagMessage}
        <p class="whitespace-pre-wrap wrap-anywhere text-foreground">
          {target.tagMessage}
        </p>
      {/if}
    </ItemGroup>

    <GitRecoveryCommandBox
      label="删除后重建命令："
      copyLabel="复制标签恢复命令"
      command={recoveryCommand}
      testId="git-tag-delete-recovery-cmd"
    />

    <GitDialogFooter
      {dialogError}
      {submitting}
      confirmLabel="确认删除本地标签"
      submittingLabel="正在删除…"
      confirmVariant="destructive"
      confirmTestId="git-tag-delete-confirm-btn"
      {onConfirm}
      {onClose}
    />
  </div>
</ResponsiveDialog>
