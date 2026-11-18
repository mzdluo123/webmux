<script lang="ts">
  import { ListButton } from "$lib/components/ui/list-button";
  import type { SessionInfo } from "../../contracts/schemas";
  import {
    rowStatus,
    sessionRowLabel,
    sessionRowTitle,
  } from "../../domains/sessions/session-status";
  import LaunchConfigIcon from "../settings/LaunchConfigIcon.svelte";
  import RowStatus from "../shared/RowStatus.svelte";
  import SidebarSessionName from "./SidebarSessionName.svelte";
  import SidebarSessionMenu from "./SidebarSessionMenu.svelte";
  import { m } from "$lib/paraglide/messages.js";

  let {
    session,
    current,
    icon,
    pinned = false,
    location,
    locationIsBranch = false,
    compact = false,
    onopen,
    onrename,
    onclose,
    onmenuopen,
  }: {
    session: SessionInfo;
    current: string;
    icon: string;
    pinned?: boolean;
    location?: string;
    locationIsBranch?: boolean;
    compact?: boolean;
    onopen: (session: SessionInfo) => void;
    onrename: (session: SessionInfo) => void;
    onclose: (session: SessionInfo) => void;
    onmenuopen?: (open: boolean) => void;
  } = $props();
  const locationId = $props.id();
  let showFullLabel = $state(false);

  function onLabelHover(event: PointerEvent & { currentTarget: HTMLElement }) {
    showFullLabel = Array.from(
      event.currentTarget.querySelectorAll<HTMLElement>(
        "[data-sidebar-session-text]",
      ),
    ).some((label) => label.scrollWidth > label.clientWidth);
  }
</script>

<li class="group/row relative" data-sidebar-session-id={session.id}>
  <ListButton
    variant={compact ? "navigation-compact-action" : "navigation-action"}
    aria-current={session.id === current ? "page" : undefined}
    aria-label={pinned
      ? m.s1_pinned_prefix({ label: sessionRowLabel(session) })
      : sessionRowLabel(session)}
    title={locationIsBranch
      ? showFullLabel
        ? [session.title, location].join("\n")
        : undefined
      : sessionRowTitle(session)}
    onpointerenter={locationIsBranch ? onLabelHover : undefined}
    onpointerleave={locationIsBranch
      ? () => (showFullLabel = false)
      : undefined}
    aria-describedby={location ? locationId : undefined}
    data-allow-wrap={location ? true : undefined}
    onclick={() => onopen(session)}
  >
    <LaunchConfigIcon {icon} size={compact ? "xs" : "sm"} />
    <SidebarSessionName
      title={session.title}
      {location}
      {locationId}
      {locationIsBranch}
    />
    <RowStatus status={rowStatus(session)} />
  </ListButton>
  <SidebarSessionMenu
    label={pinned
      ? m.s1_pinned_prefix({ label: session.title })
      : session.title}
    target={{ kind: "live", session, onrename, onclose }}
    {onmenuopen}
  />
</li>
