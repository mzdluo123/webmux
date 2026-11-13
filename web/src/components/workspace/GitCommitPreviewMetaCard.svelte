<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import type { GitCommitPreviewModel } from "../../contracts/git-workbench-schemas";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    model,
    acknowledgePushedAmend = false,
    submitting = false,
    onAcknowledgePushedAmendChange,
  }: {
    model: GitCommitPreviewModel;
    acknowledgePushedAmend?: boolean;
    submitting?: boolean;
    onAcknowledgePushedAmendChange?: (checked: boolean) => void;
  } = $props();

  const distinctCommitter = $derived(
    Boolean(
      model.committerName &&
      (model.committerName !== model.authorName ||
        model.committerEmail !== model.authorEmail),
    ),
  );
</script>

<div class="space-y-2.5">
  <div
    class="grid min-w-0 grid-cols-1 gap-1.5 rounded-lg border bg-background p-3 text-xs sm:grid-cols-2"
  >
    <p class="wrap-anywhere">
      <span class="text-muted-foreground">目标分支：</span>
      <span class="font-medium text-foreground">{model.branch}</span>
      {#if model.amend}
        <Badge variant="warning" class="ml-1">修正提交 (--amend)</Badge>
      {/if}
    </p>
    <p class="wrap-anywhere">
      <span class="text-muted-foreground">工作树：</span>
      <span class="font-mono text-foreground">{model.worktreePath}</span>
    </p>
    <p class="wrap-anywhere">
      <span class="text-muted-foreground">
        {model.authorOverride ? "作者身份（覆盖）：" : "作者身份："}
      </span>
      <span class="text-foreground">
        {model.authorName || "(未配置)"} &lt;{model.authorEmail || "未配置"}&gt;
      </span>
    </p>
    {#if distinctCommitter || model.authorOverride}
      <p class="wrap-anywhere">
        <span class="text-muted-foreground">提交者身份 (Committer)：</span>
        <span class="text-foreground">
          {model.committerName || "(未配置)"} &lt;{model.committerEmail ||
            "未配置"}&gt;
        </span>
      </p>
    {/if}
    <p class="wrap-anywhere">
      <span class="text-muted-foreground">签名与钩子：</span>
      <span class="text-foreground">
        {model.signEnabled
          ? model.signFormat
            ? `已启用 ${model.signFormat.toUpperCase()} 签名`
            : "已启用签名"
          : "未启用签名"}
        {#if model.signoff}
          · 含 Sign-off 尾注
        {/if}
        ·
        {model.executableHooks.length > 0
          ? model.executableHooks.join(", ")
          : "无可执行钩子"}
      </span>
    </p>
  </div>

  {#if model.amend && model.amendPushed}
    <div
      class="space-y-2 rounded-lg border border-destructive/50 bg-destructive/5 p-3 text-xs"
      data-testid="git-preview-pushed-amend-guard"
    >
      <InlineNotice tone="danger">
        该提交已推送至远端，修正将改写提交历史并导致分支与上游分叉。WebMux
        不会自动执行强制推送（--force）。
      </InlineNotice>
      <Label class="items-start">
        <Checkbox
          checked={acknowledgePushedAmend}
          disabled={submitting}
          onCheckedChange={(val) =>
            onAcknowledgePushedAmendChange?.(Boolean(val))}
          class="mt-0.5"
          data-testid="git-preview-ack-pushed-amend-checkbox"
        />
        <span class="leading-snug text-foreground">
          我已知晓该提交已推送至远端，确认在本地改写该提交历史并承担与远端分叉的影响
        </span>
      </Label>
    </div>
  {/if}
</div>
