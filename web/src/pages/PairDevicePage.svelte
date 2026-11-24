<script lang="ts">
  import { onMount } from "svelte";
  import IconFingerprint from "~icons/tabler/fingerprint";
  import AuthGateFrame from "../components/shell/AuthGateFrame.svelte";
  import PasskeyContextGuard from "../components/shell/PasskeyContextGuard.svelte";
  import { Button } from "$lib/components/ui/button";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import type { AuthStatus } from "../contracts/auth-schemas";
  import {
    consumeAuthFragment,
    pairDeviceWithPasskey,
  } from "../platform/auth-api";
  import { errorMessage } from "../platform/error";
  import {
    defaultDeviceLabel,
    isWebAuthnSupported,
    localhostRedirectUrl,
  } from "../platform/webauthn";
  import { m } from "$lib/paraglide/messages.js";

  let {
    authStatus,
    onpaired,
    onback,
  }: {
    authStatus: AuthStatus | null;
    onpaired: () => void;
    onback: () => void;
  } = $props();

  let code = $state("");
  let name = $state(defaultDeviceLabel());
  let busy = $state(false);
  let error = $state("");

  const rpId = $derived(authStatus?.rpId || location.hostname);
  const canPair = $derived(
    isWebAuthnSupported() &&
      !localhostRedirectUrl() &&
      Boolean(authStatus?.rpIdValid),
  );

  async function handlePair() {
    if (!code.trim()) return;
    busy = true;
    error = "";
    try {
      await pairDeviceWithPasskey(
        code.trim(),
        name.trim() || defaultDeviceLabel(),
      );
      onpaired();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    const fromHash = consumeAuthFragment("#code=");
    if (fromHash) code = fromHash;
  });
</script>

<AuthGateFrame
  title={m.pair_page_title()}
  subtitle={m.pair_page_subtitle({ domain: rpId })}
>
  <div class="space-y-5">
    <PasskeyContextGuard {authStatus} />
    {#if error}
      <InlineNotice tone="danger">{error}</InlineNotice>
    {/if}

    <form
      class="space-y-4"
      onsubmit={(event) => {
        event.preventDefault();
        void handlePair();
      }}
    >
      <div class="space-y-2">
        <Label for="pair-code">{m.pair_code_label()}</Label>
        <Input
          id="pair-code"
          type="text"
          bind:value={code}
          placeholder={m.pair_code_placeholder()}
          autocomplete="one-time-code"
          disabled={busy}
        />
      </div>
      <div class="space-y-2">
        <Label for="pair-name">{m.pair_device_name_label()}</Label>
        <Input id="pair-name" type="text" bind:value={name} disabled={busy} />
      </div>
      <Button
        type="submit"
        class="w-full"
        disabled={busy || !code.trim() || !canPair}
      >
        <IconFingerprint class="size-4" aria-hidden="true" />
        {busy ? m.pair_binding() : m.pair_submit_button()}
      </Button>
      <Button
        type="button"
        variant="ghost"
        class="w-full"
        disabled={busy}
        onclick={onback}
      >
        {m.pair_back_to_login()}
      </Button>
    </form>
  </div>
</AuthGateFrame>
