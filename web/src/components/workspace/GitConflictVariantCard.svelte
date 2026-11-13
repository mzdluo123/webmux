<script lang="ts">
  import IconCheck from "~icons/tabler/check";
  import IconTrash from "~icons/tabler/trash";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import type { GitConflictFileDetail } from "../../contracts/schemas";
  import {
    formatConflictSideMeta,
    gitConflictKindLabel,
  } from "../../domains/workspace/git-conflict-model";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    detail,
    resolving = false,
    onResolveVariant,
  }: {
    detail: GitConflictFileDetail;
    resolving?: boolean;
    onResolveVariant: (
      resolution: "choose_left" | "choose_right" | "delete",
    ) => void;
  } = $props();
</script>

<div
  class="min-h-0 flex-1 space-y-3 overflow-y-auto p-3 text-xs"
  data-testid="git-conflict-variant-card"
>
  <InlineNotice tone="warning">
    <div class="space-y-1">
      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        <Badge variant="warning" class="max-w-full">
          <span class="truncate">{gitConflictKindLabel(detail.kind)}</span>
        </Badge>
        <span class="font-mono break-all font-medium">{detail.path}</span>
      </div>
      <p class="leading-relaxed">
        {detail.degradeReason ||
          "此文件冲突无法执行文本三方合并，请显式选择要保留的版本或删除此文件。"}
      </p>
    </div>
  </InlineNotice>

  <div class="grid min-w-0 grid-cols-1 gap-3 lg:grid-cols-2">
    <div
      class="flex min-w-0 flex-col justify-between gap-3 rounded-lg border border-git-border bg-git-panel p-3"
    >
      <div class="space-y-1.5">
        <div class="flex flex-wrap items-center gap-1.5">
          <Badge variant="info">左侧来源</Badge>
          <span class="font-semibold wrap-anywhere text-foreground">
            {detail.left.label}
          </span>
        </div>
        <p class="wrap-anywhere text-muted-foreground">{detail.left.role}</p>
        <p class="font-mono break-all text-muted-foreground">
          {formatConflictSideMeta(detail.left)}
        </p>
        {#if detail.left.present && !detail.left.binary && detail.left.text}
          <pre
            class="max-h-40 overflow-auto rounded-md border border-git-border bg-git-surface p-2 font-mono text-xs whitespace-pre-wrap break-all text-foreground">{detail
              .left.text}</pre>
        {/if}
      </div>

      <div>
        <Button
          size="sm"
          variant="git-primary"
          wrap
          disabled={!detail.left.present || resolving}
          onclick={() => onResolveVariant("choose_left")}
          data-testid="git-conflict-choose-left-btn"
        >
          <IconCheck aria-hidden="true" />
          保留左侧版本并标记已解决
        </Button>
      </div>
    </div>

    <div
      class="flex min-w-0 flex-col justify-between gap-3 rounded-lg border border-git-border bg-git-panel p-3"
    >
      <div class="space-y-1.5">
        <div class="flex flex-wrap items-center gap-1.5">
          <Badge variant="outline">右侧来源</Badge>
          <span class="font-semibold wrap-anywhere text-foreground">
            {detail.right.label}
          </span>
        </div>
        <p class="wrap-anywhere text-muted-foreground">{detail.right.role}</p>
        <p class="font-mono break-all text-muted-foreground">
          {formatConflictSideMeta(detail.right)}
        </p>
        {#if detail.right.present && !detail.right.binary && detail.right.text}
          <pre
            class="max-h-40 overflow-auto rounded-md border border-git-border bg-git-surface p-2 font-mono text-xs whitespace-pre-wrap break-all text-foreground">{detail
              .right.text}</pre>
        {/if}
      </div>

      <div>
        <Button
          size="sm"
          variant="outline"
          wrap
          disabled={!detail.right.present || resolving}
          onclick={() => onResolveVariant("choose_right")}
          data-testid="git-conflict-choose-right-btn"
        >
          <IconCheck aria-hidden="true" />
          保留右侧版本并标记已解决
        </Button>
      </div>
    </div>
  </div>

  <div
    class="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-git-border bg-git-surface p-3"
  >
    <div class="min-w-0 space-y-0.5">
      <p class="font-medium text-foreground">
        共同祖先 Base (stage 1)：{formatConflictSideMeta(detail.base)}
      </p>
      <p class="text-muted-foreground">
        若需要将此文件从工作树与暂存索引中移除，可确认删除文件并清除未合并阶段。
      </p>
    </div>
    <Button
      size="sm"
      variant="destructive-outline"
      wrap
      disabled={resolving}
      onclick={() => onResolveVariant("delete")}
      data-testid="git-conflict-delete-file-btn"
    >
      <IconTrash aria-hidden="true" />
      删除文件并标记已解决
    </Button>
  </div>
</div>
