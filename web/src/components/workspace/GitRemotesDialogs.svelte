<script lang="ts">
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import { containsRedactedRemoteMask } from "../../domains/workspace/git-v2-adapter";
  import type { GitRemotesController } from "../../hooks/workspace/git-remotes-controller.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import GitDialogFooter from "./GitDialogFooter.svelte";

  let { remotes }: { remotes: GitRemotesController } = $props();

  const hasMask = $derived(
    containsRedactedRemoteMask(remotes.fetchUrl) ||
      containsRedactedRemoteMask(remotes.pushUrl),
  );
  const canSave = $derived(
    !remotes.submitting &&
      Boolean(remotes.name.trim()) &&
      Boolean(remotes.fetchUrl.trim()) &&
      !hasMask,
  );
</script>

{#if remotes.editOpen}
  <ResponsiveDialog
    title={remotes.editMode === "add"
      ? "添加远程仓库 (Add Remote)"
      : `编辑远程仓库 · ${remotes.name}`}
    description="保存后回读的远程地址将自动脱敏隐藏密码与访问令牌（显示为 ***）。"
    drawerDescription="填写远程名称与拉取/推送地址。"
    busy={remotes.submitting}
    class="sm:max-w-lg"
    onclose={() => remotes.closeDialogs()}
  >
    <div class="space-y-3" data-testid="git-remote-edit-dialog">
      <div class="space-y-1.5">
        <Label for="git-remote-name-input">远程名称</Label>
        <Input
          id="git-remote-name-input"
          mono
          value={remotes.name}
          placeholder="origin"
          disabled={remotes.editMode === "edit" || remotes.submitting}
          oninput={(e) => remotes.setName(e.currentTarget.value)}
          data-testid="git-remote-name-input"
        />
      </div>

      <div class="space-y-1.5">
        <Label for="git-remote-fetch-input">获取地址 (Fetch URL，必填)</Label>
        <Input
          id="git-remote-fetch-input"
          mono
          value={remotes.fetchUrl}
          placeholder="https://github.com/org/repo.git 或 git@github.com:org/repo.git"
          disabled={remotes.submitting}
          oninput={(e) => remotes.setFetchUrl(e.currentTarget.value)}
          data-testid="git-remote-fetch-input"
        />
      </div>

      <div class="space-y-1.5">
        <Label for="git-remote-push-input">
          独立推送地址 (Push URL，选填，留空则与获取地址一致)
        </Label>
        <Input
          id="git-remote-push-input"
          mono
          value={remotes.pushUrl}
          placeholder="留空则默认复用获取地址"
          disabled={remotes.submitting}
          oninput={(e) => remotes.setPushUrl(e.currentTarget.value)}
          data-testid="git-remote-push-input"
        />
      </div>

      {#if hasMask}
        <InlineNotice tone="warning">
          检测到脱敏占位符 <span class="font-mono">***</span
          >，请填写完整的真实远程地址后再保存。
        </InlineNotice>
      {/if}

      <GitDialogFooter
        dialogError={remotes.dialogError}
        submitting={remotes.submitting}
        canConfirm={canSave}
        confirmLabel="保存远程配置"
        submittingLabel="正在保存…"
        confirmTestId="git-remote-save-btn"
        onConfirm={() => void remotes.confirmSave()}
        onClose={() => remotes.closeDialogs()}
      />
    </div>
  </ResponsiveDialog>
{/if}

{#if remotes.removeTarget}
  <ResponsiveDialog
    title={`确认移除远程 · ${remotes.removeTarget.name}`}
    description="移除远程仓库配置会同步清理本地关联的远程跟踪分支引用（refs/remotes/<name>/*）。"
    drawerDescription="确认移除此远程仓库配置。"
    busy={remotes.submitting}
    class="sm:max-w-md"
    onclose={() => remotes.closeDialogs()}
  >
    <div class="space-y-3" data-testid="git-remote-remove-dialog">
      <div
        class="space-y-1 rounded-lg border border-git-border bg-git-panel p-3 text-xs"
      >
        <p class="font-mono font-semibold text-foreground">
          {remotes.removeTarget.name}
        </p>
        <p class="font-mono break-all text-muted-foreground">
          {remotes.removeTarget.fetchUrl}
        </p>
      </div>

      <GitDialogFooter
        dialogError={remotes.dialogError}
        submitting={remotes.submitting}
        confirmLabel="确认移除远程"
        submittingLabel="正在移除…"
        confirmVariant="destructive"
        confirmTestId="git-remote-remove-confirm-btn"
        onConfirm={() => void remotes.confirmRemove()}
        onClose={() => remotes.closeDialogs()}
      />
    </div>
  </ResponsiveDialog>
{/if}
