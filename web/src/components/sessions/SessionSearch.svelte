<script lang="ts">
  import IconSearch from "~icons/tabler/search";
  import * as InputGroup from "$lib/components/ui/input-group";
  import { m } from "$lib/paraglide/messages.js";

  let {
    query = $bindable(),
    input = $bindable(null),
    placeholder,
    shortcutDisabled = false,
    onclear,
  }: {
    query: string;
    input?: HTMLInputElement | null;
    placeholder: string;
    shortcutDisabled?: boolean;
    onclear: () => void;
  } = $props();

  function searchShortcut(event: KeyboardEvent) {
    if (
      event.key !== "/" ||
      event.defaultPrevented ||
      event.isComposing ||
      event.metaKey ||
      event.ctrlKey ||
      event.altKey ||
      shortcutDisabled
    )
      return;
    const target = event.target;
    if (
      target instanceof HTMLElement &&
      (target.isContentEditable ||
        target.closest('input, textarea, select, [role="dialog"]'))
    )
      return;
    event.preventDefault();
    input?.focus();
  }
</script>

<svelte:window onkeydown={searchShortcut} />

<InputGroup.Root>
  <InputGroup.Addon><IconSearch aria-hidden="true" /></InputGroup.Addon>
  <InputGroup.Input
    type="search"
    aria-label={m.s3_search_sessions_label()}
    aria-keyshortcuts="/"
    title={m.s3_search_sessions_shortcut_title()}
    {placeholder}
    bind:ref={input}
    bind:value={query}
    onkeydown={(event) => {
      if (event.key === "Escape" && !event.isComposing) {
        event.preventDefault();
        onclear();
      }
    }}
  />
  <InputGroup.Addon align="inline-end" class="hidden md:flex">
    <kbd
      class="rounded-sm border bg-muted/70 px-1.5 font-mono text-xs text-muted-foreground"
      aria-hidden="true">/</kbd
    >
  </InputGroup.Addon>
</InputGroup.Root>
