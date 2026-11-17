<script lang="ts">
  import AuthGateFrame from "./AuthGateFrame.svelte";
  import TokenHelp from "../shared/TokenHelp.svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import { m } from "$lib/paraglide/messages.js";
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

<AuthGateFrame title={m.s1_gate_headline()} subtitle={m.s1_gate_subtitle()}>
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
</AuthGateFrame>
