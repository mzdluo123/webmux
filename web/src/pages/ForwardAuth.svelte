<script lang="ts">
  import { onMount } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { forwardTicket } from "../api";
  import { errorMessage } from "../error";
  import { forwardAuthNext } from "../route";

  // The gateway sends an unauthenticated tab here. With WebMux signed in,
  // the page trades a one-time ticket for the gateway cookie and returns to
  // the forwarded address; App shows the token form first otherwise.
  let error = $state("");

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

  onMount(() => {
    const next = forwardAuthNext(location.search);
    if (!next) {
      error = "转发地址无效。";
      return;
    }
    let disposed = false;
    forwardTicket()
      .then((ticket) => {
        if (!disposed) redeem(ticket.code, next);
      })
      .catch((err: unknown) => {
        if (!disposed) error = `无法打开转发：${errorMessage(err)}`;
      });
    return () => {
      disposed = true;
    };
  });
</script>

<main
  class="flex min-h-dvh flex-col items-center justify-center gap-4 p-6 text-center"
>
  {#if error}
    <p role="alert" class="max-w-md text-destructive wrap-anywhere">{error}</p>
    <Button href="/" variant="outline">返回 WebMux</Button>
  {:else}
    <p role="status" class="text-muted-foreground">正在打开转发…</p>
  {/if}
</main>
