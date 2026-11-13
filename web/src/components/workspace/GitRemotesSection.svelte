<script lang="ts">
  import IconCopy from "~icons/tabler/copy";
  import IconEdit from "~icons/tabler/edit";
  import IconPlus from "~icons/tabler/plus";
  import IconTrash from "~icons/tabler/trash";
  import IconWorldOff from "~icons/tabler/world-off";
  import { Button } from "$lib/components/ui/button";
  import * as Empty from "$lib/components/ui/empty";
  import { copyText } from "../../platform/clipboard";
  import type { Remote } from "../../contracts/schemas";
  import type { GitRemotesController } from "../../hooks/workspace/git-remotes-controller.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    remotes,
    controller,
    error = "",
    compact = false,
    onretry,
  }: {
    remotes: readonly Remote[] | undefined;
    controller?: GitRemotesController;
    error?: string;
    compact?: boolean;
    onretry?: () => void;
  } = $props();
</script>

{#snippet urlRow(label: string, name: string, value: string)}
  <dt class="text-muted-foreground">{label}</dt>
  <dd class="flex min-w-0 items-start gap-1">
    <span class="min-w-0 flex-1 font-mono break-all">{value}</span>
    <Button
      size="icon-sm"
      variant="ghost"
      aria-label={`复制 ${name} 的${label}地址`}
      title={`复制 ${name} 的${label}地址`}
      onclick={() => void copyText(value)}
    >
      <IconCopy aria-hidden="true" />
    </Button>
  </dd>
{/snippet}

<div class="space-y-2" data-testid="git-remotes-section">
  {#if controller}
    <div class="flex flex-wrap items-center justify-between gap-2">
      <span class="text-xs font-semibold text-muted-foreground">
        远程仓库 ({remotes?.length ?? 0})
      </span>
      <Button
        size="sm"
        variant="outline"
        onclick={() => controller.openAddDialog()}
        data-testid="git-remote-add-btn"
      >
        <IconPlus aria-hidden="true" />
        添加远程…
      </Button>
    </div>
  {/if}

  {#if error}
    <div class="space-y-2">
      <InlineNotice tone="danger">无法读取远程：{error}</InlineNotice>
      {#if onretry}
        <Button size="sm" variant="outline" onclick={onretry}>重新读取</Button>
      {/if}
    </div>
  {:else if !remotes}
    <p role="status" class="text-xs text-muted-foreground">正在读取远程…</p>
  {:else if remotes.length === 0}
    {#if compact}
      <p class="px-1.5 py-1 text-xs text-muted-foreground">
        尚未配置远程仓库。
      </p>
    {:else}
      <Empty.Root variant="outline">
        <Empty.Header>
          <Empty.Media variant="icon">
            <IconWorldOff aria-hidden="true" />
          </Empty.Media>
          <Empty.Title>没有远程</Empty.Title>
          <Empty.Description>
            这个仓库还没有配置远程；可点击上方「添加远程…」或在终端用 git remote
            add 添加。
          </Empty.Description>
        </Empty.Header>
      </Empty.Root>
    {/if}
  {:else}
    <ul class="space-y-2" aria-label="远程列表">
      {#each remotes as remote (remote.name)}
        <li
          class="space-y-2 rounded-lg border bg-background p-2.5"
          data-remote-row={remote.name}
        >
          <div class="flex min-w-0 items-center justify-between gap-2">
            <p class="min-w-0 flex-1 font-mono text-xs font-semibold break-all">
              {remote.name}
            </p>
            {#if controller}
              <div class="flex shrink-0 items-center gap-1">
                <Button
                  size="icon-sm"
                  variant="ghost"
                  aria-label={`编辑远程 ${remote.name}`}
                  title={`编辑远程 ${remote.name}`}
                  onclick={() => controller.openEditDialog(remote)}
                >
                  <IconEdit aria-hidden="true" />
                </Button>
                <Button
                  size="icon-sm"
                  variant="ghost"
                  aria-label={`移除远程 ${remote.name}`}
                  title={`移除远程 ${remote.name}`}
                  onclick={() => controller.openRemoveDialog(remote)}
                >
                  <IconTrash aria-hidden="true" />
                </Button>
              </div>
            {/if}
          </div>
          <dl
            class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1.5 text-xs"
          >
            {#if remote.fetchUrl === remote.pushUrl}
              {@render urlRow("获取与推送", remote.name, remote.fetchUrl)}
            {:else}
              {@render urlRow("获取", remote.name, remote.fetchUrl)}
              {@render urlRow("推送", remote.name, remote.pushUrl)}
            {/if}
          </dl>
        </li>
      {/each}
    </ul>
  {/if}
</div>
