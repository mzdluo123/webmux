<script lang="ts">
  import * as Popover from "$lib/components/ui/popover";
  import { Button } from "$lib/components/ui/button";
  import { cn } from "$lib/utils";
  import { m } from "$lib/paraglide/messages.js";
  import { relayLoginHref } from "../../hooks/shared/relay-login";

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
  const detail = $derived(
    checking ? m.relay_auth_reconnecting() : m.relay_auth_retry_stopped(),
  );
</script>

<div class={cn("fixed bottom-2 z-50", className ?? "left-2")}>
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
      <p role="status" class="leading-relaxed">{detail}</p>
      <p class="text-xs text-muted-foreground">
        {m.relay_auth_drafts_kept()}
      </p>
      <div class="flex flex-wrap justify-end gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={checking}
          onclick={onretry}>{m.relay_auth_check_again()}</Button
        >
        <Button size="sm" href={relayLoginHref(returnTo)}
          >{m.relay_auth_login()}</Button
        >
      </div>
    </Popover.Content>
  </Popover.Root>
</div>
