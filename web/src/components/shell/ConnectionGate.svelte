<script lang="ts">
  import Brand from "./Brand.svelte";
  import TokenHelp from "../shared/TokenHelp.svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as Card from "$lib/components/ui/card";
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

<main class="flex min-h-dvh items-center justify-center p-6">
  <Card.Root class="w-full max-w-md">
    <Card.Header>
      <p class="brand mb-6"><Brand /></p>
      <Card.Title class="text-2xl">你的终端，始终在线。</Card.Title>
      <Card.Description>在可信内网中连接这台机器。</Card.Description>
    </Card.Header>
    <Card.Content>
      {#if checking}
        <p role="status" class="text-muted-foreground">正在连接…</p>
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
              <Label for="token">共享 Token</Label>
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
          <Button type="submit" class="w-full">连接 WebMux →</Button>
        </form>
      {/if}
    </Card.Content>
  </Card.Root>
</main>
