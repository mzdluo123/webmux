<script lang="ts">
  import { untrack, tick, onMount } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import * as Command from "$lib/components/ui/command";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconFolder from "~icons/tabler/folder";
  import IconFolderPlus from "~icons/tabler/folder-plus";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconBrandGithub from "~icons/tabler/brand-github";
  import IconPlus from "~icons/tabler/plus";
  import { pluginAvailable } from "../../platform/plugin-host.svelte";
  import {
    createProjectSources,
    type ProjectSources,
  } from "../../hooks/workspace/project-sources.svelte";
  import type { ProjectSelection } from "../../domains/sessions/project-selection";
  import ChoiceDetails from "../shared/ChoiceDetails.svelte";
  import ProjectSourceForm from "./ProjectSourceForm.svelte";
  import GitHubProjectSource from "./GitHubProjectSource.svelte";

  let {
    defaultCwd,
    recent = [],
    onselect,
    controller,
    initialStep = "projects",
  }: {
    defaultCwd: string;
    recent?: readonly string[];
    onselect: (selection: ProjectSelection) => void;
    controller?: ProjectSources;
    initialStep?: "projects" | "sources" | "new" | "local" | "git" | "github";
  } = $props();
  const source = untrack(() => controller ?? createProjectSources(onselect));
  type Step = "projects" | "sources" | "new" | "local" | "git" | "github";
  let step = $state<Step>(untrack(() => initialStep));
  let projectQuery = $state("");
  let sourceQuery = $state("");
  let root = $state<HTMLDivElement | null>(null);
  let parent = $state(untrack(() => defaultCwd));
  let name = $state("");
  let path = $state(untrack(() => defaultCwd));
  let clonePath = $state("");
  let url = $state("");
  let initializeGit = $state(false);
  let githubQuery = $state("");
  let githubURL = $state("");
  let githubPath = $state("");
  let searchInput = $state<HTMLInputElement | null>(null);
  const recentCandidates = $derived(
    [...new Set(recent)]
      .filter((dir) => !source.projects.some((project) => project.path === dir))
      .slice(0, 5),
  );
  const title = $derived(
    {
      projects: "选择项目",
      sources: "添加项目",
      new: "新建项目",
      local: "本地文件夹",
      git: "Git URL",
      github: "GitHub 仓库",
    }[step],
  );
  const cloneActive = $derived(
    source.operation?.state === "running" ||
      source.operation?.state === "registering",
  );

  onMount(() => {
    void source.load();
    if (source.pending && pluginAvailable("workspace.clone"))
      void source.resume();
    return () => source.dispose();
  });
  async function go(next: Step) {
    step = next;
    await tick();
    (searchInput ?? root?.querySelector("input"))?.focus();
  }
  export function back(): boolean {
    if (step === "projects") return false;
    void go(step === "sources" ? "projects" : "sources");
    return true;
  }
</script>

