<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { pluginAvailable } from "../../platform/plugin-host.svelte";
  import type { ProjectSources } from "../../hooks/workspace/project-sources.svelte";
  import ProjectDirectoryField from "./ProjectDirectoryField.svelte";
  let {
    source,
    controller,
    parent = $bindable(""),
    name = $bindable(""),
    path = $bindable(""),
    url = $bindable(""),
    initializeGit = $bindable(false),
  }: {
    source: "new" | "local" | "git";
    controller: ProjectSources;
    parent: string;
    name: string;
    path: string;
    url: string;
    initializeGit: boolean;
  } = $props();
  const disabled = $derived(controller.busy || !!controller.pending);
  function submit(event: SubmitEvent) {
    event.preventDefault();
    if (disabled) return;
    if (source === "new") void controller.create(parent, name, initializeGit);
    else if (source === "local") void controller.open(path);
    else void controller.clone(url, path);
  }
</script>

<form onsubmit={submit} class="space-y-4">
  <fieldset {disabled} class="min-w-0 space-y-4">
    {#if source === "new"}
      <ProjectDirectoryField
        id="project-parent"
        label="存放位置"
        bind:value={parent}
      />
      <div class="space-y-2">
        <Label for="project-name">项目名称</Label><Input
          id="project-name"
          bind:value={name}
          required
          autocomplete="off"
        />
      </div>
      <Label
        ><Checkbox
          bind:checked={initializeGit}
          disabled={!pluginAvailable("workspace.manage")}
        />初始化 Git 仓库</Label
      >
      {#if !pluginAvailable("workspace.manage")}<p
          class="text-sm text-muted-foreground"
        >
          Git 暂不可用，仍可创建普通项目文件夹。
        </p>{/if}
    {:else if source === "local"}
      <ProjectDirectoryField
        id="project-local"
        label="已有文件夹"
        bind:value={path}
      />
      <p class="text-sm text-muted-foreground">
        选择运行 WebMux 的服务端文件夹。
      </p>
    {:else}
      <div class="space-y-2">
        <Label for="project-url">Git 仓库地址</Label><Input
          id="project-url"
          bind:value={url}
          required
          placeholder="https://… 或 git@host:owner/repo.git"
          autocapitalize="off"
          spellcheck={false}
        />
      </div>
      <ProjectDirectoryField
        id="project-clone-path"
        label="克隆到新目录"
        bind:value={path}
      />
      <p class="text-sm text-muted-foreground">
        目标目录必须尚不存在；使用服务端的 Git 凭据。
      </p>
    {/if}
    <Button
      type="submit"
      disabled={disabled ||
        (source === "git" && !pluginAvailable("workspace.clone"))}
      >{controller.busy
        ? "处理中…"
        : source === "new"
          ? "创建项目并继续"
          : source === "local"
            ? "打开文件夹并继续"
            : "克隆并继续"}</Button
    >
  </fieldset>
</form>
