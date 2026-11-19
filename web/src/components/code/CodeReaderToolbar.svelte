<script lang="ts">
  import type { Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import IconSearch from "~icons/tabler/search";
  import IconListNumbers from "~icons/tabler/list-numbers";
  import IconZoomIn from "~icons/tabler/zoom-in";
  import IconNumber from "~icons/tabler/number";
  import IconTextWrap from "~icons/tabler/text-wrap";
  import type { CodeReaderPanel } from "../../domains/code/code-reader-controls";
  let {
    panel,
    open,
    totals,
    searchable = true,
    pressed,
    toggle,
  }: {
    panel?: CodeReaderPanel;
    open: (panel: CodeReaderPanel) => void;
    totals?: Snippet;
    searchable?: boolean;
    pressed: boolean;
    toggle: () => void;
  } = $props();
</script>

<div
  class="flex min-w-0 shrink-0 flex-wrap items-center gap-1 border-b px-1 py-0.5"
>
  {#if totals}<span class="mr-auto">{@render totals()}</span>{/if}
  <Button
    variant="ghost"
    size="icon-sm"
    aria-label={m.s5_find_in_file()}
    aria-expanded={panel === "search"}
    aria-keyshortcuts="Control+f Meta+f"
    title={m.s5_find_in_file_shortcut()}
    disabled={!searchable}
    onclick={() => open("search")}><IconSearch aria-hidden="true" /></Button
  >
  {#if !totals}<Button
      variant="ghost"
      size="icon-sm"
      aria-label={m.s5_goto_line_btn()}
      aria-expanded={panel === "goto"}
      aria-keyshortcuts="Control+Alt+g Meta+Alt+g"
      title={m.s5_goto_line_shortcut()}
      onclick={() => open("goto")}
      ><IconListNumbers aria-hidden="true" /></Button
    >{/if}
  <Button
    variant="ghost"
    size="icon-sm"
    aria-label={m.s5_zoom()}
    title={m.s5_zoom()}
    aria-expanded={panel === "zoom"}
    onclick={() => open("zoom")}><IconZoomIn aria-hidden="true" /></Button
  >
  <Button
    variant="ghost"
    size="icon-sm"
    aria-label={totals ? m.s5_word_wrap() : m.s5_show_line_numbers()}
    title={totals
      ? m.s5_word_wrap()
      : pressed
        ? m.s5_hide_line_numbers()
        : m.s5_show_line_numbers()}
    aria-pressed={pressed}
    onclick={toggle}
    >{#if totals}<IconTextWrap aria-hidden="true" />{:else}<IconNumber
        aria-hidden="true"
      />{/if}</Button
  >
</div>
