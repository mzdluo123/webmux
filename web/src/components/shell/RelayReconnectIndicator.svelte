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
</script>

<!-- Touch layouts keep soft keys and the chat composer at the bottom edge;
     the pill lifts above them so it never swallows their taps. -->
<div
  class={cn(
    "fixed bottom-2 z-50 max-sm:bottom-16 any-pointer-coarse:bottom-16",
    className ?? "left-2",
  )}
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
