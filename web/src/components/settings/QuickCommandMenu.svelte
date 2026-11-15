<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import IconBolt from "~icons/tabler/bolt";
  import IconCornerDownLeft from "~icons/tabler/corner-down-left";
  import IconPlus from "~icons/tabler/plus";
  import IconSettings from "~icons/tabler/settings";
  import {
    Button,
    type ButtonSize,
    type ButtonVariant,
  } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import type { QuickCommand } from "../../contracts/schemas";
  import { commandPreview } from "../../domains/settings/quick-commands";

  let {
    commands,
    target,
    unavailable,
    disabled,
    side,
    size = "icon-sm",
    variant = "outline",
    run,
    manage,
    onCloseAutoFocus,
    submenu = false,
  }: {
    commands: QuickCommand[];
    // "chat" inserts into a hybrid session's message box, never sending.
    target: "terminal" | "draft" | "chat";
    unavailable: string;
    disabled: boolean;
    side: "top" | "bottom";
    // Match the neighbouring buttons of the toolbar that hosts the trigger.
    size?: ButtonSize;
    variant?: ButtonVariant;
    run: (cmd: QuickCommand) => void;
    manage: () => void;
    onCloseAutoFocus: (event: Event) => void;
    submenu?: boolean;
  } = $props();
  const blocked = $derived(target === "terminal" && !!unavailable);
  const title = $derived(
    target === "terminal"
      ? m.s2_qc_menu_send_terminal()
      : target === "draft"
        ? m.s2_qc_menu_insert_draft()
        : m.s2_qc_menu_insert_chat(),
  );
</script>

{#snippet items()}
  <DropdownMenu.Label variant="title">{title}</DropdownMenu.Label>
  {#if blocked}
    <p class="px-2 pb-2 text-xs text-muted-foreground" role="status">
      {unavailable}
    </p>
  {/if}
  {#each commands as cmd (cmd.id)}
    {@const preview = commandPreview(cmd)}
    {@const enter = target === "terminal" && cmd.enter}
    <DropdownMenu.Item
      class="min-h-9 any-pointer-coarse:min-h-11"
      disabled={blocked}
      aria-label={[cmd.name, preview].filter(Boolean).join(m.s2_qc_item_sep()) +
        (enter ? m.s2_qc_item_enter_suffix() : "")}
      onSelect={() => run(cmd)}
    >
      <span class="flex min-w-0 flex-1 flex-col gap-0.5">
        {#if cmd.name}<span class="truncate">{cmd.name}</span>{/if}
        <span
          class={["truncate font-mono", cmd.name && "text-muted-foreground"]}
          >{preview}</span
        >
      </span>
      {#if enter}<DropdownMenu.Shortcut
          ><IconCornerDownLeft aria-hidden="true" /></DropdownMenu.Shortcut
        >{/if}
    </DropdownMenu.Item>
  {:else}
    <div class="px-2 py-2 text-xs">
      <p class="font-medium">{m.s2_qc_empty_title()}</p>
      <p class="mt-1 text-muted-foreground">
        {m.s2_qc_menu_empty_desc()}
      </p>
    </div>
    <DropdownMenu.Item class="any-pointer-coarse:min-h-11" onSelect={manage}
      ><IconPlus aria-hidden="true" />{m.s2_qc_menu_add()}</DropdownMenu.Item
    >
  {/each}
  <DropdownMenu.Separator />
  <DropdownMenu.Item class="any-pointer-coarse:min-h-11" onSelect={manage}
    ><IconSettings
      aria-hidden="true"
    />{m.s2_qc_menu_manage()}</DropdownMenu.Item
  >
{/snippet}

{#if submenu}
  <DropdownMenu.Sub>
    <DropdownMenu.SubTrigger
      aria-label={m.s2_qc_title()}
      title={m.s2_qc_title()}
      {disabled}
    >
      <IconBolt aria-hidden="true" />{m.s2_qc_title()}
    </DropdownMenu.SubTrigger>
    <DropdownMenu.SubContent
      class="max-h-[min(60dvh,24rem)] w-72 max-w-[calc(100vw-1rem)]"
    >
      {@render items()}
    </DropdownMenu.SubContent>
  </DropdownMenu.Sub>
{:else}
  <DropdownMenu.Root>
    <DropdownMenu.Trigger {disabled}>
      {#snippet child({ props })}<Button
          {...props}
          {variant}
          {size}
          aria-label={m.s2_qc_title()}
          title={m.s2_qc_title()}
          ><IconBolt class="size-4" aria-hidden="true" /></Button
        >{/snippet}
    </DropdownMenu.Trigger>
    <DropdownMenu.Content
      {side}
      align="end"
      class="max-h-[min(60dvh,24rem)] w-72 max-w-[calc(100vw-1rem)]"
      {onCloseAutoFocus}
    >
      {@render items()}
    </DropdownMenu.Content>
  </DropdownMenu.Root>
{/if}
