<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import { Textarea } from "$lib/components/ui/textarea";
  import { useP2PSettings } from "../../hooks/settings/p2p-settings.svelte";
  import {
    p2pState,
    runP2PEchoABTest,
    type P2PPhase,
  } from "../../platform/p2p.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";

  const p2p = useP2PSettings();
  const ui = p2p.ui;
  const status = $derived(ui.status);
  const canEdit = $derived(
    !ui.saving && !!status?.supported && !!status?.storageAvailable,
  );
  const canReset = $derived(canEdit && status?.source === "web");

  let stunText = $state("");
  let stunDirty = $state(false);
  let stunError = $state("");

  $effect(() => {
    void ui.revision;
    if (status && !stunDirty) {
      stunText = status.stunServers.join("\n");
      stunError = "";
    }
  });

  function parseStunLines(raw: string): string[] | null {
    const lines = raw
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter((line) => line.length > 0);
    if (lines.length > 8) {
      stunError = m.s2_p2p_err_max_stun();
      return null;
    }
    for (const line of lines) {
      if (!/^stuns?:[^\s/?#]+/i.test(line)) {
        stunError = m.s2_p2p_err_invalid_stun({ url: line });
        return null;
      }
    }
    stunError = "";
    return lines;
  }

  async function toggleEnabled(checked: boolean) {
    if (!status) return;
    await p2p.save({
      enabled: checked,
      portMapping: status.portMapping,
      stunServers: status.stunServers,
    });
  }

  async function togglePortMapping(checked: boolean) {
    if (!status) return;
    await p2p.save({
      enabled: status.enabled,
      portMapping: checked,
      stunServers: status.stunServers,
    });
  }

  async function saveStunServers() {
    if (!status) return;
    const parsed = parseStunLines(stunText);
    if (!parsed) return;
    const ok = await p2p.save({
      enabled: status.enabled,
      portMapping: status.portMapping,
      stunServers: parsed,
    });
    if (ok) stunDirty = false;
  }

  async function resetToConfig() {
    const ok = await p2p.reset();
    if (ok) {
      stunDirty = false;
      stunError = "";
    }
  }

  function phaseText(phase: P2PPhase): string {
    switch (phase) {
      case "idle":
        return m.s2_p2p_phase_idle();
      case "disabled":
        return m.s2_p2p_phase_disabled();
      case "unsupported":
        return m.s2_p2p_phase_unsupported();
      case "skipped_local":
        return m.s2_p2p_phase_skipped_local();
      case "skipped_low_rtt":
        return m.s2_p2p_phase_skipped_low_rtt();
      case "probing":
        return m.s2_p2p_phase_probing();
      case "negotiating":
        return m.s2_p2p_phase_negotiating();
      case "connected":
        return m.s2_p2p_phase_connected();
      case "rejected_relay":
        return m.s2_p2p_phase_rejected_relay();
      case "direct_failed":
        return m.s2_p2p_phase_direct_failed();
      case "standby_hysteresis":
        return m.s2_p2p_phase_standby();
      case "fallback":
        return m.s2_p2p_phase_fallback();
    }
  }

  const portMapBadge = $derived.by(() => {
    const pm = status?.portMap;
    if (!pm || pm.state === "off") {
      return { variant: "outline" as const, text: m.s2_p2p_phase_disabled() };
    }
    if (pm.state === "probing") {
      return { variant: "info" as const, text: m.s2_p2p_portmap_probing() };
    }
    if (pm.state === "mapped" && pm.externalPort) {
      const proto = (pm.protocol ?? "UDP").toUpperCase();
      return {
        variant: "success" as const,
        text: m.s2_p2p_portmap_mapped({
          proto,
          port: String(pm.externalPort),
        }),
      };
    }
    return {
      variant: "secondary" as const,
      text: m.s2_p2p_portmap_unsupported(),
    };
  });
</script>

<div class="space-y-4">
  <div class="flex flex-wrap items-center justify-between gap-2">
    <div class="space-y-1">
      <h3 class="text-base font-medium">{m.s2_p2p_title()}</h3>
      <p class="text-xs text-muted-foreground">
        {m.s2_p2p_desc()}
      </p>
    </div>
    {#if status}
      <Badge variant={status.source === "web" ? "info" : "outline"}>
        {status.source === "web" ? m.s2_source_web() : m.s2_source_config()}
      </Badge>
    {/if}
  </div>

  {#if ui.failure && !status}
    <InlineNotice tone="warning"
      >{m.s2_p2p_err_load({ error: ui.failure })}</InlineNotice
    >
  {:else if status && !status.supported}
    <InlineNotice tone="info">
      {m.s2_p2p_nop2p_before()}<code>-tags nop2p</code>{m.s2_p2p_nop2p_after()}
    </InlineNotice>
  {:else if status}
    {#if !status.storageAvailable && status.storageError}
      <InlineNotice tone="warning"
        >{m.s2_p2p_storage_err({ error: status.storageError })}</InlineNotice
      >
    {/if}

    <div class="space-y-2">
      <div class="flex items-center justify-between gap-4">
        <Label for="p2p-enabled">{m.s2_p2p_enable_label()}</Label>
        <Switch
          id="p2p-enabled"
          checked={status.enabled}
          disabled={!canEdit}
          aria-describedby="p2p-enabled-help"
          onCheckedChange={(checked) => void toggleEnabled(checked)}
        />
      </div>
      <p id="p2p-enabled-help" class="text-xs text-muted-foreground">
        {m.s2_p2p_enable_help()}
      </p>
    </div>

    <div class="space-y-2">
      <div class="flex items-center justify-between gap-4">
        <div class="flex flex-wrap items-center gap-2">
          <Label for="p2p-port-mapping">{m.s2_p2p_portmap_label()}</Label>
          <Badge variant={portMapBadge.variant}>{portMapBadge.text}</Badge>
        </div>
        <Switch
          id="p2p-port-mapping"
          checked={status.portMapping}
          disabled={!canEdit || !status.enabled}
          aria-describedby="p2p-port-mapping-help"
          onCheckedChange={(checked) => void togglePortMapping(checked)}
        />
      </div>
      <p id="p2p-port-mapping-help" class="text-xs text-muted-foreground">
        {m.s2_p2p_portmap_help()}
      </p>
    </div>

    <div class="space-y-2">
      <Label for="p2p-stun-servers">{m.s2_p2p_stun_label()}</Label>
      <Textarea
        id="p2p-stun-servers"
        code
        rows={3}
        disabled={!canEdit}
        bind:value={stunText}
        oninput={() => {
          stunDirty = true;
          if (stunError) parseStunLines(stunText);
        }}
        aria-invalid={!!stunError}
        aria-describedby="p2p-stun-help"
        placeholder="stun:stun.cloudflare.com:3478"
      />
      <p id="p2p-stun-help" class="text-xs text-muted-foreground">
        {m.s2_p2p_stun_help_1()}<code>stun:</code>{m.s2_p2p_stun_help_2()}<code
          >stuns:</code
        >{m.s2_p2p_stun_help_3()}<code>turn:</code>{m.s2_p2p_stun_help_4()}
      </p>
      {#if stunError}
        <p role="alert" class="text-xs text-destructive">{stunError}</p>
      {/if}
      <div class="flex flex-wrap items-center gap-2">
        <Button
          variant="outline"
          disabled={!canEdit || !stunDirty}
          onclick={() => void saveStunServers()}>{m.s2_p2p_save_stun()}</Button
        >
        <Button
          variant="outline"
          disabled={!canReset}
          onclick={() => void resetToConfig()}>{m.s2_p2p_reset_config()}</Button
        >
        {#if ui.feedback}
          <span role="status" class="text-xs text-muted-foreground"
            >{ui.feedback}</span
          >
        {/if}
      </div>
    </div>

    {#if ui.failure}
      <InlineNotice tone="danger">{ui.failure}</InlineNotice>
    {/if}

    <section
      aria-label={m.s2_p2p_diag_aria()}
      class="space-y-3 rounded-lg border bg-muted/30 p-3"
    >
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div class="flex flex-wrap items-center gap-2" role="status">
          <Badge variant={p2pState.mode === "p2p" ? "success" : "secondary"}>
            {p2pState.mode === "p2p"
              ? m.s2_p2p_mode_direct()
              : m.s2_p2p_mode_ws()}
          </Badge>
          <span class="text-xs text-muted-foreground"
            >{phaseText(p2pState.phase)}</span
          >
        </div>
        <Button
          variant="outline"
          disabled={p2pState.abTest.running || !status.supported}
          onclick={() => void runP2PEchoABTest(status.stunServers)}
        >
          {p2pState.abTest.running ? m.s2_p2p_ab_running() : m.s2_p2p_ab_run()}
        </Button>
      </div>

      <dl class="grid gap-x-6 gap-y-2 text-xs sm:grid-cols-3">
        <div>
          <dt class="text-muted-foreground">{m.s2_p2p_dt_candidate()}</dt>
          <dd class="font-mono">
            {p2pState.candidateType
              ? `${p2pState.candidateType}${p2pState.ipFamily ? ` (${p2pState.ipFamily})` : ""}`
              : "—"}
          </dd>
        </div>
        <div>
          <dt class="text-muted-foreground">{m.s2_p2p_dt_rtt()}</dt>
          <dd class="font-mono">
            {p2pState.baselineRttMs !== null
              ? `${p2pState.baselineRttMs} ms`
              : "—"} / {p2pState.p2pRttMs !== null
              ? `${p2pState.p2pRttMs} ms`
              : "—"}
          </dd>
        </div>
        <div>
          <dt class="text-muted-foreground">{m.s2_p2p_dt_timing()}</dt>
          <dd class="font-mono">
            {p2pState.firstReadyMs !== null
              ? `${p2pState.firstReadyMs} ms`
              : "—"} / {p2pState.upgradeDurationMs !== null
              ? `${p2pState.upgradeDurationMs} ms`
              : "—"} / {m.s2_p2p_switch_count({
              count: String(p2pState.switchCount),
            })}
          </dd>
        </div>
      </dl>

      {#if p2pState.abTest.error}
        <p role="alert" class="text-xs text-destructive">
          {m.s2_p2p_ab_err({ error: p2pState.abTest.error })}
        </p>
      {:else if p2pState.abTest.baselineMedianMs !== null}
        <div
          role="status"
          class="grid gap-2 border-t pt-2 text-xs sm:grid-cols-2"
        >
          <div>
            <span class="text-muted-foreground"
              >{m.s2_p2p_ab_baseline_label()}</span
            >
            <span class="font-mono"
              >{m.s2_p2p_ab_stats({
                min: String(p2pState.abTest.baselineMinMs),
                median: String(p2pState.abTest.baselineMedianMs),
              })}</span
            >
          </div>
          <div>
            <span class="text-muted-foreground"
              >{m.s2_p2p_ab_direct_label()}</span
            >
            {#if p2pState.abTest.directSuccess}
              <span class="font-mono text-success"
                >{m.s2_p2p_ab_direct_stats({
                  min: String(p2pState.abTest.p2pMinMs),
                  median: String(p2pState.abTest.p2pMedianMs),
                  candidate: `${p2pState.abTest.candidateType || "direct"}${p2pState.abTest.ipFamily ? `/${p2pState.abTest.ipFamily}` : ""}`,
                })}</span
              >
            {:else}
              <span class="text-muted-foreground"
                >{m.s2_p2p_ab_no_direct()}</span
              >
            {/if}
          </div>
        </div>
      {/if}
    </section>
  {/if}
</div>