<div bind:this={root} class="min-w-0 space-y-4">
  <div class="flex items-center gap-2">
    {#if step !== "projects"}<Button
        variant="ghost"
        size="icon-touch"
        aria-label="返回上一层"
        title="返回上一层（Esc）"
        onclick={back}><IconArrowLeft aria-hidden="true" /></Button
      >{/if}
    <h3 class="text-base font-medium">{title}</h3>
  </div>
  {#if source.error}<p
      role="alert"
      class="break-words text-sm text-destructive"
    >
      {source.error}
    </p>
    <Button
      variant="outline"
      size="sm"
      disabled={source.busy}
      onclick={() => (source.pending ? source.resume() : source.load())}
      >重试</Button
    >{/if}
  {#if source.pending}
    <div role="status" class="space-y-2">
      <p class="text-sm">
        {cloneActive
          ? "正在克隆或登记项目…"
          : source.operation?.state === "succeeded"
            ? "克隆已完成"
            : source.operation?.error || "正在恢复克隆操作…"}
      </p>
      <p class="break-all font-mono text-xs text-muted-foreground">
        {source.pending.path}
      </p>
      {#if source.operation?.state === "registration_failed"}<Button
          variant="outline"
          disabled={source.busy}
          onclick={() => source.retryRegistration()}>重试项目登记</Button
        >{/if}
      {#if source.operation && !cloneActive}<Button
          variant="ghost"
          disabled={source.busy}
          onclick={() => source.clearClone()}>返回添加项目</Button
        >{/if}
    </div>
  {/if}
  {#if step === "projects" || step === "sources"}
    <Command.Root variant="outline">
      <Command.Input
        bind:value={
          () => (step === "projects" ? projectQuery : sourceQuery),
          (next) => {
            if (step === "projects") projectQuery = next;
            else sourceQuery = next;
          }
        }
        bind:ref={searchInput}
        placeholder={step === "projects"
          ? "搜索项目名称或路径…"
          : "选择项目来源…"}
        aria-label={step === "projects" ? "搜索项目" : "搜索项目来源"}
      />
      <Command.List>
        <Command.Empty>没有匹配项，请更换关键词或添加项目。</Command.Empty>
        {#if step === "projects"}
          {#if source.loading}<p
              role="status"
              class="p-3 text-sm text-muted-foreground"
            >
              正在读取项目…
            </p>{/if}
          <Command.Group heading="项目">
            {#each source.projects as project (project.id)}
              <Command.Item
                value={project.id}
                keywords={[project.name, project.path]}
                disabled={source.busy || project.availability !== "available"}
                onSelect={() => source.select(project)}
              >
                <IconFolder aria-hidden="true" />
                <div class="min-w-0 flex-1">
                  <ChoiceDetails
                    primary={project.name}
                    secondary={project.path}
                  />
                  {#if project.error}<p
                      class="break-words text-xs text-destructive"
                    >
                      {project.error}
                    </p>{/if}
                </div>
              </Command.Item>
            {/each}
          </Command.Group>
          {#if recentCandidates.length}<Command.Group heading="最近文件夹">
              {#each recentCandidates as dir (dir)}<Command.Item
                  value={dir}
                  keywords={[dir]}
                  disabled={source.busy}
                  onSelect={() => {
                    path = dir;
                    void go("local");
                  }}
                  ><IconFolder aria-hidden="true" /><ChoiceDetails
                    primary={dir.split("/").pop() || dir}
                    secondary={dir}
                  /></Command.Item
                >{/each}
            </Command.Group>{/if}
        {/if}
        {#if step === "sources" || (!source.loading && source.projects.length === 0)}
          <Command.Group heading="项目来源">
            <Command.Item value="new" onSelect={() => go("new")}
              ><IconFolderPlus aria-hidden="true" /><ChoiceDetails
                primary="新建项目"
                secondary="创建文件夹，可选初始化 Git"
              /></Command.Item
            >
            <Command.Item value="local" onSelect={() => go("local")}
              ><IconFolder aria-hidden="true" /><ChoiceDetails
                primary="本地文件夹"
                secondary="打开服务端已有项目"
              /></Command.Item
            >
            <Command.Item
              value="git"
              disabled={!pluginAvailable("workspace.clone")}
              onSelect={() => go("git")}
              ><IconGitBranch aria-hidden="true" /><ChoiceDetails
                primary="Git URL"
                secondary={pluginAvailable("workspace.clone")
                  ? "克隆 HTTPS 或 SSH 仓库"
                  : "服务端 Git 或克隆存储暂不可用"}
              /></Command.Item
            >
            <Command.Item
              value="github"
              disabled={!pluginAvailable("workspace.github")}
              onSelect={() => go("github")}
              ><IconBrandGithub aria-hidden="true" /><ChoiceDetails
                primary="GitHub 仓库"
                secondary={pluginAvailable("workspace.github")
                  ? "浏览已登录账号有权访问的仓库"
                  : "服务端需要可用的 gh、Git 与克隆存储"}
              /></Command.Item
            >
          </Command.Group>
        {/if}
      </Command.List>
    </Command.Root>
    {#if step === "projects" && source.projects.length}
      <Button
        variant="outline"
        disabled={source.busy}
        onclick={() => {
          void go("sources");
        }}><IconPlus aria-hidden="true" />添加项目…</Button
      >
    {/if}
  {:else if step === "github"}
    <GitHubProjectSource
      controller={source}
      bind:query={githubQuery}
      bind:url={githubURL}
      bind:path={githubPath}
    />
  {:else}
    <ProjectSourceForm
      source={step}
      controller={source}
      bind:parent
      bind:name
      bind:path={
        () => (step === "git" ? clonePath : path),
        (next) => {
          if (step === "git") clonePath = next;
          else path = next;
        }
      }
      bind:url
      bind:initializeGit
    />
  {/if}
</div>
