<script lang="ts">
  import IconSelector from "~icons/tabler/selector";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import { Label } from "$lib/components/ui/label";
  import { useDockMotion } from "../../hooks/shared/dock-motion.svelte";
  import ChatChoiceCommandList from "./ChatChoiceCommandList.svelte";

  type Choice = {
    value: string;
    name: string;
    description?: string;
    reason?: string;
  };

  let {
    triggerId,
    name,
    value,
    choices,
    disabled,
    searchable,
    open,
    search = $bindable(),
    onOpenChange,
    onselect,
  }: {
    triggerId: string;
    name: string;
    value: string;
    choices: readonly Choice[];
    disabled: boolean;
    searchable: boolean;
    open: boolean;
    search: string;
    onOpenChange: (open: boolean) => void;
    onselect: (value: string) => void;
  } = $props();
  const currentName = $derived(
    choices.find((choice) => choice.value === value)?.name ?? value,
  );
  let panel = $state<HTMLDivElement | null>(null);
  const settle = useDockMotion(
    () => panel,
    () => true,
    () => String(open),
  );
</script>

<Collapsible.Root {open} {onOpenChange}>
  <div class="grid grid-cols-[minmax(0,1fr)_minmax(0,2fr)] items-center gap-2">
    <Label for={triggerId} class="min-w-0 wrap-anywhere">{name}</Label>
    <Collapsible.Trigger
      id={triggerId}
      aria-label={name}
      title={currentName}
      {disabled}
      class="flex h-(--control-default) w-full min-w-0 items-center justify-between gap-2 rounded-md border border-input bg-card px-3 py-1 text-left text-base outline-none select-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 aria-expanded:border-ring md:text-sm dark:bg-input/30 dark:hover:bg-input/50"
    >
      <span class="min-w-0 flex-1 truncate">{currentName}</span>
      <IconSelector
        class="size-4 shrink-0 text-muted-foreground"
        aria-hidden="true"
      />
    </Collapsible.Trigger>
  </div>
  <div bind:this={panel} inert={!open} aria-hidden={!open} oninput={settle}>
    <Collapsible.Content>
      {#if open}
        <div class="pt-2">
          <div class="overflow-hidden rounded-lg border" data-vaul-no-drag>
            <ChatChoiceCommandList
              {name}
              {value}
              {choices}
              {disabled}
              {searchable}
              inline
              bind:search
              {onselect}
            />
          </div>
        </div>
      {/if}
    </Collapsible.Content>
  </div>
</Collapsible.Root>
