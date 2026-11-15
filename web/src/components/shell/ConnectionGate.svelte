<script lang="ts">
  import Brand from "./Brand.svelte";
  import LocaleChoice from "../shared/LocaleChoice.svelte";
  import TokenHelp from "../shared/TokenHelp.svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as Card from "$lib/components/ui/card";
  import * as m from "$lib/paraglide/messages.js";
  let {
    checking,
    token = $bindable(""),
    error,
    onconnect,
  }: {
    checking: boolean;
    token: string;
    error: string;
    onconnect: () => void;
  } = $props();
</script>

<main class="flex min-h-dvh flex-col items-center justify-center gap-4 p-6">
  <Card.Root class="w-full max-w-md">
    <Card.Header>
      <p class="brand mb-6"><Brand /></p>
      <Card.Title class="text-2xl">{m.s1_gate_headline()}</Card.Title>
      <Card.Description>{m.s1_gate_subtitle()}</Card.Description>
    </Card.Header>
    <Card.Content>
      {#if checking}
        <p role="status" class="text-muted-foreground">
          {m.s1_connecting_ellipsis()}
        </p>
      {:else}
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
              autocomplete="current-password"
            />
          </div>
          {#if error}<p role="alert" class="text-destructive">{error}</p>{/if}
          <Button type="submit" class="w-full">{m.s1_connect_webmux()}</Button>
        </form>
      {/if}
    </Card.Content>
  </Card.Root>
  <LocaleChoice />
</main>
