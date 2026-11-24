<script lang="ts">
  import IconDevices from "~icons/tabler/devices";
  import IconFingerprint from "~icons/tabler/fingerprint";
  import IconKey from "~icons/tabler/key";
  import AuthGateFrame from "./AuthGateFrame.svelte";
  import PasskeyContextGuard from "./PasskeyContextGuard.svelte";
  import TokenHelp from "../shared/TokenHelp.svelte";
  import { Button } from "$lib/components/ui/button";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import type { AuthStatus } from "../../contracts/auth-schemas";
  import {
    isWebAuthnSupported,
    localhostRedirectUrl,
  } from "../../platform/webauthn";
  import { m } from "$lib/paraglide/messages.js";

  let {
    mode = "auth",
    authStatus = null,
    checking,
    token = $bindable(""),
    error,
    onconnect,
    onretry,
    onusetoken,
    onpasskeylogin,
    onrecoverylogin,
    onopenpair,
  }: {
    mode?: "auth" | "offline" | "storage_error";
    authStatus?: AuthStatus | null;
    checking: boolean;
    token: string;
    error: string;
    onconnect: () => void;
    onretry?: () => void;
    onusetoken?: () => void;
    onpasskeylogin?: () => void;
    onrecoverylogin?: (code: string) => void;
    onopenpair?: () => void;
  } = $props();

  let showRecovery = $state(false);
  let recoveryCode = $state("");

  const passkeyMode = $derived(authStatus?.state === "ready");
  const hostLabel = $derived(authStatus?.hostname || location.host);
  const rpId = $derived(authStatus?.rpId || location.hostname);
  const canUsePasskeyHere = $derived(
    isWebAuthnSupported() &&
      !localhostRedirectUrl() &&
      Boolean(authStatus?.rpIdValid),
  );
</script>

{#if mode === "storage_error"}
  <AuthGateFrame
    title={m.auth_storage_error_title()}
    subtitle={m.auth_storage_error_subtitle()}
  >
    <div class="space-y-4">
      <InlineNotice tone="danger">
        <div class="space-y-1">
          <p class="text-xs leading-relaxed">{m.auth_storage_error_help()}</p>
          {#if authStatus?.error || error}
            <p class="font-mono text-xs break-all">
              {authStatus?.error || error}
            </p>
          {/if}
        </div>
      </InlineNotice>
      <Button
        type="button"
        class="w-full"
        disabled={checking}
        onclick={() => onretry?.()}
      >
        {checking ? m.s1_connecting_ellipsis() : m.auth_retry_status()}
      </Button>
    </div>
  </AuthGateFrame>
{:else if mode === "offline"}
  <AuthGateFrame
    title={m.s1_connection_failed_headline()}
    subtitle={m.s1_connection_failed_subtitle()}
  >
    <div class="space-y-4">
      {#if error}<p role="alert" class="text-destructive">{error}</p>{/if}
      <Button
        type="button"
        class="w-full"
        disabled={checking}
        onclick={() => onretry?.()}
        >{checking
          ? m.s1_connecting_ellipsis()
          : m.s1_connection_retry()}</Button
      >
      {#if onusetoken && !passkeyMode}
        <Button
          type="button"
          variant="ghost"
          class="w-full"
          disabled={checking}
          onclick={onusetoken}>{m.s1_connection_use_token()}</Button
        >
      {/if}
    </div>
  </AuthGateFrame>
{:else if passkeyMode}
  <AuthGateFrame
    title={m.auth_passkey_login_title()}
    subtitle={m.auth_passkey_login_subtitle({ host: hostLabel })}
  >
    <div class="space-y-4">
      <PasskeyContextGuard {authStatus} />

      {#if canUsePasskeyHere && !authStatus?.hasPasskeyForRpId}
        <InlineNotice tone="info">
          <div class="space-y-1">
            <p class="font-medium">
              {m.auth_no_passkey_for_domain_title({ domain: rpId })}
            </p>
            <p class="text-xs leading-relaxed">
              {m.auth_no_passkey_for_domain_desc()}
            </p>
          </div>
        </InlineNotice>
      {/if}

      {#if error}<p role="alert" class="text-sm text-destructive">
          {error}
        </p>{/if}

      {#if authStatus?.hasPasskeyForRpId}
        <Button
          type="button"
          class="w-full"
          disabled={checking || !canUsePasskeyHere}
          onclick={() => onpasskeylogin?.()}
        >
          <IconFingerprint class="size-4" aria-hidden="true" />
          {checking
            ? m.auth_passkey_signing_in()
            : m.auth_passkey_login_button()}
        </Button>
      {/if}

      {#if onopenpair}
        <Button
          type="button"
          variant={authStatus?.hasPasskeyForRpId ? "outline" : "default"}
          class="w-full"
          disabled={checking}
          onclick={onopenpair}
        >
          <IconDevices class="size-4" aria-hidden="true" />
          {m.auth_pair_domain_button()}
        </Button>
      {/if}

      {#if onrecoverylogin}
        <div class="pt-1">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            class="w-full"
            aria-expanded={showRecovery}
            onclick={() => {
              showRecovery = !showRecovery;
            }}
          >
            <IconKey class="size-4" aria-hidden="true" />
            {m.auth_use_recovery_code()}
          </Button>
          {#if showRecovery}
            <form
              class="mt-3 space-y-3 rounded-lg border p-3"
              onsubmit={(event) => {
                event.preventDefault();
                if (recoveryCode.trim()) onrecoverylogin(recoveryCode.trim());
              }}
            >
              <div class="space-y-1.5">
                <Label for="recovery-code">{m.auth_recovery_code_label()}</Label
                >
                <Input
                  id="recovery-code"
                  type="text"
                  bind:value={recoveryCode}
                  placeholder={m.auth_recovery_code_placeholder()}
                  autocomplete="one-time-code"
                  disabled={checking}
                />
              </div>
              <Button
                type="submit"
                variant="outline"
                class="w-full"
                disabled={checking || !recoveryCode.trim()}
              >
                {m.auth_recovery_login_button()}
              </Button>
            </form>
          {/if}
        </div>
      {/if}
    </div>
  </AuthGateFrame>
{:else}
  <AuthGateFrame title={m.s1_gate_headline()} subtitle={m.s1_gate_subtitle()}>
    <form
      class="space-y-5"
      onsubmit={(event) => {
        event.preventDefault();
        onconnect();
      }}
    >
      <div class="space-y-2">
        <div class="flex items-center gap-1">
          <Label for="token">{m.s1_shared_token()}</Label>
          <TokenHelp />
        </div>
        <Input
          id="token"
          type="password"
          bind:value={token}
          disabled={checking}
          autocomplete="current-password"
        />
      </div>
      {#if error}<p role="alert" class="text-destructive">{error}</p>{/if}
      <Button type="submit" class="w-full" disabled={checking}
        >{checking ? m.s1_connecting_ellipsis() : m.s1_connect_webmux()}</Button
      >
    </form>
  </AuthGateFrame>
{/if}
