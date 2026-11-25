<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import RelayDiagnosticSection from "./RelayDiagnosticSection.svelte";
  import type { RelayStatus } from "../../contracts/relay-schemas";
  import RelayPathStatus from "./RelayPathStatus.svelte";
  import {
    relayDiagnostic,
    relayEndpoint,
  } from "../../domains/settings/relay-diagnostics";

  let {
    status,
    onreconnect,
    blocked,
  }: {
    status: RelayStatus;
    onreconnect: () => void;
    blocked: boolean;
  } = $props();
  const reachability = $derived({
    unknown: m.relay_reach_unknown(),
    reachable: m.relay_reach_reachable(),
    unavailable: m.relay_reach_unavailable(),
    unassigned: m.relay_unassigned(),
  });
</script>

<RelayDiagnosticSection title={m.relay_effective_diagnostics()}>
  <div class="space-y-4">
    <section class="space-y-2" aria-label={m.relay_health()}>
      <h3 class="text-sm font-medium">{m.relay_health()}</h3>
      <dl class="grid min-w-0 grid-cols-1 gap-2 text-sm sm:grid-cols-2">
        <dt>{m.relay_baseline_transport()}</dt>
        <dd>{status.transport.toUpperCase()}</dd>
        <dt>{m.relay_control_readiness()}</dt>
        <dd>
          {status.controlReady ? m.relay_state_ready() : m.relay_not_ready()}
        </dd>
        <dt>{m.relay_data_readiness()}</dt>
        <dd>
          {status.dataReady ? m.relay_state_ready() : m.relay_not_ready()}
        </dd>
        <dt>{m.relay_control_reachability()}</dt>
        <dd>{reachability[status.controlReachability]}</dd>
        <dt>{m.relay_data_reachability()}</dt>
        <dd>{reachability[status.dataReachability]}</dd>
        <dt>{m.relay_tcp_algorithm()}</dt>
        <dd>{relayDiagnostic(status.tcpAlgorithm) || m.relay_unknown()}</dd>
      </dl>
      {#if !status.hybridSupported}
        <p class="text-sm">
          {m.relay_hybrid_disabled({
            reason:
              status.hybridUnavailableReason === "p2p-build-unavailable"
                ? m.relay_build_unavailable()
                : m.relay_feature_unselected(),
          })}
        </p>
      {/if}
      {#if status.reconnectReason}
        <p class="text-sm wrap-anywhere">
          {relayDiagnostic(status.reconnectReason)}
        </p>
      {/if}
      {#if status.paired}
        <Button
          variant="outline"
          onclick={onreconnect}
          disabled={blocked || !status.enabled}>{m.relay_reconnect()}</Button
        >
      {/if}
    </section>
    <RelayDiagnosticSection title={m.relay_routing()}>
      <div>
        <dl class="grid min-w-0 grid-cols-1 gap-2 text-sm sm:grid-cols-2">
          <dt>{m.relay_configured_control()}</dt>
          <dd class="wrap-anywhere">
            {relayEndpoint(status.controlEndpoint)} · {status.controlSource}
          </dd>
          <dt>{m.relay_control_trust()}</dt>
          <dd>{m.relay_control_ca_trust()}</dd>
          <dt>{m.relay_assignment_source()}</dt>
          <dd>
            {status.assignmentSource === "authenticated-control"
              ? m.relay_authenticated_control()
              : m.relay_unassigned()} · {m.relay_revision({
              revision: status.assignmentRevision,
            })}
          </dd>
          <dt>{m.relay_connector_source()}</dt>
          <dd class="wrap-anywhere">
            {relayEndpoint(status.connectorUrl)} / {relayEndpoint(
              status.dataOrigin,
            )}
          </dd>
          <dt>{m.relay_node_capabilities()}</dt>
          <dd class="wrap-anywhere">
            {status.nodeId || m.relay_unassigned()} · {status.capabilities.join(
              " / ",
            ) || m.relay_none()}
          </dd>
          <dt>{m.relay_applied_revision()}</dt>
          <dd>
            {status.configRevision} / {status.appliedRevision === "0"
              ? m.relay_unknown()
              : status.appliedRevision}
          </dd>
          <dt>{m.relay_account()}</dt>
          <dd class="wrap-anywhere">
            {m.relay_owner({
              login: status.ownerLogin || m.relay_account(),
              id: status.ownerId || m.relay_unassigned(),
            })}
          </dd>
        </dl>
        <p class="mt-3 text-xs text-muted-foreground">
          {m.relay_assignment_disclaimer()}
        </p>
      </div>
    </RelayDiagnosticSection>
    <RelayDiagnosticSection title={m.relay_host_candidates()}>
      <div class="space-y-2">
        {#each status.sessions as session (session.id)}
          <p class="text-sm wrap-anywhere">
            {m.relay_session_path({
              id: session.id.slice(0, 8),
              path: session.path,
              reason: relayDiagnostic(session.reason) || m.relay_no_reason(),
            })} · {relayDiagnostic(session.candidateType)} · {relayDiagnostic(
              session.transport,
            )}
          </p>
        {:else}
          <p class="text-sm">{m.relay_none()}</p>
        {/each}
        <p class="text-xs text-muted-foreground">
          {m.relay_candidate_disclaimer()}
        </p>
      </div>
    </RelayDiagnosticSection>
    <RelayPathStatus />
  </div>
</RelayDiagnosticSection>
