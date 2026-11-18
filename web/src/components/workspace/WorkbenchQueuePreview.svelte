<script lang="ts">
  import IconX from "~icons/tabler/x";
  import { Button } from "$lib/components/ui/button";
  import * as Drawer from "$lib/components/ui/drawer";
  import type { SessionInfo } from "../../contracts/schemas";
  import { m } from "$lib/paraglide/messages.js";
  import { previewFor } from "../../domains/settings/launch-previews.svelte";
  import { selectPreview } from "../../domains/sessions/preview";
  import { openSession } from "../../hooks/shared/shell.svelte";

  let {
    session,
    location,
    docked,
    open,
    opener,
    onclose,
    onrestored,
  }: {
    session?: SessionInfo;
    location: string;
    docked: boolean;
    open: boolean;
    opener: HTMLElement | null;
    onclose: () => void;
    onrestored: () => void;
  } = $props();

  let closeButton = $state<HTMLButtonElement | null>(null);
  $effect(() => {
    if (open) closeButton?.focus();
  });

  function finish() {
    const target = opener?.isConnected
      ? opener
      : session &&
        document.querySelector<HTMLElement>(
          `[data-session-id="${CSS.escape(session.id)}"] [data-queue-preview]`,
        );
    target?.focus();
    onrestored();
    return Boolean(target);
  }
  function closeOnEscape(event: KeyboardEvent) {
    if (docked && event.key === "Escape") {
      event.preventDefault();
      onclose();
      finish();
    }
  }

  const text = $derived(
    session
      ? session.view === "chat" && !session.transition
        ? session.preview
        : selectPreview(session.preview, previewFor(session.configId))
      : "",
  );
</script>

{#snippet output(active: SessionInfo)}
  <div class="space-y-3">
    <p class="min-w-0 text-xs wrap-anywhere text-muted-foreground">
      {location}
    </p>
    {#if active.view === "chat" && !active.transition}
      <p
        class="max-h-72 overflow-auto text-sm wrap-anywhere whitespace-pre-wrap"
      >
        {text || m.s8_queue_preview_description()}
      </p>
    {:else}
      <pre
        data-slot="terminal-preview"
        aria-label={m.s3_terminal_output_preview_label()}
        class="max-h-72 overflow-auto rounded-md border bg-background p-3 font-mono text-xs text-foreground">{text ||
          (active.preview
            ? m.s3_preview_trailing_hidden()
            : m.s3_waiting_terminal_output())}</pre>
    {/if}
  </div>
{/snippet}

{#snippet enter(active: SessionInfo)}
  <Button
    variant="outline"
    onkeydown={closeOnEscape}
    onclick={() => openSession(active)}
  >
    {m.s8_queue_open()}
  </Button>
{/snippet}

{#if docked}
  {#if open && session}
    <aside
      class="sticky top-4 self-start rounded-lg border bg-card p-4"
      aria-label={m.s8_queue_preview({ title: session.title })}
    >
      <div class="mb-3 flex items-start justify-between gap-2">
        <h2 class="min-w-0 text-base font-semibold wrap-anywhere">
          {session.title}
        </h2>
        <Button
          variant="ghost"
          size="icon-sm"
          bind:ref={closeButton}
          onkeydown={closeOnEscape}
          aria-label={m.s8_queue_close_preview()}
          title={m.s8_queue_close_preview()}
          onclick={() => {
            onclose();
            finish();
          }}><IconX aria-hidden="true" /></Button
        >
      </div>
      {@render output(session)}
      <div class="mt-3">{@render enter(session)}</div>
    </aside>
  {/if}
{:else}
  <Drawer.Root
    {open}
    onOpenChange={(value) => {
      if (!value) onclose();
    }}
  >
    <Drawer.Content
      onCloseAutoFocus={(event) => {
        if (finish()) event.preventDefault();
      }}
    >
      {#if session}
        <div class="relative shrink-0 px-12">
          <Drawer.Header>
            <Drawer.Title>
              <span class="block truncate" title={session.title}
                >{session.title}</span
              >
            </Drawer.Title>
            <Drawer.Description class="sr-only"
              >{m.s8_queue_preview_description()}</Drawer.Description
            >
          </Drawer.Header>
          <Drawer.Close>
            {#snippet child({ props })}<Button
                {...props}
                bind:ref={closeButton}
                variant="ghost"
                size="icon-touch"
                class="absolute top-2 right-2"
                aria-label={m.s8_queue_close_preview()}
                title={m.s8_queue_close_preview()}
                ><IconX aria-hidden="true" /></Button
              >{/snippet}
          </Drawer.Close>
        </div>
        <div class="min-h-0 overflow-y-auto px-4 pt-1">
          {@render output(session)}
        </div>
        <div class="shrink-0 px-4 pt-3 pb-safe-4">
          {@render enter(session)}
        </div>
      {/if}
    </Drawer.Content>
  </Drawer.Root>
{/if}
