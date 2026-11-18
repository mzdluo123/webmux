<script lang="ts">
  import IconPin from "~icons/tabler/pin";
  import IconTerminal from "~icons/tabler/terminal-2";
  import IconEye from "~icons/tabler/eye";
  import type { SessionInfo } from "../../contracts/schemas";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import { openSession } from "../../hooks/shared/shell.svelte";
  import { sessionRelativeTime } from "../../domains/sessions/session-status";
  import { sessionPins } from "../../domains/sessions/session-pins.svelte";
  import {
    queueGroup,
    type QueueGroup,
  } from "../../domains/workspace/session-queue";
  import LaunchConfigIcon from "../settings/LaunchConfigIcon.svelte";
  import SidebarSessionMenu from "../shell/SidebarSessionMenu.svelte";
  import SessionStatus from "../sessions/SessionStatus.svelte";

  let {
    session,
    group,
    location,
    now,
    previewing,
    onpreview,
    onfocusrow,
    onblurrow,
    onmenuopen,
    onrename,
    onclose,
  }: {
    session: SessionInfo;
    group: QueueGroup;
    location: string;
    now: number;
    previewing: boolean;
    onpreview: (trigger: HTMLElement) => void;
    onfocusrow: () => void;
    onblurrow: () => void;
    onmenuopen: (open: boolean) => void;
    onrename: (session: SessionInfo) => void;
    onclose: (session: SessionInfo) => void;
  } = $props();

  const attention = $derived(session.attention?.kind);
  const needsAttention = $derived(group === "attention");
  const activeAttention = $derived(queueGroup(session) === "attention");
  const relative = $derived(
    sessionRelativeTime(
      now,
      activeAttention
        ? (session.attention?.since ?? session.lastActiveAt)
        : session.lastActiveAt,
    ),
  );
  const attentionLabel = $derived(
    attention === "permission"
      ? m.s8_queue_permission()
      : attention === "question"
        ? m.s8_queue_question()
        : attention === "error"
          ? m.s8_queue_error()
          : m.s8_queue_waiting(),
  );
  const pinned = $derived(sessionPins.has(session.id));
</script>

<li
  class={[
    "group/row relative flex min-w-0 items-start gap-1 border-b",
    needsAttention && "rounded-md border border-l-2 bg-card",
    needsAttention &&
      (attention === "error" ? "border-l-destructive" : "border-l-warning"),
    previewing && "bg-muted/70",
  ]}
  data-session-id={session.id}
  data-status={session.status}
  data-queue-group={group}
  onfocusin={onfocusrow}
  onfocusout={(event) => {
    if (!(
      event.relatedTarget instanceof Node &&
      event.currentTarget.contains(event.relatedTarget)
    ))
      onblurrow();
  }}
>
  <a
    class="grid min-h-13 min-w-0 flex-1 grid-cols-1 items-center gap-1 rounded-sm px-3 py-2 hover:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]"
    data-session-link
    aria-label={session.title}
    href={`/s/${session.id}`}
    onclick={(event) => {
      if (
        event.ctrlKey ||
        event.metaKey ||
        event.shiftKey ||
        event.altKey ||
        event.button !== 0
      )
        return;
      event.preventDefault();
      openSession(session);
    }}
  >
    <span
      class="flex min-w-0 items-center gap-2 text-sm font-medium"
      title={session.title}
    >
      {#if session.agent?.icon}<LaunchConfigIcon
          icon={session.agent.icon}
          size="xs"
        />
      {:else}<IconTerminal
          class="size-4 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />{/if}
      <span class="truncate">{session.title}</span>
      {#if pinned}<span
          class="shrink-0 text-muted-foreground"
          title={m.s8_queue_pin()}
        >
          <IconPin class="size-3.5" aria-hidden="true" /><span class="sr-only"
            >{m.s8_queue_pin()}</span
          >
        </span>{/if}
    </span>
    <span
      class="min-w-0 truncate font-mono text-xs text-muted-foreground"
      title={location}>{location}</span
    >
    <span class="flex min-w-0 flex-wrap items-center gap-2 text-xs">
      {#if activeAttention}
        <span
          class={attention === "error" ? "text-destructive" : "text-warning"}
          >{attentionLabel}</span
        >
      {:else}<SessionStatus {session} />{/if}
      <span class="text-muted-foreground tabular-nums">{relative}</span>
    </span>
    {#if activeAttention}
      {#if session.attention?.message}
        <span
          class="line-clamp-2 min-w-0 text-sm wrap-anywhere sm:col-span-full"
          >{session.attention.message}</span
        >
      {/if}
      <span class="text-xs font-medium text-primary sm:col-span-full"
        >{m.s8_queue_review()} <span aria-hidden="true">→</span></span
      >
    {:else if group === "running" && session.agent?.work}
      <span
        class="min-w-0 truncate font-mono text-xs text-muted-foreground sm:col-span-full"
        title={session.agent.work}>{session.agent.work}</span
      >
    {/if}
  </a>
  <div class="my-1 flex shrink-0 items-center gap-1 pr-1">
    <Button
      size="icon-touch-sm"
      variant="ghost"
      data-queue-preview
      aria-pressed={previewing}
      aria-label={m.s8_queue_preview({ title: session.title })}
      title={m.s8_queue_preview({ title: session.title })}
      onclick={(event) => onpreview(event.currentTarget)}
      ><IconEye aria-hidden="true" /></Button
    >
    <div class="relative flex size-11 items-center justify-center sm:size-8">
      <SidebarSessionMenu
        label={session.title}
        {onmenuopen}
        target={{ kind: "live", session, onrename, onclose }}
      />
    </div>
  </div>
</li>
