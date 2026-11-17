<script lang="ts">
  import IconChevronLeft from "~icons/tabler/chevron-left";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import { Button } from "$lib/components/ui/button";
  import { Badge } from "$lib/components/ui/badge";
  import * as Command from "$lib/components/ui/command";
  import { m } from "$lib/paraglide/messages.js";
  import {
    featureAvailable,
    requestFeatureHostRefresh,
  } from "../../platform/feature-host.svelte";
  import { errorMessage } from "../../platform/error";
  import type { ProjectSources } from "../../hooks/workspace/project-sources.svelte";
  import { suggestProjectClonePath } from "../../domains/workspace/project-clone-path";
  import ProjectDirectoryField from "./ProjectDirectoryField.svelte";
  import ChoiceDetails from "../shared/ChoiceDetails.svelte";
  let {
    controller,
    parent,
    query = $bindable(""),
    url = $bindable(""),
    path = $bindable(""),
  }: {
    controller: ProjectSources;
    parent: string;
    query: string;
    url: string;
    path: string;
  } = $props();
  const available = $derived(featureAvailable("workspace.github"));
  let refreshError = $state("");
  async function retry() {
    refreshError = "";
    try {
      if (available) await controller.searchGitHub(query);
      else await requestFeatureHostRefresh();
    } catch (error) {
      refreshError = errorMessage(error);
    }
  }
  $effect(() => {
    if (!available) return;
    const search = query;
    const timer = setTimeout(() => void controller.searchGitHub(search), 250);
    return () => clearTimeout(timer);
  });
  const disabled = $derived(controller.busy || !!controller.pending);
</script>

<div class="space-y-4">
  <p class="text-sm text-muted-foreground">
    {m.s6_github_auth_hint()}
  </p>
  <Command.Root variant="outline" shouldFilter={false}>
    <Command.Input
      bind:value={query}
      aria-label={m.s6_github_search_aria()}
      placeholder={m.s6_github_search_placeholder()}
      disabled={disabled || !available}
    />
    <Command.List>
      {#if !available}<p
          role="status"
          class="p-3 text-sm text-muted-foreground"
        >
          {m.s6_github_unavailable()}
        </p>
      {:else if controller.githubLoading}<p
          role="status"
          class="p-3 text-sm text-muted-foreground"
        >
          {m.s6_github_querying()}
        </p>
      {:else if controller.github?.available}
        <Command.Empty>{m.s6_github_no_matches()}</Command.Empty>
        {#each controller.github.repositories as repo (repo.fullName)}
          <Command.Item
            value={repo.fullName}
            {disabled}
            data-checked={url === repo.cloneUrl}
            onSelect={() => {
              url = repo.cloneUrl;
              if (!path.trim()) path = suggestProjectClonePath(parent, url);
            }}
          >
            <ChoiceDetails
              primary={repo.fullName}
              secondary={repo.description}
            />
            {#if repo.private}<Badge variant="outline"
                >{m.s6_github_private_badge()}</Badge
              >{/if}
          </Command.Item>
        {/each}
      {:else}<p role="status" class="p-3 text-sm text-muted-foreground">
          {controller.github?.reason ?? m.s6_github_waiting_identity()}
        </p>{/if}
    </Command.List>
  </Command.Root>
  {#if controller.github?.available}<p
      class="break-words text-xs text-muted-foreground"
    >
      {m.s6_github_login_page({
        login: controller.github.login,
        page: String(controller.github.page),
      })}
    </p>{/if}
  <div class="flex flex-wrap items-center gap-2">
    <Button
      variant="outline"
      size="sm"
      disabled={controller.githubLoading || disabled}
      onclick={retry}
      >{available ? m.s6_github_refresh_repos() : m.s6_github_recheck()}</Button
    >
    {#if available && controller.github?.available}
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={m.s6_prev_page()}
        title={m.s6_prev_page()}
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
        aria-label={m.s6_next_page()}
        title={m.s6_next_page()}
        disabled={disabled ||
          controller.githubLoading ||
          !controller.github.hasMore}
        onclick={() =>
          controller.searchGitHub(query, (controller.github?.page ?? 1) + 1)}
        ><IconChevronRight aria-hidden="true" /></Button
      >
    {/if}
  </div>
  {#if refreshError}<p
      role="alert"
      class="break-words text-sm text-destructive"
    >
      {refreshError}
    </p>{/if}
  {#if url}<p class="break-all text-sm" title={url}>{url}</p>{/if}
  <ProjectDirectoryField
    id="github-clone-path"
    label={m.s6_github_clone_dir_label()}
    bind:value={path}
    disabled={disabled || !available}
  />
  <p class="text-sm text-muted-foreground">
    {m.s6_github_clone_hint()}
  </p>
  <Button
    disabled={disabled ||
      !available ||
      !url ||
      !path ||
      !controller.github?.available}
    onclick={() => controller.clone(url, path, "github")}
    >{m.s6_github_clone_continue()}</Button
  >
</div>
