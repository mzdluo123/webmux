<script lang="ts">
  import { usePanelMotion } from "../../hooks/shared/dock-motion.svelte";
  import { untrack, tick, onMount } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import * as Command from "$lib/components/ui/command";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconFolder from "~icons/tabler/folder";
  import IconFolderPlus from "~icons/tabler/folder-plus";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconBrandGithub from "~icons/tabler/brand-github";
  import IconPlus from "~icons/tabler/plus";
  import {
    featureAvailable,
    featureCapabilityRevision,
  } from "../../platform/feature-host.svelte";
  import {
    createProjectSources,
    type ProjectSources,
  } from "../../hooks/workspace/project-sources.svelte";
  import type { ProjectSelection } from "../../domains/sessions/project-selection";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
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
  const ownsController = untrack(() => !controller);
  const source = untrack(() => controller ?? createProjectSources(onselect));
  type Step = "projects" | "sources" | "new" | "local" | "git" | "github";
  let step = $state<Step>(untrack(() => initialStep));
  let body = $state<HTMLDivElement | null>(null);
  usePanelMotion(
    () => body,
    () => step,
  );
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
      projects: m.s8_proj_step_projects(),
      sources: m.s8_proj_step_sources(),
      new: m.s8_proj_step_new(),
      local: m.s8_proj_step_local(),
      git: "Git URL",
      github: m.s8_proj_step_github(),
    }[step],
  );
  const cloneActive = $derived(
    source.operation?.state === "running" ||
      source.operation?.state === "registering",
  );

  let retriedRequest = $state("");
  let cloneDetails = $state<HTMLElement | null>(null);
  const registrationFailed = $derived(
    source.operation?.state === "registration_failed",
  );
  const showRegistrationRetry = $derived(
    registrationFailed ||
      (!source.error &&
        source.operation?.state === "registering" &&
        source.pending?.requestId === retriedRequest),
  );
  const retryUnavailable = $derived(
    source.busy || (showRegistrationRetry && !registrationFailed),
  );
  const retryInProgress = $derived(showRegistrationRetry && retryUnavailable);
  const cloneFailed = $derived(
    !!source.error ||
      (registrationFailed && !retryInProgress) ||
      source.operation?.state === "failed" ||
      source.operation?.state === "interrupted",
  );
  const cloneStatus = $derived(
    source.error ||
      (cloneActive || retryInProgress
        ? m.s8_proj_cloning_or_registering()
        : source.operation?.state === "succeeded"
          ? m.s8_proj_clone_succeeded()
          : source.operation?.error || m.s8_proj_clone_resuming()),
  );

  $effect(() => {
    if (cloneStatus && cloneDetails) cloneDetails.scrollTop = 0;
  });
  function retryClone() {
    if (retryUnavailable || !source.pending) return;
    if (registrationFailed) {
      retriedRequest = source.pending.requestId;
      void source.retryRegistration();
    } else if (source.error) {
      void source.resume();
    }
  }
  function stopTracking() {
    source.clearClone();
    void go(step);
  }

  onMount(() => {
    void source.load();
    return () => {
      if (ownsController) source.dispose();
    };
  });
  $effect(() => {
    featureCapabilityRevision();
    untrack(() => {
      if (source.pending && featureAvailable("workspace.clone"))
        void source.resume();
    });
  });
  async function go(next: Step) {
    step = next;
    await tick();
    (searchInput ?? root?.querySelector("input"))?.focus();
  }
  export function showProjects() {
    void go("projects");
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
        aria-label={m.s8_proj_back_aria()}
        title={m.s8_proj_back_title()}
        onclick={back}><IconArrowLeft aria-hidden="true" /></Button
      >{/if}
    <h3 class="text-base font-medium">{title}</h3>
  </div>
  {#if source.error && !source.pending}<p
      role="alert"
      class="break-words text-sm text-destructive"
    >
      {source.error}
    </p>
    <Button
      variant="outline"
      size="sm"
      disabled={source.busy}
      onclick={() => source.load()}>{m.s8_retry()}</Button
    >{/if}
  {#if source.pending}
    <div class="space-y-2 [overflow-anchor:none]">
      <ScrollArea
        bind:viewportRef={cloneDetails}
        class="h-24"
        type="auto"
        viewportClasses="overscroll-y-contain"
        viewportProps={{
          tabindex: 0,
          role: "region",
          "aria-label": m.s8_proj_clone_status(),
        }}
      >
        <div class="space-y-2 pr-3 wrap-anywhere">
          <p
            role={cloneFailed ? "alert" : "status"}
            class="text-sm"
            class:text-destructive={cloneFailed}
          >
            {cloneStatus}
          </p>
          <p class="font-mono text-xs text-muted-foreground">
            {source.pending.path}
          </p>
        </div>
      </ScrollArea>
      <div class="grid grid-cols-2 items-start justify-items-start gap-2">
        <Button
          variant="outline"
          wrap
          class={showRegistrationRetry || source.error
            ? undefined
            : "invisible"}
          disabled={!showRegistrationRetry && !source.error}
          aria-disabled={retryUnavailable}
          onclick={retryClone}
          >{showRegistrationRetry
            ? m.s8_proj_retry_registration()
            : m.s8_retry()}</Button
        >
        <Button
          variant="ghost"
          wrap
          disabled={source.busy}
          onclick={stopTracking}>{m.s8_proj_stop_tracking()}</Button
        >
      </div>
      <p class="text-xs text-muted-foreground">
        {m.s8_proj_stop_tracking_hint()}
      </p>
    </div>
  {/if}
  <div bind:this={body} class="space-y-4">
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
            ? m.s8_proj_search_placeholder()
            : m.s8_proj_source_placeholder()}
          aria-label={step === "projects"
            ? m.s8_proj_search_aria()
            : m.s8_proj_source_aria()}
        />
        <Command.List>
          <Command.Empty>{m.s8_proj_empty()}</Command.Empty>
          {#if step === "projects"}
            {#if source.loading}<p
                role="status"
                class="p-3 text-sm text-muted-foreground"
              >
                {m.s8_proj_loading()}
              </p>{/if}
            <Command.Group heading={m.s8_proj_group_projects()}>
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
            {#if recentCandidates.length}<Command.Group
                heading={m.s8_proj_group_recent_folders()}
              >
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
            <Command.Group heading={m.s8_proj_group_sources()}>
              <Command.Item
                value="new"
                keywords={[m.s8_proj_source_new_keywords()]}
                onSelect={() => go("new")}
                ><IconFolderPlus aria-hidden="true" /><ChoiceDetails
                  primary={m.s8_proj_step_new()}
                  secondary={m.s8_proj_source_new_desc()}
                /></Command.Item
              >
              <Command.Item
                value="local"
                keywords={[m.s8_proj_source_local_keywords()]}
                onSelect={() => go("local")}
                ><IconFolder aria-hidden="true" /><ChoiceDetails
                  primary={m.s8_proj_step_local()}
                  secondary={m.s8_proj_source_local_desc()}
                /></Command.Item
              >
              <Command.Item
                value="git"
                keywords={[m.s8_proj_source_git_keywords()]}
                disabled={!featureAvailable("workspace.clone")}
                onSelect={() => go("git")}
                ><IconGitBranch aria-hidden="true" /><ChoiceDetails
                  primary="Git URL"
                  secondary={featureAvailable("workspace.clone")
                    ? m.s8_proj_source_git_desc()
                    : m.s8_proj_source_git_unavailable()}
                /></Command.Item
              >
              <Command.Item
                value="github"
                keywords={[m.s8_proj_source_github_keywords()]}
                disabled={source.busy}
                onSelect={() => go("github")}
                ><IconBrandGithub aria-hidden="true" /><ChoiceDetails
                  primary={m.s8_proj_step_github()}
                  secondary={featureAvailable("workspace.github")
                    ? m.s8_proj_source_github_desc()
                    : m.s8_proj_source_github_unavailable()}
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
          }}><IconPlus aria-hidden="true" />{m.s8_proj_add_btn()}</Button
        >
      {/if}
    {:else if step === "github"}
      <GitHubProjectSource
        controller={source}
        parent={defaultCwd}
        bind:query={githubQuery}
        bind:url={githubURL}
        bind:path={githubPath}
      />
    {:else}
      <ProjectSourceForm
        cloneParent={defaultCwd}
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
</div>
