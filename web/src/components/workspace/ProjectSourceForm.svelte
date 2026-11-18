<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { Label } from "$lib/components/ui/label";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { featureAvailable } from "../../platform/feature-host.svelte";
  import type { ProjectSources } from "../../hooks/workspace/project-sources.svelte";
  import { suggestProjectClonePath } from "../../domains/workspace/project-clone-path";
  import ProjectDirectoryField from "./ProjectDirectoryField.svelte";
  let {
    source,
    controller,
    cloneParent,
    parent = $bindable(""),
    name = $bindable(""),
    path = $bindable(""),
    url = $bindable(""),
    initializeGit = $bindable(false),
  }: {
    source: "new" | "local" | "git";
    controller: ProjectSources;
    cloneParent: string;
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
        label={m.s8_proj_form_parent_label()}
        bind:value={parent}
      />
      <div class="space-y-2">
        <Label for="project-name">{m.s8_proj_form_name_label()}</Label><Input
          id="project-name"
          bind:value={name}
          aria-describedby={name ? "project-path-preview" : undefined}
          required
          autocomplete="off"
        />
      </div>
      <ScrollArea
        class="h-12"
        type="auto"
        viewportClasses="overscroll-y-contain"
        viewportProps={{ tabindex: name ? 0 : undefined }}
      >
        {#if name}<p
            id="project-path-preview"
            class="pr-3 font-mono text-xs wrap-anywhere text-muted-foreground"
          >
            {m.s8_proj_form_will_create({
              path: `${parent.replace(/\/+$/, "")}/${name}`,
            })}
          </p>{/if}
      </ScrollArea>
      <Label
        ><Checkbox
          bind:checked={initializeGit}
          disabled={!featureAvailable("workspace.manage")}
        />{m.s8_proj_form_init_git()}</Label
      >
      {#if !featureAvailable("workspace.manage")}<p
          class="text-sm text-muted-foreground"
        >
          {m.s8_proj_form_git_unavailable_hint()}
        </p>{/if}
    {:else if source === "local"}
      <ProjectDirectoryField
        id="project-local"
        label={m.s8_proj_form_existing_folder()}
        bind:value={path}
      />
      <p class="text-sm text-muted-foreground">
        {m.s8_proj_form_server_folder_hint()}
      </p>
    {:else}
      <div class="space-y-2">
        <Label for="project-url">{m.s8_proj_form_git_url_label()}</Label><Input
          id="project-url"
          bind:value={url}
          onblur={() => {
            if (!path.trim()) path = suggestProjectClonePath(cloneParent, url);
          }}
          required
          placeholder={m.s8_proj_form_git_url_placeholder()}
          autocapitalize="off"
          spellcheck={false}
        />
      </div>
      <ProjectDirectoryField
        id="project-clone-path"
        label={m.s8_proj_form_clone_path_label()}
        bind:value={path}
      />
      <p class="text-sm text-muted-foreground">
        {m.s8_proj_form_clone_hint()}
      </p>
    {/if}
    <Button
      type="submit"
      wrap={source === "new"}
      disabled={disabled ||
        (source === "git" && !featureAvailable("workspace.clone"))}
      >{controller.busy
        ? m.s8_proj_form_busy()
        : source === "new"
          ? m.s8_proj_form_submit_create()
          : source === "local"
            ? m.s8_proj_form_submit_open()
            : m.s8_proj_form_submit_clone()}</Button
    >
  </fieldset>
</form>
