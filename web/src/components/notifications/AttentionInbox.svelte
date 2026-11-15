<script lang="ts">
  import IconBell from "~icons/tabler/bell";
  import * as m from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Badge } from "$lib/components/ui/badge";
  import * as Popover from "$lib/components/ui/popover";
  import { buildAttentionToast } from "../../domains/notifications/attention-toast";
  import {
    attentionDeferred,
    deferAttention,
    wakeAttention,
  } from "../../domains/notifications/attention-toast.svelte";
  import { hasUrgentAttention } from "../../domains/sessions/session-status";
  import type { SessionInfo } from "../../contracts/schemas";
  import { openSession } from "../../hooks/shared/shell.svelte";

  let { sessions }: { sessions: readonly SessionInfo[] } = $props();
  let open = $state(false);
  const alerts = $derived(
    sessions
      .flatMap((session) => {
        const attention = session.attention;
        if (
          !attention ||
          session.status === "exited" ||
          !hasUrgentAttention(session)
        )
          return [];
        return [
          {
            session,
            alert: buildAttentionToast(session, attention),
            deferred: attentionDeferred(session),
          },
        ];
      })
      .sort(
        (a, b) =>
          Number(a.deferred) - Number(b.deferred) ||
          Date.parse(b.alert.since) - Date.parse(a.alert.since),
      ),
  );
  const pending = $derived(alerts.filter((item) => !item.deferred).length);
</script>

{#if alerts.length || open}
  <div class="px-2 pt-1">
    <Popover.Root bind:open>
      <Popover.Trigger>
        {#snippet child({ props })}
          <Button
            {...props}
            variant="ghost"
            size="sm-touch"
            class="w-full justify-start"
          >
            <IconBell aria-hidden="true" />{m.s2_inbox_title()}
            {#if pending}<Badge variant="warning" class="ml-auto"
                >{pending}</Badge
              >{/if}
          </Button>
        {/snippet}
      </Popover.Trigger>
      <Popover.Content
        side="bottom"
        align="start"
        role="dialog"
        aria-label={m.s2_inbox_title()}
        class="flex max-h-(--bits-popover-content-available-height) w-[min(21rem,calc(100vw-1.5rem))] flex-col overflow-hidden"
      >
        <Popover.Header>
          <Popover.Title>{m.s2_inbox_title()}</Popover.Title>
          <Popover.Description>{m.s2_inbox_desc()}</Popover.Description>
        </Popover.Header>
        {#if alerts.length}
          <ul
            class="min-h-0 max-h-[min(22rem,45dvh)] space-y-2 overflow-y-auto px-0.5 py-0.5"
          >
            {#each alerts as { session, alert, deferred } (session.id)}
              <li class="flex min-w-0 flex-wrap items-center gap-1">
                <Button
                  variant="ghost"
                  size="sm"
                  class="min-w-0 flex-1 justify-start"
                  title={alert.title}
                  onclick={() => {
                    open = false;
                    openSession(session);
                  }}
                >
                  <span class="truncate">{alert.title}</span>
                </Button>
                <Badge variant="secondary"
                  >{deferred ? m.s2_inbox_deferred() : alert.label}</Badge
                >
                <Button
                  variant="outline"
                  size="sm"
                  onclick={() =>
                    deferred ? wakeAttention(session) : deferAttention(session)}
                  aria-label={deferred
                    ? m.s2_inbox_wake_aria({ title: alert.title })
                    : m.s2_inbox_defer_aria({ title: alert.title })}
                >
                  {deferred ? m.s2_inbox_wake() : m.s2_inbox_defer()}
                </Button>
              </li>
            {/each}
          </ul>
        {:else}
          <p class="text-sm text-muted-foreground">{m.s2_inbox_empty()}</p>
        {/if}
      </Popover.Content>
    </Popover.Root>
  </div>
{/if}
