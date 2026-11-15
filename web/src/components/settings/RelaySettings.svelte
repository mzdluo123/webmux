<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { locale } from "$lib/locale";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import * as Card from "$lib/components/ui/card";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import type { RelaySettingsView } from "../../hooks/settings/relay-settings.svelte";
  import {
    relayDiagnostic,
    relayEndpoint,
  } from "../../domains/settings/relay-diagnostics";

  let {
    view,
    onrefresh,
    onpair,
    oncancel,
    onenable,
    ondisconnect,
    onreconnect,
    ondelete,
  }: {
    view: RelaySettingsView;
    onrefresh: () => void;
    onpair: (rebind: boolean) => void;
    oncancel: () => void;
    onenable: () => void;
    ondisconnect: () => void;
    onreconnect: () => void;
    ondelete: () => void;
  } = $props();
  let confirmation = $state<
    "pair" | "rebind" | "enable" | "disconnect" | "reconnect" | "delete" | null
  >(null);
  const status = $derived(view.status);
  const blocked = $derived(view.busy || !!view.enrollment);
  const stateLabel = $derived({
    off: m.relay_state_off(),
    connecting: m.s1_connecting_short(),
    ready: m.relay_state_ready(),
    degraded: m.relay_state_degraded(),
    failed: m.relay_state_failed(),
  });
  const reachability = $derived({
    unknown: m.relay_reach_unknown(),
    reachable: m.relay_reach_reachable(),
    unavailable: m.relay_reach_unavailable(),
    unassigned: m.relay_unassigned(),
  });
  const confirmTitle = $derived(
    confirmation === "pair"
      ? m.relay_confirm_pair()
      : confirmation === "rebind"
        ? m.relay_confirm_rebind()
        : confirmation === "enable"
          ? m.relay_confirm_enable()
          : confirmation === "delete"
            ? m.relay_confirm_delete()
            : confirmation === "reconnect"
              ? m.relay_confirm_reconnect()
              : m.relay_confirm_disconnect(),
  );

  function confirm() {
    const action = confirmation;
    confirmation = null;
    if (action === "pair" || action === "rebind") onpair(action === "rebind");
    else if (action === "enable") onenable();
    else if (action === "disconnect") ondisconnect();
    else if (action === "reconnect") onreconnect();
    else if (action === "delete") ondelete();
  }
</script>

