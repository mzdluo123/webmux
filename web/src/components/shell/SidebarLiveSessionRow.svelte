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
    compact = false,
    onopen,
    onrename,
    onclose,
  }: {
    session: SessionInfo;
    current: string;
    icon: string;
    pinned?: boolean;
    location?: string;
    compact?: boolean;
    onopen: (session: SessionInfo) => void;
    onrename: (session: SessionInfo) => void;
    onclose: (session: SessionInfo) => void;
  } = $props();
  const locationId = $props.id();
</script>

<li class="group/row relative" data-sidebar-session-id={session.id}>
  <ListButton
    variant={compact ? "navigation-compact-action" : "navigation-action"}
    aria-current={session.id === current ? "page" : undefined}
    aria-label={pinned
      ? m.s1_pinned_prefix({ label: sessionRowLabel(session) })
      : sessionRowLabel(session)}
    title={sessionRowTitle(session)}
    aria-describedby={location ? locationId : undefined}
    data-allow-wrap={location ? true : undefined}
    onclick={() => onopen(session)}
  >
    <LaunchConfigIcon {icon} size={compact ? "xs" : "sm"} />
    <SidebarSessionName title={session.title} {location} {locationId} />
    <RowStatus status={rowStatus(session)} />
  </ListButton>
  <SidebarSessionMenu
    label={pinned
      ? m.s1_pinned_prefix({ label: session.title })
      : session.title}
    target={{ kind: "live", session, onrename, onclose }}
  />
</li>
