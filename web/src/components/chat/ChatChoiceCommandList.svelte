<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import type { Snippet } from "svelte";
  import * as Command from "$lib/components/ui/command";

  // The searchable option list shared by desktop model popovers and the
  // mobile session-options sheet. Rows match by name, value or description;
  // a choice with a `reason` stays listed, disabled, with the reason under it.
  type Choice = {
    value: string;
    name: string;
    description?: string;
    reason?: string;
  };

  let {
    name,
    value,
    choices,
    disabled = false,
    searchable = true,
    search = $bindable(""),
    choiceIcon,
    onselect,
  }: {
    name: string;
    value: string;
    choices: readonly Choice[];
    disabled?: boolean;
    /** Long catalogs get a filter box; short lists show all rows. */
    searchable?: boolean;
    search?: string;
    choiceIcon?: Snippet<[string]>;
    onselect: (value: string) => void;
  } = $props();
</script>

<Command.Root
  label={name}
  class="h-auto min-w-0"
  filter={(itemValue, query, keywords) =>
    [itemValue, ...(keywords ?? [])].some((text) =>
      text.toLowerCase().includes(query.trim().toLowerCase()),
    )
      ? 1
      : 0}
  disableInitialScroll
>
  {#if searchable}
    <Command.Input
      bind:value={search}
      placeholder={m.s3_search_option_placeholder({ name })}
      aria-label={m.s3_search_option_label({ name })}
      autocomplete="off"
      autocapitalize="off"
      autocorrect="off"
      spellcheck={false}
    />
  {/if}
  <Command.List
    aria-label={m.s3_option_list_label({ name })}
    class="max-h-[min(14rem,var(--bits-popover-content-available-height,50dvh))] p-1"
  >
    <Command.Empty>{m.s3_no_matching_option({ name })}</Command.Empty>
    {#each choices as choice (choice.value)}
      {@const note = choice.reason ?? choice.description}
      <Command.Item
        value={choice.value}
        keywords={[choice.name, choice.description ?? ""]}
        disabled={disabled || !!choice.reason}
        data-checked={choice.value === value}
        onSelect={() => onselect(choice.value)}
      >
        {#if choiceIcon}{@render choiceIcon(choice.value)}{/if}
        <span class="min-w-0 flex-1 wrap-anywhere">
          {choice.name}
          {#if note}<span class="block text-xs text-muted-foreground"
              >{note}</span
            >{/if}
        </span>
      </Command.Item>
    {/each}
  </Command.List>
</Command.Root>
