<script lang="ts">
  import { onMount } from "svelte";
  import IconCheck from "~icons/tabler/check";
  import IconCopy from "~icons/tabler/copy";
  import IconDownload from "~icons/tabler/download";
  import IconFingerprint from "~icons/tabler/fingerprint";
  import IconKey from "~icons/tabler/key";
  import IconShieldCheck from "~icons/tabler/shield-check";
  import AuthGateFrame from "../components/shell/AuthGateFrame.svelte";
  import PasskeyContextGuard from "../components/shell/PasskeyContextGuard.svelte";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import type { AuthStatus } from "../contracts/auth-schemas";
  import { ConfigSchema } from "../contracts/schemas";
  import { api } from "../platform/api";
  import {
    completeOnboarding,
    consumeAuthFragment,
    registerFirstPasskey,
    verifyOnboardingCode,
  } from "../platform/auth-api";
  import { copyText } from "../platform/clipboard";
  import { errorMessage } from "../platform/error";
  import {
    defaultDeviceLabel,
    isWebAuthnSupported,
    localhostRedirectUrl,
  } from "../platform/webauthn";
  import { m } from "$lib/paraglide/messages.js";

  let {
    authStatus,
    oncomplete,
  }: {
    authStatus: AuthStatus | null;
    oncomplete: () => void;
  } = $props();

  type Step = 1 | 2 | 3 | 4;
  let step = $state<Step>(1);
  let setupCode = $state("");
  let ticket = $state("");
  let passkeyName = $state(defaultDeviceLabel());
  let recoveryCodes = $state<string[]>([]);
  let copiedCodes = $state(false);
  let busy = $state(false);
  let error = $state("");
  let defaultCwd = $state("");

  const hostLabel = $derived(authStatus?.hostname || location.host);
  const rpId = $derived(authStatus?.rpId || location.hostname);
  const canCreatePasskey = $derived(
    isWebAuthnSupported() &&
      !localhostRedirectUrl() &&
      Boolean(authStatus?.rpIdValid),
  );

  async function verifyCode(raw: string) {
    const trimmed = raw.trim();
    if (!trimmed) return;
    busy = true;
    error = "";
    try {
      const res = await verifyOnboardingCode(trimmed);
      ticket = res.ticket;
      step = 2;
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }

  async function handleCreatePasskey() {
    if (!ticket) return;
    busy = true;
    error = "";
    try {
      const res = await registerFirstPasskey(
        ticket,
        passkeyName.trim() || defaultDeviceLabel(),
      );
      recoveryCodes = res.recoveryCodes;
      void loadWorkspaceSummary();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }

  async function loadWorkspaceSummary() {
    try {
      const cfg = await api("/config", ConfigSchema, "GET");
      defaultCwd = cfg.defaultCwd;
    } catch {
      defaultCwd = "";
    }
  }

  async function copyRecoveryCodes() {
    if (!recoveryCodes.length) return;
    try {
      await copyText(recoveryCodes.join("\n"));
      copiedCodes = true;
    } catch {
      copiedCodes = false;
    }
  }

  function downloadRecoveryCodes() {
    const text = [
      `WebMux Recovery Codes (${hostLabel})`,
      "Each code can be used once to sign in if you lose access to all Passkeys.",
      "",
      ...recoveryCodes,
      "",
    ].join("\n");
    const blob = new Blob([text], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "webmux-recovery-codes.txt";
    link.click();
    URL.revokeObjectURL(url);
  }

  async function finishOnboarding() {
    busy = true;
    error = "";
    try {
      await completeOnboarding();
      oncomplete();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    const fromHash = consumeAuthFragment("#setup=");
    if (fromHash) {
      setupCode = fromHash;
      void verifyCode(fromHash);
    }
  });
</script>

<AuthGateFrame
  title={m.onboarding_page_title()}
  subtitle={m.onboarding_page_subtitle({ host: hostLabel })}
>
  <div class="space-y-5">
    <div
      class="flex items-center justify-between gap-2 text-xs text-muted-foreground"
    >
      <span
        >{m.onboarding_step_label({ current: String(step), total: "4" })}</span
      >
      <div class="flex items-center gap-1">
        {#each [1, 2, 3, 4] as num (num)}
          <span
            class={num <= step
              ? "h-1.5 w-6 rounded-full bg-primary"
              : "h-1.5 w-6 rounded-full bg-muted"}
          ></span>
        {/each}
      </div>
    </div>

    <PasskeyContextGuard {authStatus} />
    {#if error}
      <InlineNotice tone="danger">{error}</InlineNotice>
    {/if}

    {#if step === 1}
      <form
        class="space-y-4"
        onsubmit={(event) => {
          event.preventDefault();
          void verifyCode(setupCode);
        }}
      >
        <p class="text-sm text-muted-foreground leading-relaxed">
          {m.onboarding_claim_desc()}
        </p>
        <div class="space-y-2">
          <Label for="setup-code">{m.onboarding_setup_code_label()}</Label>
          <Input
            id="setup-code"
            type="text"
            bind:value={setupCode}
            placeholder={m.onboarding_setup_code_placeholder()}
            autocomplete="one-time-code"
            disabled={busy}
          />
        </div>
        <Button
          type="submit"
          class="w-full"
          disabled={busy || !setupCode.trim()}
        >
          <IconKey class="size-4" aria-hidden="true" />
          {busy ? m.onboarding_verifying() : m.onboarding_verify_button()}
        </Button>
      </form>
    {:else if step === 2 && recoveryCodes.length === 0}
      <form
        class="space-y-4"
        onsubmit={(event) => {
          event.preventDefault();
          void handleCreatePasskey();
        }}
      >
        <div class="flex flex-wrap items-center gap-2 text-sm">
          <span class="text-muted-foreground">
            {m.onboarding_passkey_desc({ domain: rpId })}
          </span>
          <Badge variant="outline">{rpId}</Badge>
        </div>
        <div class="space-y-2">
          <Label for="passkey-name">{m.onboarding_passkey_name_label()}</Label>
          <Input id="passkey-name" bind:value={passkeyName} disabled={busy} />
        </div>
        <Button
          type="submit"
          class="w-full"
          disabled={busy || !canCreatePasskey}
        >
          <IconFingerprint class="size-4" aria-hidden="true" />
          {busy
            ? m.onboarding_passkey_creating()
            : m.onboarding_passkey_create_button()}
        </Button>
      </form>
    {:else if step === 2}
      <div class="space-y-4">
        <InlineNotice tone="info">
          <div class="space-y-1">
            <p class="font-medium flex items-center gap-1.5">
              <IconShieldCheck class="size-4 shrink-0" aria-hidden="true" />
              {m.onboarding_recovery_title()}
            </p>
            <p class="text-xs leading-relaxed">
              {m.onboarding_recovery_desc()}
            </p>
          </div>
        </InlineNotice>
        <div
          class="grid grid-cols-2 gap-2 rounded-lg border bg-muted/40 p-3 font-mono text-sm"
        >
          {#each recoveryCodes as code (code)}
            <span class="select-all text-center">{code}</span>
          {/each}
        </div>
        <div class="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            class="flex-1"
            onclick={() => void copyRecoveryCodes()}
          >
            {#if copiedCodes}
              <IconCheck class="size-4" aria-hidden="true" />
              {m.onboarding_recovery_copied()}
            {:else}
              <IconCopy class="size-4" aria-hidden="true" />
              {m.onboarding_recovery_copy()}
            {/if}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            class="flex-1"
            onclick={downloadRecoveryCodes}
          >
            <IconDownload class="size-4" aria-hidden="true" />
            {m.onboarding_recovery_download()}
          </Button>
        </div>
        {#if error}<p role="alert" class="text-sm text-destructive">
            {error}
          </p>{/if}
        {@render stepActions(3)}
      </div>
    {:else if step === 3}
      <div class="space-y-4">
        <div class="space-y-1">
          <h2 class="text-base font-medium">
            {m.onboarding_workspace_title()}
          </h2>
          <p class="text-sm text-muted-foreground leading-relaxed">
            {m.onboarding_workspace_desc()}
          </p>
        </div>
        {#if defaultCwd}
          <div class="rounded-lg border p-3 text-sm space-y-1">
            <p class="text-xs text-muted-foreground">
              {m.onboarding_default_cwd_label()}
            </p>
            <p class="font-mono break-all">{defaultCwd}</p>
          </div>
        {/if}
        {#if error}<p role="alert" class="text-sm text-destructive">
            {error}
          </p>{/if}
        {@render stepActions(4)}
      </div>
    {:else}
      <div class="space-y-4">
        <div class="space-y-1">
          <h2 class="text-base font-medium">{m.onboarding_remote_title()}</h2>
          <p class="text-sm text-muted-foreground leading-relaxed">
            {m.onboarding_remote_desc()}
          </p>
        </div>
        <InlineNotice tone="info">
          <p class="text-xs leading-relaxed">
            {m.onboarding_remote_settings_hint()}
          </p>
        </InlineNotice>
        {#if error}<p role="alert" class="text-sm text-destructive">
            {error}
          </p>{/if}
        <Button
          type="button"
          class="w-full"
          disabled={busy}
          onclick={() => void finishOnboarding()}
        >
          {m.onboarding_finish_button()}
        </Button>
      </div>
    {/if}
  </div>
</AuthGateFrame>

{#snippet stepActions(nextStep: Step)}
  <div class="space-y-2 pt-1">
    <Button
      type="button"
      class="w-full"
      disabled={busy}
      onclick={() => {
        step = nextStep;
      }}
    >
      {m.onboarding_continue()}
    </Button>
    <Button
      type="button"
      variant="ghost"
      class="w-full"
      disabled={busy}
      onclick={() => void finishOnboarding()}
    >
      {m.onboarding_skip_enter()}
    </Button>
  </div>
{/snippet}
