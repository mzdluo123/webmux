<script lang="ts">
  import { onMount, untrack } from "svelte";
  import ProjectPicker from "../src/components/workspace/ProjectPicker.svelte";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";
  import { Button } from "../src/lib/components/ui/button";
  import {
    setFeatureCapabilities,
    resetFeatureCapabilities,
  } from "../src/platform/feature-host.svelte";
  import type { ProjectSources } from "../src/hooks/workspace/project-sources.svelte";
  import type { ProjectSelection } from "../src/domains/sessions/project-selection";
  import type {
    Project,
    ProjectCloneOperation,
  } from "../src/contracts/schemas";

  let {
    scene = "projects",
  }: {
    scene?:
      | "projects"
      | "empty"
      | "sources"
      | "new"
      | "local"
      | "git"
      | "github"
      | "github-unavailable"
      | "registration-failure";
  } = $props();
  const initial = untrack(() => scene);
  let ready = $state(false);
  let open = $state(true);
  let selected = $state<ProjectSelection | null>(null);
  let picker = $state<{ back: () => boolean }>();
  const projects: Project[] = [
    {
      id: "directory:demo",
      name: "webmux",
      path: "/home/demo/projects/webmux",
      repositoryId: "",
      availability: "available",
      error: "",
    },
    {
      id: "directory:notes",
      name: "研究笔记",
      path: "/home/demo/projects/research/weekly-notes",
      repositoryId: "",
      availability: "available",
      error: "",
    },
    {
      id: "directory:offline",
      name: "归档项目",
      path: "/mnt/team/非常长的项目目录/archived-client-project",
      repositoryId: "",
      availability: "unavailable",
      error: "目录不可访问，请检查挂载与权限",
    },
  ];
  let operation = $state<ProjectCloneOperation | null>(
    initial === "registration-failure"
      ? {
          id: "a".repeat(32),
          state: "registration_failed",
          path: "/home/demo/projects/new-project",
          error: "仓库已克隆，但项目登记失败；目录已保留，可重试登记",
          projectId: "",
        }
      : null,
  );
  const controller: ProjectSources = {
    projects: initial === "empty" ? [] : projects,
    loading: false,
    busy: false,
    error: "",
    githubLoading: false,
    github: {
      available: initial !== "github-unavailable",
      reason:
        initial === "github-unavailable"
          ? "无法验证服务端 GitHub 身份，请检查服务用户的 gh 登录、权限与网络"
          : "",
      login: "demo",
      repositories: [
        {
          fullName: "demo/webmux",
          description: "持久终端与 CLI 工作区",
          private: true,
          cloneUrl: "https://github.com/demo/webmux.git",
        },
      ],
      page: 1,
      hasMore: false,
    },
    get operation() {
      return operation;
    },
    get pending() {
      return operation
        ? {
            requestId: "storybook-clone",
            url: "https://github.com/demo/webmux.git",
            path: operation.path,
            operationId: operation.id,
          }
        : null;
    },
    load: async () => {},
    select: async (project) => {
      selected = { project };
    },
    open: async (path) => {
      selected = { project: { ...projects[0], path } };
    },
    create: async (parent, name) => {
      selected = {
        project: { ...projects[0], name, path: `${parent}/${name}` },
      };
    },
    clone: async (_url, path) => {
      selected = { project: { ...projects[0], path } };
    },
    resume: async () => {},
    retryRegistration: async () => {
      operation = null;
      selected = { project: projects[0] };
    },
    clearClone: () => {
      operation = null;
    },
    searchGitHub: async () => {},
    dispose: () => {},
  };
  const initialStep =
    initial === "empty" || initial === "registration-failure"
      ? "projects"
      : initial === "github-unavailable"
        ? "github"
        : initial;
  onMount(() => {
    setFeatureCapabilities({
      version: 1,
      features: [{ id: "workspace", enabled: true, status: "ready" }],
      capabilities: [
        { id: "workspace.projects", available: true },
        { id: "workspace.manage", available: true },
        { id: "workspace.clone", available: true },
        { id: "workspace.github", available: true },
      ],
    });
    ready = true;
    return () => resetFeatureCapabilities();
  });
</script>

<div class="p-6">
  <Button
    onclick={() => {
      open = true;
      selected = null;
    }}>新建会话</Button
  >
</div>
{#if ready && open}
  <ResponsiveDialog
    title="新建会话"
    description="选择项目，再设置运行方式。"
    onclose={() => (open = false)}
    onback={() => picker?.back() ?? false}
    pinDrawerHeader
  >
    {#if selected}
      <div class="space-y-4">
        <p class="text-sm">已选择 {selected.project.name}</p>
        <p class="break-all font-mono text-xs">{selected.project.path}</p>
        <Button variant="outline" onclick={() => (selected = null)}
          >返回项目列表</Button
        >
      </div>
    {:else}
      <ProjectPicker
        bind:this={picker}
        defaultCwd="/home/demo/projects"
        recent={["/home/demo/downloads/scratch"]}
        {controller}
        {initialStep}
        onselect={(selection) => (selected = selection)}
      />
    {/if}
  </ResponsiveDialog>
{/if}
