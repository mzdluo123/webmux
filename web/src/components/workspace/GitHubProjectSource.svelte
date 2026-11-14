<script lang="ts">
  import IconChevronLeft from "~icons/tabler/chevron-left";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import { Button } from "$lib/components/ui/button";
  import { Badge } from "$lib/components/ui/badge";
  import * as Command from "$lib/components/ui/command";
  import type { ProjectSources } from "../../hooks/workspace/project-sources.svelte";
  import ProjectDirectoryField from "./ProjectDirectoryField.svelte";
  import ChoiceDetails from "../shared/ChoiceDetails.svelte";
  let {
    controller,
    query = $bindable(""),
    url = $bindable(""),
    path = $bindable(""),
  }: {
    controller: ProjectSources;
    query: string;
    url: string;
    path: string;
  } = $props();
  $effect(() => {
    const search = query;
    const timer = setTimeout(() => void controller.searchGitHub(search), 250);
    return () => clearTimeout(timer);
  });
  const disabled = $derived(controller.busy || !!controller.pending);
</script>

<div class="space-y-4">
  <p class="text-sm text-muted-foreground">
    使用 WebMux 服务用户的 GitHub 登录。
  </p>
  <Command.Root variant="outline" shouldFilter={false}>
    <Command.Input
      bind:value={query}
      aria-label="搜索 GitHub 仓库"
      placeholder="搜索 owner/name…"
      {disabled}
    />
    <Command.List>
      {#if controller.githubLoading}<p
          role="status"
          class="p-3 text-sm text-muted-foreground"
        >
          正在查询 GitHub…
        </p>
      {:else if controller.github?.available}
        <Command.Empty>没有匹配的仓库</Command.Empty>
        {#each controller.github.repositories as repo (repo.fullName)}
          <Command.Item
            value={repo.fullName}
            {disabled}
            data-checked={url === repo.cloneUrl}
            onSelect={() => (url = repo.cloneUrl)}
          >
            <ChoiceDetails
              primary={repo.fullName}
              secondary={repo.description}
            />
            {#if repo.private}<Badge variant="outline">私有</Badge>{/if}
          </Command.Item>
        {/each}
      {:else}<p role="status" class="p-3 text-sm text-muted-foreground">
          {controller.github?.reason ?? "等待查询服务端 GitHub 身份…"}
        </p>{/if}
    </Command.List>
  </Command.Root>
  {#if controller.github?.available}<p
      class="break-words text-xs text-muted-foreground"
    >
      {controller.github.login} · 第 {controller.github.page} 页
    </p>{/if}
  <div class="flex flex-wrap items-center gap-2">
    <Button
      variant="outline"
      size="sm"
      disabled={controller.githubLoading || disabled}
      onclick={() => controller.searchGitHub(query)}>刷新仓库</Button
    >
    {#if controller.github?.available}
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label="上一页"
        title="上一页"
        disabled={disabled ||
          controller.githubLoading ||
          controller.github.page <= 1}
        onclick={() =>
          controller.searchGitHub(query, (controller.github?.page ?? 1) - 1)}
        ><IconChevronLeft aria-hidden="true" /></Button
      >
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label="下一页"
        title="下一页"
        disabled={disabled ||
          controller.githubLoading ||
          !controller.github.hasMore}
        onclick={() =>
          controller.searchGitHub(query, (controller.github?.page ?? 1) + 1)}
        ><IconChevronRight aria-hidden="true" /></Button
      >
    {/if}
  </div>
  {#if url}<p class="break-all text-sm" title={url}>{url}</p>{/if}
  <ProjectDirectoryField
    id="github-clone-path"
    label="克隆到新目录"
    bind:value={path}
    {disabled}
  />
  <p class="text-sm text-muted-foreground">
    目标必须尚不存在；克隆期间可关闭弹窗，重新打开后继续查看。
  </p>
  <Button
    disabled={disabled || !url || !path || !controller.github?.available}
    onclick={() => controller.clone(url, path, "github")}>克隆并继续</Button
  >
</div>
