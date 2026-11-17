<script lang="ts">
  import * as Popover from "$lib/components/ui/popover";
  import { Button } from "$lib/components/ui/button";
  import { cn } from "$lib/utils";
  import { m } from "$lib/paraglide/messages.js";
  import RelayReconnectDetails from "./RelayReconnectDetails.svelte";

  let {
    checking = false,
    returnTo,
    onretry,
    class: className,
  }: {
    checking?: boolean;
    returnTo: string;
    onretry: () => void;
    class?: string;
  } = $props();

  const statusLabel = $derived(
    checking ? m.s1_disconnected_reconnecting() : m.relay_reconnect_paused(),
  );

  // Views that keep chrome at the bottom edge (chat composer dock, terminal
  // soft keys, draft pane) mark its top boundary with data-bottom-dock. The
  // draft pane is user-resizable, so a fixed lift cannot clear it; the pill
  // measures the real stack and sits above it instead of swallowing taps.
  let lift = $state(0);

  $effect(() => {
    const measure = () => {
      let top = Number.POSITIVE_INFINITY;
      for (const dock of document.querySelectorAll("[data-bottom-dock]")) {
        const rect = dock.getBoundingClientRect();
        if (rect.height > 0 && rect.bottom > 0) top = Math.min(top, rect.top);
      }
      lift = Number.isFinite(top) ? Math.max(0, window.innerHeight - top) : 0;
    };
    measure();
    const timer = setInterval(measure, 400);
    window.addEventListener("resize", measure);
    return () => {
      clearInterval(timer);
      window.removeEventListener("resize", measure);
    };
  });
</script>

<div
  class={cn(
    "fixed bottom-[calc(var(--reconnect-lift,0px)+0.5rem)] z-50",
    className ?? "left-2",
  )}
  style:--reconnect-lift={`${lift}px`}
>
  <Popover.Root>
    <Popover.Trigger>
      {#snippet child({ props })}
        <Button
          {...props}
          variant="outline"
          size="sm"
          raised
          aria-label={m.relay_reconnect_status_aria({ status: statusLabel })}
          title={statusLabel}
        >
          <span
            aria-hidden="true"
            class="status-dot status-waiting shrink-0"
            class:motion-safe:animate-pulse={checking}
          ></span>
          <span class="truncate text-warning">{statusLabel}</span>
        </Button>
      {/snippet}
    </Popover.Trigger>
    <Popover.Content
      side="top"
      align="start"
      role="dialog"
      aria-label={m.s1_host_status()}
      class="w-[min(20rem,calc(100vw-2rem))] gap-3"
    >
      <RelayReconnectDetails {checking} {returnTo} {onretry} />
    </Popover.Content>
  </Popover.Root>
</div>
