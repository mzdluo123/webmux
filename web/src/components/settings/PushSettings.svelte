<script lang="ts">
  import { onMount } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import * as Card from "$lib/components/ui/card";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { APIError } from "../../platform/api";
  import {
    attentionToasts,
    setToastNotificationsEnabled,
  } from "../../domains/notifications/attention-toast.svelte";
  import { PushReasonSchema, type PushStatus } from "../../contracts/schemas";
  import {
    advise,
    currentSubscription,
    describeTest,
    detectSupport,
    disable,
    enable,
    pushStatus,
    requestPermission,
    savedSubscriptionId,
    sendTest,
    sync,
    type PushAdvice,
    type PushSupport,
  } from "../../platform/push";

  // Both are injectable so component tests need no service worker or server.
  let {
    support = detectSupport(),
    loadStatus = pushStatus,
  }: {
    support?: PushSupport;
    loadStatus?: () => Promise<PushStatus>;
  } = $props();

  let status = $state<PushStatus>();
  let loadError = $state("");
  let subscribed = $state(false);
  let busy = $state(false);
  // The state a toggle in flight is heading for; the switch shows it until the
  // attempt settles and then shows the real subscription again.
  let target = $state(false);
  let notice = $state("");
  let error = $state("");
  // True while the browser's notification permission prompt is unanswered.
  let awaitingPermission = $state(false);
  // A server refusal at registration time overrides the status reason.
  let refused = $state<PushAdvice>();
  const advice = $derived<PushAdvice | undefined>(
    refused ?? (status ? advise(support, status) : undefined),
  );

  async function refresh() {
    try {
      status = await loadStatus();
      loadError = "";
    } catch (cause) {
      loadError = cause instanceof Error ? cause.message : String(cause);
    }
    subscribed =
      support.serviceWorker &&
      !!(await currentSubscription().catch(() => null)) &&
      !!savedSubscriptionId();
  }

  function failure(cause: unknown) {
    if (cause instanceof APIError && cause.status === 409) {
      const reason = PushReasonSchema.safeParse(cause.reason);
      if (reason.success) refused = reason.data;
    }
    error = cause instanceof Error ? cause.message : String(cause);
  }

  async function toggle(checked: boolean) {
    target = checked;
    busy = true;
    notice = error = "";
    try {
      if (checked) {
        if (!status?.publicKey) throw new Error(m.s2_push_err_no_pubkey());
        if (Notification.permission === "default") {
          awaitingPermission = true;
          try {
            await requestPermission();
          } finally {
            awaitingPermission = false;
          }
        }
        await enable(status.publicKey);
      } else {
        await disable();
      }
    } catch (cause) {
      failure(cause);
    } finally {
      await refresh();
      busy = false;
    }
  }

  async function test() {
    busy = true;
    notice = error = "";
    try {
      const result = describeTest(await sendTest());
      if (result.failed) error = result.message;
      else notice = result.message;
    } catch (cause) {
      failure(cause);
    } finally {
      await refresh();
      busy = false;
    }
  }

  onMount(() => {
    void (async () => {
      if (support.secure && support.serviceWorker) {
        const result = await sync().catch(() => "none" as const);
        if (result === "key-changed") error = m.s2_push_err_key_changed();
      }
      await refresh();
    })();
  });
</script>

