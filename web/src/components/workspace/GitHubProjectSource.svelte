<script lang="ts">
  import IconChevronLeft from "~icons/tabler/chevron-left";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
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
  const loading = $derived(
    available && (controller.githubLoading || controller.githubQuery !== query),
  );
  const github = $derived(available && !loading ? controller.github : null);
  function updateQuery(next: string) {
    if (next === query) return;
    query = next;
    url = "";
  }
</script>

<div class="space-y-4">
  <p class="text-sm text-muted-foreground">
    {m.s6_github_auth_hint()}
  </p>
  <Command.Root variant="outline" shouldFilter={false}>
    <Command.Input
      bind:value={() => query, updateQuery}
      aria-label={m.s6_github_search_aria()}
      placeholder={m.s6_github_search_placeholder()}
      disabled={disabled || !available}
    />
    <Command.List class="h-56">
      {#if !available}<p
          role="status"
          class="p-3 text-sm text-muted-foreground"
        >
          {refreshError || m.s6_github_unavailable()}
        </p>
      {:else if loading}<p
          role="status"
          class="p-3 text-sm text-muted-foreground"
        >
          {m.s6_github_querying()}
        </p>
      {:else if controller.githubError}
        <p role="alert" class="p-3 wrap-anywhere text-sm text-destructive">
          {controller.githubError}
        </p>
      {:else if github?.available}
        <Command.Empty>{m.s6_github_no_matches()}</Command.Empty>
        {#each github.repositories as repo (repo.fullName)}
          <Command.Item
            value={repo.fullName}
            {disabled}
            data-checked={url === repo.cloneUrl}
            onSelect={() => {
              if (loading || !available) return;
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
      {:else}<p
          role="status"
          class="p-3 wrap-anywhere text-sm text-muted-foreground"
        >
          {refreshError || github?.reason || m.s6_github_waiting_identity()}
        </p>{/if}
    </Command.List>
  </Command.Root>
  <ScrollArea class="h-12" viewportClasses="overscroll-y-contain wrap-anywhere">
    {#if github?.available}
      <p class="text-xs text-muted-foreground">
        {m.s6_github_login_page({
          login: github.login,
          page: String(github.page),
        })}
      </p>
    {/if}
    {#if url}<p class="text-sm" title={url}>{url}</p>{/if}
  </ScrollArea>
  <div class="flex flex-wrap items-center gap-2">
    <Button
      variant="outline"
      size="sm"
      disabled={loading || disabled}
      onclick={retry}
      >{available ? m.s6_github_refresh_repos() : m.s6_github_recheck()}</Button
    >
    <Button
      variant="ghost"
      size="icon-sm"
      class={github?.available ? undefined : "invisible"}
      aria-label={m.s6_prev_page()}
      title={m.s6_prev_page()}
      disabled={disabled || !github?.available || github.page <= 1}
      onclick={() => github && controller.searchGitHub(query, github.page - 1)}
      ><IconChevronLeft aria-hidden="true" /></Button
    >
    <Button
      variant="ghost"
      size="icon-sm"
      class={github?.available ? undefined : "invisible"}
      aria-label={m.s6_next_page()}
      title={m.s6_next_page()}
      disabled={disabled || !github?.available || !github.hasMore}
      onclick={() => github && controller.searchGitHub(query, github.page + 1)}
      ><IconChevronRight aria-hidden="true" /></Button
    >
  </div>
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
    disabled={disabled || !available || !url || !path || !github?.available}
    onclick={() => controller.clone(url, path, "github")}
    >{m.s6_github_clone_continue()}</Button
  >
</div>
