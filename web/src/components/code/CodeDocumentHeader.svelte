<script lang="ts">
  import { onMount, type Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import * as NativeSelect from "$lib/components/ui/native-select";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconPhoto from "~icons/tabler/photo";
  import IconSearch from "~icons/tabler/search";
  import type { CodeReaderPanel } from "../../domains/code/code-reader-controls";
  import { copyText } from "../../platform/clipboard";
  import type { CodeDiff, CodeFile } from "../../contracts/schemas";
  import type { MarkdownHeading } from "./CodeMarkdownPreview.svelte";
  import { formatBytes } from "../../domains/code/code-image";
  import {
    previewFormatLabel,
    type PreviewFormat,
  } from "../../domains/code/code-preview";
  import { sourceLabel } from "../../domains/code/code-diff-client";

  let {
    doc,
    file,
    diffAvailable = true,
    busy,
    shownDiff,
    mode = $bindable<"file" | "diff">("file"),
    side = $bindable<"old" | "new">("new"),
    formatFile,
    formatPreview,
    headings,
    svgFile,
    svgView,
    svgNotice,
    status,
    unified = false,
    canSearch = false,
    searchOpen = false,
    readerMenu,
    onopenpanel,
    quick = $bindable(),
    menu = $bindable(),
    oncompare,
    onshowformat,
    onshowsvg,
    onscrolltoheading,
    onclearselection,
    onfull,
    onrefresh,
  }: {
    doc: CodeFile["document"] | CodeDiff["old"] | undefined;
    file?: CodeFile;
    diffAvailable?: boolean;
    busy: boolean;
    shownDiff?: CodeDiff;
    mode: "file" | "diff";
    side: "old" | "new";
    formatFile?: PreviewFormat;
    formatPreview: boolean;
    headings: MarkdownHeading[];
    svgFile: boolean;
    svgView: "preview" | "source";
    svgNotice: string;
    unified?: boolean;
    canSearch?: boolean;
    searchOpen?: boolean;
    readerMenu?: Snippet;
    onopenpanel?: (panel: CodeReaderPanel) => void;
    quick?: Snippet;
    status: string;
    menu?: Snippet;
    oncompare: () => void;
    onshowformat: (view: "preview" | "source") => void;
    onshowsvg: (view: "preview" | "source") => void;
    onscrolltoheading: (slug: string) => void;
    onclearselection: () => void;
    onfull: (which: "old" | "new") => void;
    onrefresh: () => void;
  } = $props();

  const diffSides = ["new", "old"] as const;
  const outline = $derived(
    formatFile === "markdown" && formatPreview && headings.length >= 2,
  );
  const outlineTop = $derived(Math.min(...headings.map((h) => h.level)));
  // Outline entries indent one step per heading level below the top one.
  const indents = ["", "ps-3", "ps-6", "ps-9", "ps-12", "ps-15"];

  const imageChange = $derived(
    shownDiff?.kind !== "image"
      ? ""
      : shownDiff.old.source === "empty"
        ? m.s5_image_added()
        : shownDiff.new.source === "empty"
          ? m.s5_image_deleted()
          : m.s5_image_modified(),
  );
  const renamed = $derived(
    !!shownDiff && shownDiff.change.oldPath !== shownDiff.change.newPath,
  );
  const quickPreviewActive = $derived(
    svgFile ? svgView === "preview" : formatPreview,
  );
  const quickPreviewLabel = $derived(
    quickPreviewActive ? m.s5_back_to_source() : m.s5_preview(),
  );
  function togglePreview() {
    const next = quickPreviewActive ? "source" : "preview";
    if (svgFile) onshowsvg(next);
    else onshowformat(next);
  }
  function openPanelFromMenu(panel: CodeReaderPanel) {
    const open = onopenpanel;
    // Let the dropdown release its focus scope before focusing the panel.
    if (open) requestAnimationFrame(() => open(panel));
  }
  const unreadable = $derived(
    shownDiff?.kind === "metadata" ||
      (doc?.kind !== "text" && doc?.kind !== "image" && doc?.kind !== "pdf"),
  );
  // Reading notices remain visible when commands move into the code menu.
  const notices = $derived(
    !!imageChange || status !== "unchanged" || unreadable || !!svgNotice,
  );
  const sideSize = (value: CodeDiff["old"]) =>
    value.source === "empty" ? m.s5_source_missing() : formatBytes(value.size);

  onMount(() => {
    menu = documentMenu;
    if (unified) quick = quickCommands;
  });
</script>

{#snippet quickCommands()}
  {#if canSearch}
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={m.s5_find_in_file()}
      aria-expanded={searchOpen}
      aria-keyshortcuts="Control+f Meta+f"
      title={m.s5_find_in_file_shortcut()}
      onclick={() => onopenpanel?.("search")}
      ><IconSearch aria-hidden="true" /></Button
    >
  {/if}
  {#if formatFile || svgFile}
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={quickPreviewLabel}
      aria-pressed={quickPreviewActive}
      title={quickPreviewLabel}
      onclick={togglePreview}><IconPhoto aria-hidden="true" /></Button
    >
  {/if}
{/snippet}

<!-- Markdown headings, indented by level, scroll the preview when chosen. -->
{#snippet outlineItems()}
  {#each headings as item, index (index)}
    <DropdownMenu.Item onSelect={() => onscrolltoheading(item.slug)}
      ><span class={["truncate", indents[item.level - outlineTop]]}
        >{item.text}</span
      ></DropdownMenu.Item
    >
  {/each}
{/snippet}

<!-- The workspace renders this beside the selected file tab. -->
{#snippet documentMenu()}
  <DropdownMenu.Label variant="title" class="break-all">
    {doc?.absolutePath}
    {#if renamed}
      <span class="mt-1 block text-muted-foreground"
        >{shownDiff?.change.oldPath} → {shownDiff?.change.newPath}</span
      >
    {/if}
  </DropdownMenu.Label>
  {#if unified && doc?.absolutePath}
    {@const path = doc.absolutePath}
    <DropdownMenu.Item onSelect={() => void copyText(path)}
      >{m.s8_copy_path()}</DropdownMenu.Item
    >
  {/if}
  {#if unified && onopenpanel}
    <DropdownMenu.Separator />
    <DropdownMenu.Item
      disabled={!canSearch}
      aria-keyshortcuts="Control+f Meta+f"
      onSelect={() => openPanelFromMenu("search")}
      >{m.s5_find_in_file()}</DropdownMenu.Item
    >
    <DropdownMenu.Item
      disabled={!canSearch}
      aria-keyshortcuts="Control+Alt+g Meta+Alt+g"
      onSelect={() => openPanelFromMenu("goto")}
      >{m.s5_goto_line_btn()}</DropdownMenu.Item
    >
    <DropdownMenu.Item onSelect={() => openPanelFromMenu("zoom")}
      >{m.s5_zoom()}</DropdownMenu.Item
    >
    {@render readerMenu?.()}
    <DropdownMenu.Separator />
  {/if}
  {#if file && diffAvailable}
    <DropdownMenu.Item
      disabled={busy}
      onSelect={() => (shownDiff ? (mode = "file") : oncompare())}
      >{shownDiff ? m.s5_back_to_file() : m.s5_view_diff()}</DropdownMenu.Item
    >
  {/if}
  {#if formatFile}
    <DropdownMenu.Item
      onSelect={() => onshowformat(formatPreview ? "source" : "preview")}
      >{formatPreview
        ? m.s5_back_to_source()
        : previewFormatLabel(formatFile)}</DropdownMenu.Item
    >
  {/if}
  {#if outline}
    <DropdownMenu.Sub>
      <DropdownMenu.SubTrigger>{m.s5_outline()}</DropdownMenu.SubTrigger>
      <DropdownMenu.SubContent class="max-h-[60svh] overflow-y-auto">
        {@render outlineItems()}
      </DropdownMenu.SubContent>
    </DropdownMenu.Sub>
  {/if}
  {#if svgFile}
    <DropdownMenu.Separator />
    <DropdownMenu.RadioGroup
      value={svgView}
      onValueChange={(value) => onshowsvg(value as "preview" | "source")}
    >
      <DropdownMenu.GroupHeading
        >{m.s5_svg_display_mode()}</DropdownMenu.GroupHeading
      >
      <!-- The view changes underneath; close like the other entries. -->
      <DropdownMenu.RadioItem value="preview" closeOnSelect
        >{m.s5_preview()}</DropdownMenu.RadioItem
      >
      <DropdownMenu.RadioItem value="source" closeOnSelect
        >{m.s5_source()}</DropdownMenu.RadioItem
      >
    </DropdownMenu.RadioGroup>
  {/if}
  {#if shownDiff}
    {#if imageChange}
      <DropdownMenu.Separator />
      <DropdownMenu.RadioGroup
        value={side}
        onValueChange={(value) => {
          side = value as "old" | "new";
          onclearselection();
        }}
      >
        <DropdownMenu.GroupHeading
          >{m.s5_preview_source_side()}</DropdownMenu.GroupHeading
        >
        {#each diffSides as which (which)}
          <DropdownMenu.RadioItem
            value={which}
            closeOnSelect
            disabled={shownDiff[which].source === "empty"}
            >{which === "old" ? m.s5_before() : m.s5_after()}<span
              class="ml-auto text-xs text-muted-foreground"
              >{sourceLabel(shownDiff[which].source)}</span
            ></DropdownMenu.RadioItem
          >
        {/each}
      </DropdownMenu.RadioGroup>
    {/if}
    <DropdownMenu.Separator />
    <DropdownMenu.Group>
      <DropdownMenu.GroupHeading>{m.s5_full_file()}</DropdownMenu.GroupHeading>
      {@render fullItems()}
    </DropdownMenu.Group>
  {/if}
  <DropdownMenu.Separator />
  <DropdownMenu.Item onSelect={onrefresh}
    >{m.s5_refresh_document()}</DropdownMenu.Item
  >
{/snippet}

{#snippet fullItems()}
  {#if shownDiff}
    {#each diffSides as which (which)}
      {#if shownDiff[which].source !== "empty"}
        <DropdownMenu.Item onSelect={() => onfull(which)}>
          {which === "old" ? m.s5_file_before() : m.s5_file_after()}
          <span class="ml-auto text-xs text-muted-foreground"
            >{sourceLabel(shownDiff[which].source)}</span
          >
        </DropdownMenu.Item>
      {/if}
    {/each}
  {/if}
{/snippet}

{#snippet standaloneBar()}
  <div class="flex items-center gap-2 max-sm:hidden">
    <p
      class="min-w-0 flex-1 truncate text-left text-muted-foreground [direction:rtl]"
      title={doc?.absolutePath}
    >
      <bdi>{doc?.absolutePath}</bdi>
    </p>
    {#if file && diffAvailable}<Button
        variant="ghost"
        size="sm"
        disabled={busy}
        onclick={() => (shownDiff ? (mode = "file") : oncompare())}
        >{shownDiff ? m.s5_back_to_file() : m.s5_view_diff()}</Button
      >{/if}
    {#if formatFile}<Button
        variant="ghost"
        size="sm"
        aria-pressed={formatPreview}
        onclick={() => onshowformat(formatPreview ? "source" : "preview")}
        >{formatPreview ? m.s5_back_to_source() : m.s5_preview()}</Button
      >{/if}
    {#if outline}<DropdownMenu.Root>
        <DropdownMenu.Trigger>
          {#snippet child({ props })}
            <Button {...props} variant="ghost" size="sm">
              {m.s5_outline()}<IconChevronDown aria-hidden="true" />
            </Button>
          {/snippet}
        </DropdownMenu.Trigger>
        <DropdownMenu.Content align="end">
          {@render outlineItems()}
        </DropdownMenu.Content>
      </DropdownMenu.Root>{/if}
    {#if svgFile}<SegmentedControl
        value={svgView}
        onValueChange={(view) =>
          onshowsvg(view === "source" ? "source" : "preview")}
        aria-label={m.s5_svg_display_mode()}
        class="shrink-0"
      >
        <SegmentedControlItem value="preview"
          >{m.s5_preview()}</SegmentedControlItem
        >
        <SegmentedControlItem value="source"
          >{m.s5_source()}</SegmentedControlItem
        >
      </SegmentedControl>{/if}
    {#if shownDiff}
      {#if imageChange}<Label size="sm" class="shrink-0"
          >{m.s5_preview_source_side()}
          <NativeSelect.Root
            size="sm"
            aria-label={m.s5_preview_source_side()}
            bind:value={side}
            onchange={onclearselection}
            ><NativeSelect.Option value="new"
              >{m.s5_after_with_source({
                source: sourceLabel(shownDiff.new.source),
              })}</NativeSelect.Option
            ><NativeSelect.Option value="old"
              >{m.s5_before_with_source({
                source: sourceLabel(shownDiff.old.source),
              })}</NativeSelect.Option
            ></NativeSelect.Root
          ></Label
        >{/if}
      <DropdownMenu.Root>
        <DropdownMenu.Trigger>
          {#snippet child({ props })}
            <Button {...props} variant="ghost" size="sm">
              {m.s5_full_file()}<IconChevronDown aria-hidden="true" />
            </Button>
          {/snippet}
        </DropdownMenu.Trigger>
        <DropdownMenu.Content align="end">
          {@render fullItems()}
        </DropdownMenu.Content>
      </DropdownMenu.Root>
    {/if}
  </div>
  {#if renamed}<p class="break-all max-sm:hidden">
      {shownDiff?.change.oldPath} → {shownDiff?.change.newPath}
    </p>{/if}
{/snippet}

{#if !unified || notices}
  <div
    class={[
      "shrink-0 space-y-1 border-b px-2 py-1 text-xs",
      !notices && "max-sm:hidden",
    ]}
  >
    {#if !unified}{@render standaloneBar()}{/if}
    {#if imageChange && shownDiff}<p>
        <span class="font-medium">{imageChange}</span> ·
        <span aria-label={m.s5_size_change()}
          >{sideSize(shownDiff.old)} → {sideSize(shownDiff.new)}</span
        >
      </p>{/if}
    {#if status !== "unchanged"}<p role="status">
        {m.s5_stale_keep_old_content({
          reason:
            status === "updated"
              ? m.s5_status_updated()
              : status === "missing"
                ? m.s5_status_missing()
                : status === "expired"
                  ? m.s5_status_expired()
                  : m.s5_check_failed_status_unknown(),
        })}<Button variant="outline" size="sm" onclick={onrefresh}
          >{m.s5_refresh_document()}</Button
        >
      </p>{/if}
    {#if unreadable}<p>
        {shownDiff?.reason || doc?.reason}
      </p>{/if}
    {#if svgNotice}<p role="status">{svgNotice}</p>{/if}
  </div>
{/if}
