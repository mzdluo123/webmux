<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import type { SearchSpec } from "../../domains/code/code-search";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import IconArrowDown from "~icons/tabler/arrow-down";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconSelectAll from "~icons/tabler/select-all";
  import IconX from "~icons/tabler/x";

  let {
    query,
    onquery,
    onnext,
    onprevious,
    onall,
    onclose,
  }: {
    query: SearchSpec;
    onquery: (query: SearchSpec) => void;
    onnext: () => void;
    onprevious: () => void;
    onall?: () => void;
    onclose: () => void;
  } = $props();
  // The reader owns the query; the panel mirrors it and reports edits back.
  // svelte-ignore state_referenced_locally
  let spec = $state({ ...query });
  export function setQuery(next: SearchSpec) {
    spec = { ...next };
  }
  function commit() {
    onquery({ ...spec });
  }
  function toggle(key: "caseSensitive" | "regexp" | "wholeWord") {
    spec[key] = !spec[key];
    commit();
  }
  const options = $derived([
    { key: "caseSensitive", label: m.s5_match_case(), text: "Aa" },
    { key: "wholeWord", label: m.s5_match_whole_word(), text: "W" },
    { key: "regexp", label: m.s5_use_regex(), text: ".*" },
  ] as const);
</script>

<div data-code-search class="flex flex-wrap items-center gap-1 p-1">
  <Input
    bind:value={spec.search}
    {...{ "main-field": "true" }}
    aria-label={m.s5_search_find()}
    placeholder={m.s5_search_find()}
    class="h-7 min-w-32 flex-1"
    oninput={commit}
    onkeydown={(e) => {
      if (e.key !== "Enter") return;
      e.preventDefault();
      if (e.shiftKey) onprevious();
      else onnext();
    }}
  />
  <div class="flex items-center gap-1">
    {#each options as option (option.key)}
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={option.label}
        title={option.label}
        aria-pressed={spec[option.key]}
        onclick={() => toggle(option.key)}
        ><span class="font-mono text-xs">{option.text}</span></Button
      >
    {/each}
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={m.s5_previous_match()}
      title={m.s5_previous_match_shortcut()}
      onclick={onprevious}><IconArrowUp aria-hidden="true" /></Button
    >
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={m.s5_next_match()}
      title={m.s5_next_match_shortcut()}
      onclick={onnext}><IconArrowDown aria-hidden="true" /></Button
    >
    {#if onall}<Button
        variant="ghost"
        size="icon-sm"
        aria-label={m.s5_select_all_matches()}
        title={m.s5_select_all_matches()}
        onclick={onall}><IconSelectAll aria-hidden="true" /></Button
      >{/if}
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={m.s5_close_search()}
      title={m.s5_close_search_esc()}
      onclick={onclose}><IconX aria-hidden="true" /></Button
    >
  </div>
</div>
