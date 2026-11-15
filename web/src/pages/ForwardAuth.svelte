<script lang="ts">
  import { onMount } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import { forwardTicket } from "../platform/api";
  import { errorMessage } from "../platform/error";
  import { forwardAuthNext } from "../hooks/shared/route";

  // The gateway sends an unauthenticated tab here. With WebMux signed in,
  // the page trades a one-time ticket for the gateway cookie and returns to
  // the forwarded address; App shows the token form first otherwise. The
  // server adds confirm=1 when another site started the load: the page then
  // waits for a click, so a foreign page cannot drive the forwarded app.
  let errorFn = $state<(() => string) | null>(null);
  const error = $derived(errorFn?.() ?? "");
  let pending = $state("");
  let busy = $state(false);

  // A top-level form POST carries the code in its body, so it never enters
  // an address bar, history entry or access log.
  function redeem(code: string, next: string) {
    // WebMux responses carry `Referrer-Policy: no-referrer`, under which the
    // browser sends `Origin: null` with a form POST and the exchange refuses
    // it. This page's own same-origin request keeps its Origin.
    const policy = document.createElement("meta");
    policy.name = "referrer";
    policy.content = "same-origin";
    document.head.append(policy);
    const form = document.createElement("form");
    form.method = "POST";
    form.action = "/forward/_auth";
    form.hidden = true;
    for (const [name, value] of [
      ["code", code],
      ["next", next],
    ] as const) {
      const field = document.createElement("input");
      field.type = "hidden";
      field.name = name;
      field.value = value;
      form.append(field);
    }
    document.body.append(form);
    form.submit();
  }

  function open(next: string) {
    busy = true;
    forwardTicket()
      .then((ticket) => redeem(ticket.code, next))
      .catch((err: unknown) => {
        busy = false;
        const cause = errorMessage(err);
        errorFn = () => m.s1_forward_open_failed({ error: cause });
      });
  }

  onMount(() => {
    const next = forwardAuthNext(location.search);
    if (!next) {
      errorFn = () => m.s1_forward_invalid_url();
      return;
    }
    if (new URLSearchParams(location.search).get("confirm") === "1") {
      pending = next;
      return;
    }
    open(next);
  });
</script>

<main
  class="flex min-h-dvh flex-col items-center justify-center gap-4 p-6 text-center"
>
  {#if error}
    <p role="alert" class="max-w-md text-destructive wrap-anywhere">{error}</p>
    <Button href="/" variant="outline">{m.s1_back_to_webmux()}</Button>
  {:else if pending}
    <p class="max-w-md wrap-anywhere">
      {m.s1_forward_confirm_before()}
      <code class="font-mono">{pending}</code>{m.s1_forward_confirm_after()}
    </p>
    <div class="flex gap-2">
      <Button disabled={busy} onclick={() => open(pending)}
        >{m.s1_kbd_open()}</Button
      >
      <Button href="/" variant="outline">{m.s1_back_to_webmux()}</Button>
    </div>
  {:else}
    <p role="status" class="text-muted-foreground">{m.s1_opening_forward()}</p>
  {/if}
</main>