<Card.Root flat id="push" class="scroll-mt-4">
  <Card.Header>
    <Card.Title>{m.s2_push_title()}</Card.Title>
    <Card.Description>{m.s2_push_desc()}</Card.Description>
  </Card.Header>
  <Card.Content class="space-y-4">
    <div class="space-y-1.5">
      <div class="flex items-center justify-between gap-4">
        <Label for="toast-enabled">{m.s2_push_toast_label()}</Label>
        <Switch
          id="toast-enabled"
          aria-describedby="toast-enabled-help"
          checked={attentionToasts.enabled}
          onCheckedChange={(checked) => setToastNotificationsEnabled(checked)}
        />
      </div>
      <p id="toast-enabled-help" class="text-xs text-muted-foreground">
        {m.s2_push_toast_help()}
      </p>
    </div>

    {#if loadError}
      <InlineNotice tone="danger"
        >{m.s2_push_err_load({ error: loadError })}</InlineNotice
      >
    {:else if status}
      <div class="flex flex-wrap items-center gap-2 text-sm" role="status">
        {#if subscribed}
          <Badge>{m.s2_push_badge_on()}</Badge>
        {:else if advice === "ready"}
          <Badge variant="secondary">{m.s2_push_badge_off()}</Badge>
        {:else}
          <Badge variant="outline">{m.s2_push_badge_unavailable()}</Badge>
        {/if}
        <span class="text-muted-foreground"
          >{m.s2_push_server_subs({
            count: String(status.subscriptions),
          })}</span
        >
      </div>
    {/if}

    {#if advice === "ready"}
      <div class="flex items-center justify-between gap-4">
        <Label for="push-enabled">{m.s2_push_enable_label()}</Label>
        <Switch
          id="push-enabled"
          bind:checked={
            () => (busy ? target : subscribed), (value) => void toggle(value)
          }
          disabled={busy}
        />
      </div>
      {#if awaitingPermission}
        <InlineNotice>
          <div class="space-y-1">
            <p class="font-medium">{m.s2_push_await_perm_title()}</p>
            <p>{m.s2_push_await_perm_desc()}</p>
          </div>
        </InlineNotice>
      {/if}
    {:else if advice}
      <InlineNotice tone="warning">
        {#if advice === "insecure"}
          <p class="font-medium">{m.s2_push_insecure_title()}</p>
          <p>{m.s2_push_insecure_desc()}</p>
          <a class="underline underline-offset-2" href="#access"
            >{m.s2_view_public_access_config()}</a
          >
        {:else if advice === "ios-home-screen"}
          <p class="font-medium">{m.s2_push_ios_title()}</p>
          <p>{m.s2_push_ios_req()}</p>
          <ol class="mt-1 list-decimal pl-5">
            <li>{m.s2_ios_step_share()}</li>
            <li>{m.s2_push_ios_step_open()}</li>
            <li>{m.s2_push_ios_step_enable()}</li>
          </ol>
        {:else if advice === "unsupported"}
          <p class="font-medium">{m.s2_push_unsupported_title()}</p>
          <p>{m.s2_push_unsupported_desc()}</p>
        {:else if advice === "quick_tunnel"}
          <p class="font-medium">{m.s2_push_quick_tunnel_title()}</p>
          <p>{m.s2_push_quick_tunnel_p1()}</p>
          <p>{m.s2_push_quick_tunnel_p2()}</p>
          <a class="underline underline-offset-2" href="/settings/tunnel"
            >{m.s2_view_public_access_config()}</a
          >
        {:else if advice === "unsupported_origin"}
          <p class="font-medium">{m.s2_push_origin_title()}</p>
          <p>{m.s2_push_origin_desc()}</p>
          <a class="underline underline-offset-2" href="/settings/tunnel"
            >{m.s2_view_public_access_config()}</a
          >
        {:else if advice === "disabled"}
          <p class="font-medium">{m.s2_push_disabled_title()}</p>
          <p>
            {m.s2_push_disabled_1()}<code>[push] enabled = false</code
            >{m.s2_push_disabled_2()}<code>true</code>{m.s2_push_disabled_3()}
          </p>
        {:else if advice === "storage"}
          <p class="font-medium">{m.s2_push_storage_title()}</p>
          <p>
            {m.s2_push_storage_1()}<code>push.json</code>{m.s2_push_storage_2()}
          </p>
        {:else if advice === "denied"}
          <p class="font-medium">{m.s2_push_denied_title()}</p>
          <p>{m.s2_push_denied_desc()}</p>
        {/if}
      </InlineNotice>
      {#if subscribed}
        <Button variant="outline" disabled={busy} onclick={() => toggle(false)}
          >{m.s2_push_disable_device()}</Button
        >
      {/if}
    {/if}

    <!-- The test reaches every device, so this browser can trigger it for the
         phones even when it has not turned push on itself. -->
    {#if status && status.subscriptions > 0}
      <Button variant="outline" disabled={busy} onclick={test}
        >{m.s2_push_send_test({ count: String(status.subscriptions) })}</Button
      >
    {/if}

    {#if error}
      <InlineNotice tone="danger">{error}</InlineNotice>
    {:else if notice}
      <InlineNotice tone="success">{notice}</InlineNotice>
    {/if}

    <div class="space-y-1 text-xs text-muted-foreground">
      <p class="font-medium">{m.s2_push_best_effort_title()}</p>
      <ul class="list-disc space-y-1 pl-5">
        <li>{m.s2_push_note_1()}</li>
        <li>{m.s2_push_note_2()}</li>
        <li>{m.s2_push_note_3()}</li>
        <li>{m.s2_push_note_4()}</li>
      </ul>
    </div>
  </Card.Content>
</Card.Root>