<Card.Root flat>
  <Card.Header>
    <Card.Title>{m.relay_settings_title()}</Card.Title>
    <Card.Description>{m.relay_settings_description()}</Card.Description>
  </Card.Header>
  <Card.Content class="space-y-5">
    {#if view.loading}
      <p role="status" aria-busy="true">{m.relay_loading()}</p>
    {:else if !status}
      <InlineNotice tone="warning">{m.relay_status_unavailable()}</InlineNotice>
    {:else}
      <div class="flex items-center justify-between gap-4">
        <Label for="relay-enabled">{m.relay_allow_public()}</Label>
        <Switch
          id="relay-enabled"
          bind:checked={
            () => status?.enabled === true,
            (enabled) => {
              confirmation = enabled ? "enable" : "disconnect";
            }
          }
          disabled={blocked ||
            (!status.enabled &&
              (!status.paired ||
                !status.tokenStrong ||
                !status.storageAvailable))}
        />
      </div>
      <p role="status" class="text-sm">
        {stateLabel[status.state]} · {status.paired
          ? m.relay_paired()
          : status.pairing
            ? m.relay_pairing()
            : m.relay_unpaired()} · {m.relay_baseline_count({
          count: String(status.readyConnections),
        })}
      </p>
      {#if !status.hybridSupported}
        <InlineNotice tone="warning"
          >{m.relay_hybrid_disabled({
            reason:
              status.hybridUnavailableReason === "p2p-build-unavailable"
                ? m.relay_build_unavailable()
                : m.relay_plugin_unselected(),
          })}</InlineNotice
        >
      {/if}
      {#if !status.controlEndpoint}
        <InlineNotice tone="warning"
          >{m.relay_control_unconfigured()}</InlineNotice
        >
      {/if}
      {#if !status.tokenStrong}
        <InlineNotice tone="warning">{m.relay_token_requirement()}</InlineNotice
        >
      {/if}
      {#if !status.storageAvailable}
        <InlineNotice tone="danger">{m.relay_storage_warning()}</InlineNotice>
      {/if}
      {#if status.ownerId}
        <p class="text-sm wrap-anywhere">
          {m.relay_owner({
            login: status.ownerLogin || m.relay_account(),
            id: status.ownerId,
          })}
        </p>
      {/if}
      {#if status.boundPublicUrl}
        <div class="space-y-2">
          <p class="text-sm text-muted-foreground">
            {m.relay_bound_address()}
          </p>
          <p class="text-sm wrap-anywhere">
            {relayEndpoint(status.boundPublicUrl)}
          </p>
          {#if status.controlReady && status.publicUrl}
            <Button
              href={new URL("/auth/relay/start", status.publicUrl).href}
              variant="outline">{m.relay_open_owner()}</Button
            >
          {/if}
        </div>
      {/if}
      {#if status.state === "degraded" || (status.controlReady && !status.bulkReady)}
        <InlineNotice tone="warning">{m.relay_bulk_warning()}</InlineNotice>
      {/if}
      {#if status.error || status.readinessError}
        <InlineNotice tone="danger"
          >{relayDiagnostic(
            status.error || status.readinessError,
          )}</InlineNotice
        >
      {/if}
      {#if status.reconnectReason}
        <InlineNotice tone="warning"
          >{relayDiagnostic(status.reconnectReason)}</InlineNotice
        >
      {/if}
      {#if view.enrollment}
        <section class="space-y-3" aria-label={m.relay_device_authorization()}>
          <p class="text-sm">{m.relay_enter_code()}</p>
          <p class="font-mono text-2xl" aria-label={m.relay_pairing_code()}>
            {view.enrollment.userCode}
          </p>
          <p class="text-xs text-muted-foreground">
            {m.relay_pairing_expiry({
              time: new Date(
                view.enrollment.expiresAt * 1000,
              ).toLocaleTimeString(locale.current),
            })}
          </p>
          <div class="flex flex-wrap gap-3">
            <Button
              href={view.enrollment.verificationUrl}
              target="_blank"
              rel="noopener noreferrer">{m.relay_verify_github()}</Button
            >
            <Button variant="outline" onclick={oncancel} disabled={view.busy}
              >{m.relay_cancel_pairing()}</Button
            >
          </div>
        </section>
      {:else}
        <div class="flex flex-wrap gap-3">
          <Button
            disabled={blocked ||
              !status.storageAvailable ||
              !status.controlEndpoint}
            onclick={() => {
              confirmation = status.paired ? "rebind" : "pair";
            }}
          >
            {status.paired ? m.relay_rebind() : m.relay_start_pairing()}
          </Button>
          {#if status.paired}
            <Button
              variant="outline"
              disabled={blocked || !status.enabled}
              onclick={() => {
                confirmation = "reconnect";
              }}>{m.relay_reconnect()}</Button
            >
            <Button
              variant="destructive-outline"
              disabled={blocked}
              onclick={() => {
                confirmation = "delete";
              }}>{m.relay_delete_binding()}</Button
            >
          {/if}
        </div>
      {/if}
      {#if status.sessions.length}
        <section class="space-y-2" aria-label={m.relay_host_candidates()}>
          <h3 class="text-sm font-medium">{m.relay_host_candidates()}</h3>
          {#each status.sessions as session (session.id)}
            <p class="text-sm wrap-anywhere">
              {m.relay_session_path({
                id: session.id.slice(0, 8),
                path: session.path,
                reason: relayDiagnostic(session.reason) || m.relay_no_reason(),
              })}
            </p>
          {/each}
          <p class="text-xs text-muted-foreground">
            {m.relay_candidate_disclaimer()}
          </p>
        </section>
      {/if}
      <details class="space-y-3">
        <summary class="cursor-pointer text-sm"
          >{m.relay_effective_diagnostics()}</summary
        >
        <dl class="grid min-w-0 grid-cols-1 gap-2 text-sm sm:grid-cols-2">
          <dt class="text-muted-foreground">{m.relay_configured_control()}</dt>
          <dd class="wrap-anywhere">
            {relayEndpoint(status.controlEndpoint)} · {status.controlSource}
          </dd>
          <dt class="text-muted-foreground">{m.relay_control_trust()}</dt>
          <dd>
            {m.relay_control_ca_trust()}
          </dd>
          <dt class="text-muted-foreground">
            {m.relay_control_reachability()}
          </dt>
          <dd>{reachability[status.controlReachability]}</dd>
          <dt class="text-muted-foreground">{m.relay_assignment_source()}</dt>
          <dd>
            {status.assignmentSource === "authenticated-control"
              ? m.relay_authenticated_control()
              : m.relay_unassigned()} · {m.relay_revision({
              revision: status.assignmentRevision,
            })}
          </dd>
          <dt class="text-muted-foreground">{m.relay_connector_source()}</dt>
          <dd class="wrap-anywhere">
            {relayEndpoint(status.connectorUrl)} / {relayEndpoint(
              status.dataOrigin,
            )}
          </dd>
          <dt class="text-muted-foreground">{m.relay_data_reachability()}</dt>
          <dd>{reachability[status.dataReachability]}</dd>
          <dt class="text-muted-foreground">{m.relay_node_capabilities()}</dt>
          <dd class="wrap-anywhere">
            {status.nodeId || m.relay_unassigned()} · {status.capabilities.join(
              " / ",
            ) || m.relay_none()}
          </dd>
          <dt class="text-muted-foreground">{m.relay_applied_revision()}</dt>
          <dd>
            {status.configRevision} / {status.appliedRevision === "0"
              ? m.relay_unknown()
              : status.appliedRevision}
          </dd>
          <dt class="text-muted-foreground">{m.relay_tcp_algorithm()}</dt>
          <dd>{relayDiagnostic(status.tcpAlgorithm) || m.relay_unknown()}</dd>
        </dl>
        <p class="text-xs text-muted-foreground">
          {m.relay_assignment_disclaimer()}
        </p>
      </details>
    {/if}
    {#if view.error}<InlineNotice tone="danger">{view.error}</InlineNotice>{/if}
    {#if view.notice}<InlineNotice>{view.notice}</InlineNotice>{/if}
    <Button variant="outline" onclick={onrefresh} disabled={blocked}
      >{m.relay_refresh()}</Button
    >
  </Card.Content>
</Card.Root>

<AlertDialog.Root
  open={confirmation !== null}
  onOpenChange={(open) => {
    if (!open) confirmation = null;
  }}
>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{confirmTitle}</AlertDialog.Title>
      <AlertDialog.Description>
        {#if confirmation === "pair" || confirmation === "rebind" || confirmation === "enable"}
          {m.relay_consent()}
          {#if confirmation === "rebind"}{m.relay_rebind_consent()}{/if}
        {:else if confirmation === "delete"}
          {m.relay_delete_consent()}
        {:else}
          {m.relay_disconnect_consent()}
        {/if}
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>{m.s1_cancel()}</AlertDialog.Cancel>
      <Button
        variant={confirmation === "delete" ? "destructive" : "default"}
        onclick={confirm}
        >{confirmation === "enable"
          ? m.relay_confirm_enable_action()
          : m.relay_confirm_action()}</Button
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
