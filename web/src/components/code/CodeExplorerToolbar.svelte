<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { errorMessage } from "../../platform/error";
  import type { z } from "zod";
  import { CodeContextSchema, type ReportedCwd } from "../../contracts/schemas";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import * as Popover from "$lib/components/ui/popover";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import IconSearch from "~icons/tabler/search";
  import IconRefresh from "~icons/tabler/refresh";
  import IconFold from "~icons/tabler/fold";
  import IconDots from "~icons/tabler/dots";
  import IconFileSearch from "~icons/tabler/file-search";

  let {
    context,
    pending,
    root,
    sessionCwd,
    reportedCwd,
    query = $bindable(""),
    directory = $bindable(""),
    includeIgnored = $bindable(false),
    collapse = $bindable(0),
    load,
    reload,
    locate,
  }: {
    context: z.infer<typeof CodeContextSchema> | undefined;
    pending: boolean;
    root: "workspace" | "artifacts";
    sessionCwd: string;
    reportedCwd: ReportedCwd | null;
    query: string;
    directory: string;
    includeIgnored: boolean;
    collapse: number;
    load: (append?: boolean) => void | Promise<void>;
    reload: () => void;
    locate: (input: string) => Promise<void>;
  } = $props();

  let searchOpen = $state(false);
  let searchInput = $state("");
  let locateOpen = $state(false);
  let locateInput = $state("");
  let locateError = $state("");
  const locateBase = $derived.by(() => {
    // The artifacts root resolves relative paths against the folder itself.
    if (root === "artifacts")
      return context
        ? { path: context.rootPath, label: m.s5_artifacts_folder() }
        : null;
    if (reportedCwd?.remote) return null;
    if (reportedCwd)
      return { path: reportedCwd.path, label: m.s5_shell_reported_cwd() };
    return sessionCwd
      ? { path: sessionCwd, label: m.s5_startup_cwd_unreported() }
      : null;
  });
  const locateHint = $derived(
    reportedCwd?.remote && root !== "artifacts"
      ? m.s5_remote_cwd_cannot_locate({ host: reportedCwd.host })
      : locateBase
        ? m.s5_locate_base_hint({
            label: locateBase.label,
            path: locateBase.path,
          })
        : m.s5_locate_no_base(),
  );
  let locating = $state(false);
  async function submitLocate(event: SubmitEvent) {
    event.preventDefault();
    locateError = "";
    if (!context || !locateBase || locating) return;
    locating = true;
    try {
      await locate(locateInput);
      locateInput = "";
      locateOpen = false;
    } catch (err) {
      locateError = errorMessage(err);
    } finally {
      locating = false;
    }
  }
</script>

<div class="flex shrink-0 items-center gap-0.5 px-2 py-1">
  <span
    class="min-w-0 flex-1 truncate text-xs font-medium text-muted-foreground"
    >{m.s5_explorer()}</span
  >
  <Button
    variant="ghost"
    size="icon-xs"
    aria-label={m.s5_toggle_path_search()}
    title={m.s5_search_file_path()}
    aria-expanded={searchOpen}
    onclick={() => {
      searchOpen = !searchOpen;
      if (!searchOpen && query) {
        query = "";
        void load();
      }
    }}><IconSearch aria-hidden="true" /></Button
  >
  <Popover.Root bind:open={locateOpen}>
    <Popover.Trigger>
      {#snippet child({ props })}<Button
          {...props}
          variant="ghost"
          size="icon-xs"
          aria-label={m.s5_open_by_path()}
          title={m.s5_open_by_path()}
          ><IconFileSearch aria-hidden="true" /></Button
        >{/snippet}
    </Popover.Trigger>
    <Popover.Content align="start" class="w-72">
      <Popover.Header
        ><Popover.Title>{m.s5_open_by_path()}</Popover.Title
        ><Popover.Description>{locateHint}</Popover.Description></Popover.Header
      >
      <form class="mt-2 flex flex-col gap-2" onsubmit={submitLocate}>
        <Input
          size="sm"
          aria-label={m.s5_locate_file_path()}
          placeholder={m.s5_relative_path_placeholder()}
          bind:value={locateInput}
          disabled={!locateBase}
        />
        <Button
          size="sm"
          variant="outline"
          type="submit"
          disabled={!locateBase || locating || !locateInput.trim()}
          >{m.s5_open()}</Button
        >
      </form>
      {#if locateError}<p
          role="alert"
          class="mt-1 break-all text-xs text-destructive"
        >
          {locateError}
        </p>{/if}
    </Popover.Content>
  </Popover.Root>
  <Button
    variant="ghost"
    size="icon-xs"
    aria-label={m.s5_refresh_file_tree()}
    title={m.s5_refresh_file_tree()}
    disabled={pending}
    onclick={reload}><IconRefresh aria-hidden="true" /></Button
  >
  <Button
    variant="ghost"
    size="icon-xs"
    aria-label={m.s5_collapse_all()}
    title={m.s5_collapse_all()}
    onclick={() => {
      collapse++;
      query = "";
      searchInput = "";
      directory = "";
      void load();
    }}><IconFold aria-hidden="true" /></Button
  >
  <DropdownMenu.Root>
    <DropdownMenu.Trigger>
      {#snippet child({ props })}<Button
          {...props}
          variant="ghost"
          size="icon-xs"
          aria-label={m.s5_file_nav_options()}
          title={m.s5_file_nav_options()}
          ><IconDots aria-hidden="true" /></Button
        >{/snippet}
    </DropdownMenu.Trigger>
    <DropdownMenu.Content align="end">
      <DropdownMenu.CheckboxItem
        closeOnSelect
        checked={includeIgnored}
        disabled={!context?.ignoreAvailable}
        onCheckedChange={(checked) => {
          includeIgnored = checked;
          void load();
        }}>{m.s5_show_ignored()}</DropdownMenu.CheckboxItem
      >
    </DropdownMenu.Content>
  </DropdownMenu.Root>
</div>
{#if searchOpen}
  <form
    class="flex shrink-0 gap-1 px-2 pb-2"
    onsubmit={(event) => {
      event.preventDefault();
      query = searchInput;
      directory = "";
      void load();
    }}
  >
    <Input
      size="sm"
      aria-label={m.s5_search_file_path()}
      placeholder={m.s5_filename_or_path_placeholder()}
      bind:value={searchInput}
    />
    <Button size="sm" variant="outline" type="submit"
      >{m.s5_search_paths_btn()}</Button
    >
  </form>
{/if}
{#if includeIgnored}<p class="px-2 pb-1 text-xs text-muted-foreground">
    {m.s5_including_ignored()}
  </p>{/if}
