<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import type { GitCommitDraftConflictState } from "../../domains/workspace/git-commit-draft-model";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    conflict,
    saving = false,
    onKeepLocal,
    onAdoptRemote,
    onMergeBoth,
  }: {
    conflict: GitCommitDraftConflictState;
    saving?: boolean;
    onKeepLocal: () => void;
    onAdoptRemote: () => void;
    onMergeBoth: () => void;
  } = $props();
</script>

<div
  class="space-y-2.5 rounded-md border border-warning/40 bg-background p-2.5 text-xs"
  role="alert"
  data-testid="git-commit-draft-conflict"
>
  <InlineNotice tone="warning">
    {conflict.message ||
      "检测到另一设备或标签页已更新提交说明草稿，系统不会静默覆盖；请并排核对两份内容后选择保留、采用远端或合并。"}
  </InlineNotice>

  <div class="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2">
    <div
      class="min-w-0 space-y-1.5 rounded-sm border bg-background p-2"
      data-testid="git-draft-conflict-local"
    >
      <div class="flex min-w-0 items-center justify-between gap-1.5">
        <span class="truncate font-semibold text-foreground">本机当前输入</span>
        <Badge variant="outline">未同步</Badge>
      </div>
      <p
        class="truncate font-mono font-medium text-foreground"
        title={conflict.localSubject || "（空标题）"}
      >
        {conflict.localSubject || "（空标题）"}
      </p>
      {#if conflict.localBody.trim()}
        <div class="rounded-sm bg-background">
          <ScrollArea viewportClasses="max-h-24">
            <pre
              class="p-1.5 font-mono text-xs whitespace-pre-wrap break-words text-muted-foreground">{conflict.localBody}</pre>
          </ScrollArea>
        </div>
      {:else}
        <p class="text-muted-foreground">（无补充正文）</p>
      {/if}
    </div>

    <div
      class="min-w-0 space-y-1.5 rounded-sm border bg-background p-2"
      data-testid="git-draft-conflict-remote"
    >
      <div class="flex min-w-0 items-center justify-between gap-1.5">
        <span class="truncate font-semibold text-foreground">
          另一设备已保存版本
        </span>
        <Badge variant="info">rev #{conflict.remoteDraft.revision}</Badge>
      </div>
      <p
        class="truncate font-mono font-medium text-foreground"
        title={conflict.remoteDraft.subject || "（空标题）"}
      >
        {conflict.remoteDraft.subject || "（空标题）"}
      </p>
      {#if conflict.remoteDraft.body.trim()}
        <div class="rounded-sm bg-background">
          <ScrollArea viewportClasses="max-h-24">
            <pre
              class="p-1.5 font-mono text-xs whitespace-pre-wrap break-words text-muted-foreground">{conflict
                .remoteDraft.body}</pre>
          </ScrollArea>
        </div>
      {:else}
        <p class="text-muted-foreground">（无补充正文）</p>
      {/if}
    </div>
  </div>

  <div class="flex min-w-0 flex-wrap items-center justify-end gap-1.5">
    <Button
      size="sm"
      variant="outline"
      disabled={saving}
      onclick={onAdoptRemote}
      data-testid="git-draft-adopt-remote"
    >
      采用远端版本
    </Button>
    <Button
      size="sm"
      variant="outline"
      disabled={saving}
      onclick={onKeepLocal}
      data-testid="git-draft-keep-local"
    >
      保留本机覆盖保存
    </Button>
    <Button
      size="sm"
      variant="default"
      disabled={saving}
      onclick={onMergeBoth}
      data-testid="git-draft-merge-both"
    >
      合并两份内容
    </Button>
  </div>
</div>
