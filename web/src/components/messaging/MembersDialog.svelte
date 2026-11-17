<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import IconRobot from "~icons/tabler/robot";
  import IconPlus from "~icons/tabler/plus";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import type { MessagingController } from "../../hooks/messaging/messaging.svelte";
  let { messaging }: { messaging: MessagingController } = $props();
  const view = $derived(messaging.state);
  const available = $derived(
    view.agents.filter(
      (agent) =>
        !view.members.some(
          (member) => member.id === "session:" + agent.sessionId,
        ),
    ),
  );
</script>

<ResponsiveDialog
  title={m.messaging_members_title({ name: view.channel })}
  description={m.messaging_members_desc()}
  busy={!!view.memberBusy}
  onclose={() => (view.membersOpen = false)}
>
  <div class="space-y-5">
    {#if view.memberError}<InlineNotice tone="danger"
        >{view.memberError}</InlineNotice
      >
      <Button
        variant="outline"
        disabled={!!view.memberBusy}
        onclick={() => void messaging.openMembers()}
        >{m.messaging_refresh()}</Button
      >{/if}
    {#if view.membersLoading}<p
        class="text-sm text-muted-foreground"
        role="status"
      >
        {m.messaging_loading()}
      </p>
    {:else}
      {#if view.members.length}
        <section class="space-y-2" aria-label={m.messaging_subscribed()}>
          <h3 class="text-sm font-medium">{m.messaging_subscribed()}</h3>
          {#each view.members as member (member.id)}
            <div class="flex min-w-0 items-center gap-3 rounded-lg border p-3">
              <IconRobot
                class="size-5 shrink-0 text-primary"
                aria-hidden="true"
              />
              <div class="min-w-0 flex-1">
                <p class="truncate text-sm font-medium">{member.name}</p>
                <p
                  class="truncate text-xs text-muted-foreground"
                  title={member.id}
                >
                  {view.agents.find(
                    (agent) => "session:" + agent.sessionId === member.id,
                  )?.name ?? member.id}
                </p>
              </div>
              <Button
                variant="ghost"
                size="sm-touch"
                disabled={!!view.memberBusy}
                aria-label={m.messaging_remove_agent({ name: member.name })}
                onclick={() =>
                  void messaging.changeMember(
                    member.id.replace(/^session:/, ""),
                    true,
                  )}>{m.messaging_remove()}</Button
              >
            </div>
          {/each}
        </section>
      {/if}
      <section class="space-y-2" aria-label={m.messaging_available_agents()}>
        <h3 class="text-sm font-medium">{m.messaging_available_agents()}</h3>
        {#each available as agent (agent.sessionId)}
          <ListButton
            variant="navigation"
            class="w-full"
            disabled={!!view.memberBusy}
            aria-label={m.messaging_invite_agent({ name: agent.name })}
            onclick={() => void messaging.changeMember(agent.sessionId)}
          >
            <IconRobot
              class="size-5 shrink-0 text-primary"
              aria-hidden="true"
            />
            <span class="flex min-w-0 flex-1 flex-col gap-1"
              ><span class="truncate">{agent.name}</span><span
                class="truncate text-xs font-normal text-muted-foreground"
                >{agent.profile} · {agent.sessionId.slice(0, 8)}</span
              ></span
            >
            <IconPlus class="size-4 shrink-0" aria-hidden="true" />
          </ListButton>
        {:else}<p class="text-sm text-muted-foreground">
            {m.messaging_no_available_agents()}
          </p>{/each}
      </section>
    {/if}
  </div>
</ResponsiveDialog>
