<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import { Skeleton } from "$lib/components/ui/skeleton";
  import * as Card from "$lib/components/ui/card";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import RelayDiagnostics from "./RelayDiagnostics.svelte";
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
    onpair: () => void;
    oncancel: () => void;
    onenable: () => void;
    ondisconnect: () => void;
    onreconnect: () => void;
    ondelete: () => void;
  } = $props();
  let confirmation = $state<
    "enable" | "disconnect" | "reconnect" | "delete" | null
  >(null);
  let now = $state(Date.now());
  let copied = $state<"code" | "address" | "failed" | null>(null);
  let copiedValue = $state("");
  const status = $derived(view.status);
  const pairing = $derived(view.pairing);
  const active = $derived(
    !status?.paired &&
      ((!!pairing &&
        [
          "preparing",
          "waiting",
          "retrying",
          "uncertain",
          "cancelling",
        ].includes(pairing.phase)) ||
        view.busy),
  );
  const blocked = $derived(view.busy || active);
  const remaining = $derived(
    Math.max(0, Math.ceil((pairing?.expiresAt ?? 0) - now / 1000)),
  );
  const codeVisible = $derived(
    active && !!pairing?.userCode && !!pairing.verificationUrl,
  );
  const stateLabel = $derived({
    off: m.relay_state_off(),
    connecting: m.s1_connecting_short(),
    ready: m.relay_state_ready(),
    degraded: m.relay_state_degraded(),
    failed: m.relay_state_failed(),
  });
  const pairingMessage = $derived(
    pairing?.phase === "denied"
      ? m.relay_error_denied()
      : pairing?.phase === "expired"
        ? m.relay_error_expired()
        : pairing?.phase === "interrupted"
          ? m.relay_pairing_interrupted()
          : pairing?.phase === "failed"
            ? pairing.error === "conflict"
              ? m.relay_error_conflict()
              : pairing.error === "storage-unavailable"
                ? m.relay_storage_warning()
                : m.relay_pairing_failed()
            : pairing?.phase === "uncertain"
              ? m.relay_pairing_uncertain()
              : pairing?.phase === "retrying"
                ? m.relay_pairing_retrying()
                : pairing?.phase === "cancelling"
                  ? m.relay_pairing_cancelling()
                  : active && !codeVisible
                    ? m.relay_pairing_preparing()
                    : "",
  );
  const stepError = $derived(
    view.phase === "submitting"
      ? ""
      : (view.error !== pairingMessage ? view.error : "") ||
          (status?.operation.phase === "uncertain"
            ? status.operation.kind === "delete"
              ? m.relay_delete_uncertain()
              : m.relay_operation_uncertain()
            : status?.operation.phase === "failed"
              ? status.operation.error === "conflict"
                ? m.relay_error_conflict()
                : status.operation.error === "forbidden"
                  ? m.relay_error_denied()
                  : status.operation.error === "unauthenticated"
                    ? m.relay_error_auth()
                    : status.operation.error === "storage-unavailable"
                      ? m.relay_error_persist()
                      : m.relay_operation_failed()
              : !active && (status?.error || status?.readinessError)
                ? relayDiagnostic(status.error || status.readinessError)
                : ""),
  );
  const confirmTitle = $derived(
    confirmation === "enable"
      ? m.relay_confirm_enable()
      : confirmation === "delete"
        ? m.relay_confirm_delete()
        : confirmation === "reconnect"
          ? m.relay_confirm_reconnect()
          : m.relay_confirm_disconnect(),
  );

  $effect(() => {
    if (!codeVisible) return;
    const timer = setInterval(() => {
      now = Date.now();
    }, 1000);
    return () => clearInterval(timer);
  });

  async function copy(value: string, kind: "code" | "address") {
    try {
      await navigator.clipboard.writeText(value);
      copiedValue = value;
      copied = kind;
    } catch {
      copied = "failed";
    }
  }

  function confirm() {
    const action = confirmation;
    confirmation = null;
    if (action === "enable") onenable();
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
    <section
      class="min-w-0 space-y-4"
      aria-label={m.relay_device_authorization()}
      aria-busy={view.loading || view.phase === "submitting"}
    >
      {#if view.loading}
        <div class="space-y-3" role="status">
          <span class="sr-only">{m.relay_loading()}</span>
          <Skeleton class="h-4 w-64 max-w-full" />
          <Skeleton class="h-4 w-80 max-w-full" />
          <Skeleton class="h-9 w-40" />
        </div>
      {:else if !status}
        <InlineNotice tone="warning"
          >{m.relay_status_unavailable()}</InlineNotice
        >
      {:else if !status.paired}
        {#if active}
          {#if codeVisible && pairing}
            <p class="text-sm">{m.relay_enter_code()}</p>
            <p
              class="font-mono text-3xl wrap-anywhere"
              aria-label={m.relay_pairing_code()}
            >
              {pairing.userCode}
            </p>
            <Button
              variant="outline"
              onclick={() => void copy(pairing.userCode, "code")}
              >{copied === "code" && copiedValue === pairing.userCode
                ? m.relay_copied()
                : m.relay_copy_code()}</Button
            >
            <p class="text-sm text-muted-foreground">
              {remaining > 0
                ? m.relay_pairing_expiry({
                    time: `${Math.floor(remaining / 60)}:${String(remaining % 60).padStart(2, "0")}`,
                  })
                : m.relay_pairing_expiry_check()}
            </p>
            <p class="text-sm">{m.relay_pairing_waiting()}</p>
          {/if}
          {#if pairingMessage}
            <InlineNotice tone="warning">{pairingMessage}</InlineNotice>
          {/if}
          <div class="flex flex-wrap gap-3">
            {#if codeVisible && pairing}
              <Button
                href={pairing.verificationUrl}
                target="_blank"
                rel="noopener noreferrer">{m.relay_verify_github()}</Button
              >
            {/if}
            <Button
              variant="outline"
              onclick={oncancel}
              disabled={view.phase === "submitting" ||
                pairing?.phase === "cancelling"}
              >{m.relay_cancel_pairing()}</Button
            >
          </div>
        {:else}
          <p class="text-sm">{m.relay_pairing_value()}</p>
          <p class="text-sm text-muted-foreground">{m.relay_pairing_trust()}</p>
          {#if pairingMessage}
            <InlineNotice
              tone={pairing?.phase === "denied" || pairing?.phase === "failed"
                ? "danger"
                : "warning"}>{pairingMessage}</InlineNotice
            >
          {/if}
          <Button
            disabled={blocked ||
              !pairing ||
              !status.storageAvailable ||
              !status.controlEndpoint}
            onclick={onpair}>{m.relay_start_pairing()}</Button
          >
        {/if}
      {:else}
        <p class="text-sm wrap-anywhere">
          {m.relay_owner_login({
            login: status.ownerLogin || m.relay_account(),
          })}
        </p>
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
                (!status.tokenStrong ||
                  !status.storageAvailable ||
                  !status.controlEndpoint))}
          />
        </div>
        <p role="status" class="text-sm">{stateLabel[status.state]}</p>
        {#if status.boundPublicUrl}
          <div class="space-y-2">
            <p class="text-xs text-muted-foreground">
              {m.relay_bound_address()}
            </p>
            <p class="text-sm wrap-anywhere">
              {relayEndpoint(status.boundPublicUrl)}
            </p>
            {#if status.controlReady && status.publicUrl}
              <div class="flex flex-wrap gap-3">
                <Button
                  variant="outline"
                  onclick={() => void copy(status.publicUrl, "address")}
                  >{copied === "address" && copiedValue === status.publicUrl
                    ? m.relay_copied()
                    : m.relay_copy_address()}</Button
                >
                <Button
                  href={new URL("/auth/relay/start", status.publicUrl).href}
                  variant="outline">{m.relay_open_owner()}</Button
                >
              </div>
            {/if}
          </div>
        {/if}
        {#if !status.tokenStrong}<InlineNotice tone="warning"
            >{m.relay_token_requirement()}</InlineNotice
          >{/if}
        {#if status.state === "degraded"}<InlineNotice tone="warning"
            >{m.relay_bulk_warning()}</InlineNotice
          >{/if}
      {/if}
      {#if status && !status.controlEndpoint}<InlineNotice tone="warning"
          >{m.relay_control_unconfigured()}</InlineNotice
        >{/if}
      {#if status && !status.storageAvailable}<InlineNotice tone="danger"
          >{m.relay_storage_warning()}</InlineNotice
        >{/if}
      {#if !view.loading}
        {#if view.phase === "submitting" && !active}
          <p role="status" class="text-sm">{m.relay_operation_submitting()}</p>
        {:else if view.phase === "verifying"}
          <InlineNotice tone="warning"
            >{m.relay_operation_verifying()}</InlineNotice
          >
        {/if}
        {#if stepError}
          <InlineNotice tone="danger">{stepError}</InlineNotice>
        {/if}
        {#if stepError || view.phase === "verifying" || pairing?.phase === "uncertain" || status?.operation.phase === "uncertain"}
          <Button
            variant="outline"
            onclick={onrefresh}
            disabled={view.phase === "submitting"}>{m.relay_refresh()}</Button
          >
        {/if}
        {#if view.notice}
          <InlineNotice
            tone={status?.operation.kind === "delete" &&
            status.operation.deleted &&
            !status.operation.accessClosed
              ? "warning"
              : "info"}>{view.notice}</InlineNotice
          >
        {/if}
        {#if copied === "failed"}<InlineNotice tone="warning"
            >{m.relay_copy_failed()}</InlineNotice
          >{/if}
      {/if}
    </section>
    {#if status && !view.loading}
      <RelayDiagnostics
        {status}
        {blocked}
        onreconnect={() => {
          confirmation = "reconnect";
        }}
      />
      {#if status.paired}
        <Collapsible.Root class="space-y-3">
          <Collapsible.Trigger>
            {#snippet child({ props })}
              <Button {...props} variant="outline"
                >{m.relay_danger_actions()}</Button
              >
            {/snippet}
          </Collapsible.Trigger>
          <Collapsible.Content>
            <Button
              variant="destructive-outline"
              disabled={blocked}
              onclick={() => {
                confirmation = "delete";
              }}>{m.relay_delete_binding()}</Button
            >
          </Collapsible.Content>
        </Collapsible.Root>
      {/if}
    {/if}
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
        {confirmation === "enable"
          ? m.relay_consent()
          : confirmation === "delete"
            ? m.relay_delete_consent()
            : m.relay_disconnect_consent()}
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
