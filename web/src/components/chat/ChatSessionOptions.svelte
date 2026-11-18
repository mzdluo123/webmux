<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import IconLoader2 from "~icons/tabler/loader-2";
  import IconSettings from "~icons/tabler/settings";
  import IconX from "~icons/tabler/x";
  import { Button } from "$lib/components/ui/button";
  import * as Drawer from "$lib/components/ui/drawer";
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import type { ConfigValue } from "../../platform/chat-api";
  import {
    changedPermissionMode,
    isModelOption,
    permissionChoices,
  } from "../../domains/chat/chat-permission";
  import type { ChatConfig, ChatPermissionMode } from "../../contracts/schemas";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import ChatInlineOptionPicker from "./ChatInlineOptionPicker.svelte";

  // Low-frequency options share one entry inside the mobile composer. The
  // temporary sheet shows names and current values without adding a bottom row.
  let {
    mode,
    config,
    disabled,
    modeDisabled,
    loading = false,
    error = "",
    onmodechange,
    onconfigchange,
  }: {
    mode: ChatPermissionMode;
    config: ChatConfig[];
    disabled: boolean;
    modeDisabled: boolean;
    loading?: boolean;
    error?: string;
    onmodechange: (mode: ChatPermissionMode) => void;
    onconfigchange: (configId: string, value: ConfigValue) => void;
  } = $props();
  let trigger = $state<HTMLButtonElement | null>(null);
  let origin = $state<DOMRectReadOnly | null>(null);
  let openPickerId = $state<string | null>(null);
  const pickerIdPrefix = $props.id();

  function setPickerOpen(optionId: string, next: boolean) {
    if (next) {
      openPickerId = optionId;
    } else if (openPickerId === optionId) {
      openPickerId = null;
    }
  }

  function closePicker(id: string) {
    setPickerOpen(id, false);
    document
      .getElementById(`${pickerIdPrefix}-${id}`)
      ?.focus({ preventScroll: true });
  }

  type RowChoice = {
    value: string;
    name: string;
    description?: string;
    reason?: string;
  };
</script>

<Drawer.Root
  shouldScaleBackground={false}
  autoFocus
  onOpenChange={(open) => {
    if (open) origin = trigger?.getBoundingClientRect() ?? null;
    else openPickerId = null;
  }}
>
  <Drawer.Trigger bind:ref={trigger}>
    {#snippet child({ props })}
      <Button
        {...props}
        variant="ghost"
        size="icon"
        aria-label={m.s3_session_options()}
        title={m.s3_session_options()}
        ><IconSettings aria-hidden="true" /></Button
      >
    {/snippet}
  </Drawer.Trigger>
  <Drawer.Content
    motion="origin"
    {origin}
    class="data-[vaul-drawer-direction=bottom]:max-h-[calc(100dvh-1rem)]"
    onEscapeKeydown={(event) => {
      if (openPickerId === null) return;
      event.preventDefault();
      closePicker(openPickerId);
    }}
    onCloseAutoFocus={(event) => {
      // Safari does not focus tapped buttons. Returning to the previously
      // focused textarea would reopen the keyboard after dismissing the sheet.
      event.preventDefault();
      trigger?.focus({ preventScroll: true });
    }}
  >
    <div class="flex shrink-0 items-center justify-between gap-2 px-4 py-2">
      <Drawer.Title>{m.s3_session_options()}</Drawer.Title>
      <Drawer.Close>
        {#snippet child({ props })}
          <Button
            {...props}
            variant="ghost"
            size="icon"
            aria-label={m.s3_close_options()}
            title={m.s3_close_options()}
          >
            <IconX aria-hidden="true" />
          </Button>
        {/snippet}
      </Drawer.Close>
    </div>
    <Drawer.Description class="sr-only"
      >{m.s3_session_options_desc()}</Drawer.Description
    >
    {#if error}
      <div class="max-h-[30dvh] shrink-0 overflow-y-auto px-4 pb-3">
        <InlineNotice tone="danger">{error}</InlineNotice>
      </div>
    {/if}
    <div
      class="flex min-h-0 flex-col gap-3 overflow-y-auto overscroll-contain px-4 pb-safe-4"
    >
      {#if loading}
        <p
          class="flex items-center gap-1.5 text-xs text-muted-foreground"
          role="status"
        >
          <IconLoader2
            class="size-3.5 animate-spin motion-reduce:animate-none"
            aria-hidden="true"
          />{m.s3_loading_config()}
        </p>
      {/if}
      {@render selectRow({
        id: "webmux-permission",
        name: m.s3_permission_name(),
        value: mode,
        choices: permissionChoices,
        disabled: modeDisabled,
        searchable: false,
        onselect: (value) => {
          const changed = changedPermissionMode(value, mode);
          if (changed) onmodechange(changed);
        },
      })}
      {#each config as option (option.id)}
        {#if option.type === "boolean"}
          <Label class="justify-between">
            <span class="min-w-0 wrap-anywhere">{option.name}</span>
            <Switch
              aria-label={option.name}
              bind:checked={
                () => !!option.enabled,
                (enabled) => onconfigchange(option.id, { enabled })
              }
              {disabled}
            />
          </Label>
        {:else}
          {@render selectRow({
            id: option.id,
            name: option.name,
            value: option.value ?? "",
            choices: option.options,
            disabled,
            searchable: isModelOption(option),
            onselect: (value) => {
              onconfigchange(option.id, { value });
            },
          })}
        {/if}
      {/each}
    </div>
  </Drawer.Content>
</Drawer.Root>

{#snippet selectRow({
  id,
  name,
  value,
  choices,
  disabled,
  searchable,
  onselect,
}: {
  id: string;
  name: string;
  value: string;
  choices: readonly RowChoice[];
  disabled: boolean;
  searchable: boolean;
  onselect: (value: string) => void;
})}
  <ChatInlineOptionPicker
    triggerId={`${pickerIdPrefix}-${id}`}
    {name}
    {value}
    {choices}
    {disabled}
    {searchable}
    open={openPickerId === id}
    onOpenChange={(next) => setPickerOpen(id, next)}
    onselect={(next) => {
      onselect(next);
      closePicker(id);
    }}
  />
{/snippet}
