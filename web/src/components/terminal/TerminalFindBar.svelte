<script lang="ts">
  import { tick } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { m } from "$lib/paraglide/messages.js";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconArrowDown from "~icons/tabler/arrow-down";
  import IconCopy from "~icons/tabler/copy";
  import IconX from "~icons/tabler/x";
  import type { TerminalController } from "../../hooks/terminal/terminal.svelte";
  import {
    terminalFindError,
    terminalFindResultText,
    terminalFindScope,
    terminalFindReplayText,
    type TerminalFindOptions,
  } from "../../domains/terminal/terminal-search";

  let {
    open,
    terminal: t,
    onclose,
  }: {
    open: boolean;
    terminal: Pick<
      TerminalController,
      | "canFind"
      | "findResults"
      | "findReplayed"
      | "bufferType"
      | "scrollback"
      | "historyTruncated"
      | "restoring"
      | "selection"
      | "findNext"
      | "findPrevious"
      | "clearFind"
      | "copySelection"
    >;
    onclose: () => void;
  } = $props();
  let input = $state<HTMLInputElement | null>(null);
  let query = $state("");
  let options = $state<TerminalFindOptions>({
    caseSensitive: false,
    wholeWord: false,
    regex: false,
  });
  let composing = $state(false);
  const error = $derived(terminalFindError(query, options.regex));
  const disabled = $derived(
    !t.canFind || t.restoring || !query || !!error || composing,
  );
  const results = $derived(terminalFindResultText(t.findResults));
  const replay = $derived(terminalFindReplayText(t.restoring, t.findReplayed));
  const replayMessages = $derived([
    terminalFindReplayText(true, false),
    terminalFindReplayText(false, true),
  ]);
  $effect(() => {
    if (open)
      void tick().then(() => {
        if (open) focus();
      });
  });
  export function focus() {
    input?.focus({ preventScroll: true });
  }
  function search(previous = false, incremental = false) {
    if (composing) return;
    if (!query) {
      t.clearFind();
      return;
    }
    if (disabled) return;
    if (previous) t.findPrevious(query, options);
    else t.findNext(query, options, incremental);
  }
  function toggle(key: keyof TerminalFindOptions) {
    options[key] = !options[key];
    search(false, true);
  }
  function findKeys(node: HTMLFormElement) {
    const keydown = (event: KeyboardEvent) => {
      if (event.isComposing || composing || event.keyCode === 229) return;
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        onclose();
      } else if (event.key === "Enter" && event.target === input) {
        event.preventDefault();
        search(event.shiftKey);
      }
    };
    node.addEventListener("keydown", keydown);
    return { destroy: () => node.removeEventListener("keydown", keydown) };
  }
</script>

{#if open}
  <form
    role="search"
    aria-label={m.s4_find_terminal_output()}
    class="absolute inset-x-2 top-2 z-20 max-h-[calc(100%-1rem)] overflow-y-auto overscroll-contain rounded-md border bg-popover p-2 text-popover-foreground shadow-md"
    onsubmit={(event) => event.preventDefault()}
    use:findKeys
  >
    <div class="flex flex-wrap items-center gap-1">
      <Input
        bind:ref={input}
        value={query}
        aria-label={m.s4_find_terminal_output()}
        aria-invalid={!!error}
        placeholder={m.s4_find_terminal_output()}
        class="min-w-24 flex-1"
        autocapitalize="off"
        autocorrect="off"
        spellcheck={false}
        oninput={(event) => {
          query = event.currentTarget.value;
          search(false, true);
        }}
        oncompositionstart={() => (composing = true)}
        oncompositionend={(event) => {
          composing = false;
          query = event.currentTarget.value;
          search(false, true);
        }}
      />
      <div class="flex flex-wrap items-center gap-1">
        {#each [{ key: "caseSensitive", label: m.s4_find_case_sensitive(), text: "Aa" }, { key: "wholeWord", label: m.s4_find_whole_word(), text: "W" }, { key: "regex", label: m.s4_find_regex(), text: ".*" }] as const as option (option.key)}
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={option.label}
            title={option.label}
            aria-pressed={options[option.key]}
            onclick={() => toggle(option.key)}
            ><span class="font-mono text-xs">{option.text}</span></Button
          >
        {/each}
        {#each [{ previous: true, label: m.s4_find_prev_match(), shortcut: "Shift+Enter", icon: IconArrowUp }, { previous: false, label: m.s4_find_next_match(), shortcut: "Enter", icon: IconArrowDown }] as direction (direction.shortcut)}
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={direction.label}
            title={m.s4_find_match_with_shortcut({
              label: direction.label,
              shortcut: direction.shortcut,
            })}
            {disabled}
            onclick={() => search(direction.previous)}
            ><direction.icon aria-hidden="true" /></Button
          >
        {/each}
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={m.s4_copy_selection()}
          title={m.s4_copy_selection()}
          disabled={!t.selection}
          onclick={() => t.copySelection()}
          ><IconCopy aria-hidden="true" /></Button
        >
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={m.s4_close_find()}
          title={m.s4_close_find_with_shortcut()}
          onclick={onclose}><IconX aria-hidden="true" /></Button
        >
      </div>
    </div>
    <div
      role="status"
      aria-live="polite"
      aria-label={m.s4_find_results_aria()}
      class="mt-1 min-h-lh text-xs"
      class:text-destructive={!!error}
    >
      {error || results}
    </div>
    <p class="mt-1 text-xs text-muted-foreground">
      {terminalFindScope(t.scrollback, t.bufferType, t.historyTruncated)}
    </p>
    <div class="mt-1 grid text-xs text-muted-foreground">
      {#each replayMessages as message (message)}
        <span class="invisible col-start-1 row-start-1" aria-hidden="true"
          >{message}</span
        >
      {/each}
      <p class="col-start-1 row-start-1" role="status">{query ? replay : ""}</p>
    </div>
  </form>
{/if}
