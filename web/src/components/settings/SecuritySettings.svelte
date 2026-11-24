<script lang="ts">
  import { onMount } from "svelte";
  import IconCopy from "~icons/tabler/copy";
  import IconDevices from "~icons/tabler/devices";
  import IconFingerprint from "~icons/tabler/fingerprint";
  import IconLogout from "~icons/tabler/logout";
  import IconPlus from "~icons/tabler/plus";
  import IconRefresh from "~icons/tabler/refresh";
  import IconTrash from "~icons/tabler/trash";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Card from "$lib/components/ui/card";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import { Separator } from "$lib/components/ui/separator";
  import type {
    AuthAPIToken,
    AuthPasskey,
    AuthSession,
    AuthStatus,
    PairCreateResponse,
  } from "../../contracts/auth-schemas";
  import {
    addPasskey,
    createPairingCode,
    deletePasskey,
    fetchAuthStatus,
    listAPITokens,
    listAuthSessions,
    listPasskeys,
    logoutCurrentSession,
    regenerateRecoveryCodes,
    revokeAuthSession,
    revokeOtherAuthSessions,
  } from "../../platform/auth-api";
  import { copyText } from "../../platform/clipboard";
  import { errorMessage } from "../../platform/error";
  import {
    defaultDeviceLabel,
    isWebAuthnSupported,
  } from "../../platform/webauthn";
  import { m } from "$lib/paraglide/messages.js";
  import ApiTokensSection from "./ApiTokensSection.svelte";

  let status = $state<AuthStatus | null>(null);
  let passkeys = $state<AuthPasskey[]>([]);
  let sessions = $state<AuthSession[]>([]);
  let apiTokens = $state<AuthAPIToken[]>([]);
  let loading = $state(true);
  let busy = $state(false);
  let error = $state("");

  let pairOpen = $state(false);
  let pairTargetUrl = $state("");
  let pairResult = $state<PairCreateResponse | null>(null);

  let newRecoveryCodes = $state<string[]>([]);

  async function refresh() {
    loading = true;
    error = "";
    try {
      const st = await fetchAuthStatus();
      status = st;
      if (st.state === "ready") {
        const [pkRes, sessRes, tokRes] = await Promise.all([
          listPasskeys(),
          listAuthSessions(),
          listAPITokens(),
        ]);
        passkeys = pkRes.passkeys;
        sessions = sessRes.sessions;
        apiTokens = tokRes.tokens;
      }
    } catch (err) {
      error = errorMessage(err);
    } finally {
      loading = false;
    }
  }

  async function handleAddPasskey() {
    busy = true;
    error = "";
    try {
      await addPasskey(defaultDeviceLabel());
      await refresh();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }

  async function handleDeletePasskey(id: string) {
    busy = true;
    error = "";
    try {
      await deletePasskey(id);
      await refresh();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }

  async function handleCreatePair() {
    busy = true;
    error = "";
    try {
      pairResult = await createPairingCode(pairTargetUrl.trim());
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }

  async function handleRevokeSession(id: string) {
    busy = true;
    error = "";
    try {
      await revokeAuthSession(id);
      await refresh();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }

  async function handleRevokeOtherSessions() {
    busy = true;
    error = "";
    try {
      await revokeOtherAuthSessions();
      await refresh();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }

  async function handleRegenerateRecovery() {
    busy = true;
    error = "";
    try {
      newRecoveryCodes = await regenerateRecoveryCodes();
      await refresh();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }

  function formatDate(iso?: string): string {
    if (!iso) return "";
    const parsed = new Date(iso);
    return Number.isNaN(parsed.getTime()) ? iso : parsed.toLocaleString();
  }

  onMount(() => {
    void refresh();
  });
</script>

<Card.Root flat id="security" class="scroll-mt-4">
  <Card.Header>
    <Card.Title>{m.settings_nav_security()}</Card.Title>
    <Card.Description>{m.settings_security_desc()}</Card.Description>
  </Card.Header>
  <Card.Content class="space-y-6">
    {#if error}
      <InlineNotice tone="danger">{error}</InlineNotice>
    {/if}

    {#if !loading && status && status.state !== "ready"}
      <InlineNotice tone="info">
        {m.settings_security_disabled_notice()}
      </InlineNotice>
    {:else if status?.state === "ready"}
      <!-- Passkeys -->
      <div class="space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h3 class="text-sm font-medium flex items-center gap-1.5">
            <IconFingerprint class="size-4" aria-hidden="true" />
            {m.settings_passkeys_heading()}
          </h3>
          <div class="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy || !isWebAuthnSupported() || !status.rpIdValid}
              onclick={() => void handleAddPasskey()}
            >
              <IconPlus class="size-4" aria-hidden="true" />
              {m.settings_passkey_add()}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              onclick={() => {
                pairOpen = !pairOpen;
                pairResult = null;
              }}
            >
              <IconDevices class="size-4" aria-hidden="true" />
              {m.settings_passkey_pair()}
            </Button>
          </div>
        </div>

        {#if pairOpen}
          <div class="rounded-lg border p-4 space-y-3 bg-muted/30">
            <div class="space-y-1">
              <p class="text-sm font-medium">
                {m.settings_pair_dialog_title()}
              </p>
              <p class="text-xs text-muted-foreground">
                {m.settings_pair_dialog_desc()}
              </p>
            </div>
            <div class="flex flex-col gap-2 sm:flex-row">
              <div class="flex-1 space-y-1">
                <Label for="pair-target-url">
                  {m.settings_pair_target_url_label()}
                </Label>
                <Input
                  id="pair-target-url"
                  type="url"
                  bind:value={pairTargetUrl}
                  placeholder={m.settings_pair_target_url_placeholder()}
                  disabled={busy}
                />
              </div>
              <Button
                type="button"
                size="sm"
                class="sm:self-end"
                disabled={busy}
                onclick={() => void handleCreatePair()}
              >
                {m.settings_pair_generate_button()}
              </Button>
            </div>
            {#if pairResult}
              <div
                class="rounded-md border bg-background p-3 space-y-2 text-sm"
              >
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <span class="font-mono text-base font-semibold select-all">
                    {pairResult.displayCode}
                  </span>
                  <div class="flex flex-wrap gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onclick={() => void copyText(pairResult!.displayCode)}
                    >
                      <IconCopy class="size-4" aria-hidden="true" />
                      {m.settings_pair_copy_code()}
                    </Button>
                    {#if pairResult.pairUrl}
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onclick={() => void copyText(pairResult!.pairUrl!)}
                      >
                        <IconCopy class="size-4" aria-hidden="true" />
                        {m.settings_pair_copy_link()}
                      </Button>
                    {/if}
                  </div>
                </div>
              </div>
            {/if}
          </div>
        {/if}

        <div class="divide-y rounded-lg border">
          {#each passkeys as pk (pk.id)}
            <div class="flex flex-wrap items-center justify-between gap-3 p-3">
              <div class="min-w-0 space-y-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="text-sm font-medium break-all">{pk.name}</span>
                  <Badge variant="outline">{pk.rpId}</Badge>
                  <Badge variant="secondary">
                    {pk.backupState
                      ? m.settings_passkey_synced()
                      : m.settings_passkey_device_bound()}
                  </Badge>
                </div>
                <p class="text-xs text-muted-foreground">
                  {pk.lastUsedAt
                    ? m.settings_passkey_last_used({
                        time: formatDate(pk.lastUsedAt),
                      })
                    : m.settings_passkey_never_used()}
                </p>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={busy || passkeys.length <= 1}
                onclick={() => void handleDeletePasskey(pk.id)}
              >
                <IconTrash class="size-4" aria-hidden="true" />
                {m.settings_passkey_delete()}
              </Button>
            </div>
          {/each}
        </div>
      </div>

      <Separator />

      <!-- Active Sessions -->
      <div class="space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h3 class="text-sm font-medium">{m.settings_sessions_heading()}</h3>
          <div class="flex flex-wrap gap-2">
            {#if sessions.length > 1}
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={busy}
                onclick={() => void handleRevokeOtherSessions()}
              >
                {m.settings_session_revoke_others()}
              </Button>
            {/if}
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              onclick={() => void logoutCurrentSession()}
            >
              <IconLogout class="size-4" aria-hidden="true" />
              {m.settings_session_logout()}
            </Button>
          </div>
        </div>
        <div class="divide-y rounded-lg border">
          {#each sessions as sess (sess.id)}
            <div class="flex flex-wrap items-center justify-between gap-3 p-3">
              <div class="min-w-0 space-y-0.5">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="text-sm font-medium">{sess.name}</span>
                  {#if sess.rpId}
                    <Badge variant="outline">{sess.rpId}</Badge>
                  {/if}
                  {#if sess.current}
                    <Badge variant="secondary">
                      {m.settings_session_current()}
                    </Badge>
                  {/if}
                </div>
                <p class="text-xs text-muted-foreground">
                  {formatDate(sess.lastUsedAt)}
                </p>
              </div>
              {#if !sess.current}
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  disabled={busy}
                  onclick={() => void handleRevokeSession(sess.id)}
                >
                  {m.settings_session_revoke()}
                </Button>
              {/if}
            </div>
          {/each}
        </div>
      </div>

      <Separator />

      <!-- Recovery Codes -->
      <div class="space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="space-y-0.5">
            <h3 class="text-sm font-medium">{m.settings_recovery_heading()}</h3>
            <p class="text-xs text-muted-foreground">
              {m.settings_recovery_remaining({
                count: String(status.recoveryCodesRemaining),
              })}
            </p>
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={busy}
            onclick={() => void handleRegenerateRecovery()}
          >
            <IconRefresh class="size-4" aria-hidden="true" />
            {m.settings_recovery_regenerate()}
          </Button>
        </div>
        {#if newRecoveryCodes.length > 0}
          <div class="space-y-2 rounded-lg border bg-muted/40 p-3">
            <div class="grid grid-cols-2 gap-2 font-mono text-sm">
              {#each newRecoveryCodes as code (code)}
                <span class="select-all text-center">{code}</span>
              {/each}
            </div>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onclick={() => void copyText(newRecoveryCodes.join("\n"))}
            >
              <IconCopy class="size-4" aria-hidden="true" />
              {m.onboarding_recovery_copy()}
            </Button>
          </div>
        {/if}
      </div>

      <Separator />

      <ApiTokensSection
        tokens={apiTokens}
        {busy}
        {formatDate}
        onRefresh={refresh}
        onError={(msg) => {
          error = msg;
        }}
      />
    {/if}
  </Card.Content>
</Card.Root>
