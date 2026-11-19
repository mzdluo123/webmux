<script lang="ts">
  import { tick } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import { Button } from "$lib/components/ui/button";
  import * as Drawer from "$lib/components/ui/drawer";
  import * as Popover from "$lib/components/ui/popover";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { m } from "$lib/paraglide/messages.js";
  import type { SessionInfo } from "../../contracts/schemas";
  import RenameSessionDialog from "../sessions/RenameSessionDialog.svelte";

  let {
    title,
    session,
    agent,
  }: { title: string; session?: SessionInfo; agent?: string } = $props();
  const phone = new MediaQuery("(max-width: 639px)");
  let open = $state(false);
  let trigger = $state<HTMLButtonElement | null>(null);
  let renaming = $state.raw<SessionInfo>();

  function rename() {
    if (!session) return;
    renaming = session;
    open = false;
  }

  function closeRename() {
    renaming = undefined;
    void tick().then(() => trigger?.focus());
  }

  function restoreFocus(event: Event) {
    if (renaming) event.preventDefault();
  }
</script>

{#snippet titleButton(props: Record<string, unknown>)}
  <Button
    {...props}
    bind:ref={trigger}
    variant="ghost"
    size="sm"
    class="min-w-0 max-w-full"
    {title}
  >
    <IconChevronDown data-icon="inline-start" aria-hidden="true" />
    <span class="min-w-0 max-w-48 truncate">{title}</span>
  </Button>
{/snippet}

{#snippet renameAction()}
  <Button
    variant="outline"
    size="sm-touch"
    disabled={!session}
    onclick={rename}
  >
    {m.s3_rename_title()}
  </Button>
{/snippet}

{#if phone.current}
  <Drawer.Root bind:open>
    <Drawer.Trigger>
      {#snippet child({ props })}{@render titleButton(props)}{/snippet}
    </Drawer.Trigger>
    <Drawer.Content onCloseAutoFocus={restoreFocus}>
      <ScrollArea class="min-h-0">
        <Drawer.Header>
          <Drawer.Title>{m.s4_session_title()}</Drawer.Title>
          <Drawer.Description>{title}</Drawer.Description>
          {#if agent}<p>{m.s4_agent_label({ title: agent })}</p>{/if}
        </Drawer.Header>
        <Drawer.Footer>{@render renameAction()}</Drawer.Footer>
      </ScrollArea>
    </Drawer.Content>
  </Drawer.Root>
{:else}
  <Popover.Root bind:open>
    <Popover.Trigger>
      {#snippet child({ props })}{@render titleButton(props)}{/snippet}
    </Popover.Trigger>
    <Popover.Content
      align="start"
      class="max-h-(--bits-popover-content-available-height)"
      role="dialog"
      aria-label={m.s4_session_title()}
      onCloseAutoFocus={restoreFocus}
    >
      <ScrollArea class="min-h-0">
        <p class="wrap-anywhere">{title}</p>
        {#if agent}
          <p class="wrap-anywhere text-muted-foreground md:hidden">
            {m.s4_agent_label({ title: agent })}
          </p>
        {/if}
      </ScrollArea>
      <div class="shrink-0">{@render renameAction()}</div>
    </Popover.Content>
  </Popover.Root>
{/if}

{#if renaming}
  <RenameSessionDialog session={renaming} onclose={closeRename} />
{/if}
