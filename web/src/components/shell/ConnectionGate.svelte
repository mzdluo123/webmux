<script lang="ts">
  import AuthGateFrame from "./AuthGateFrame.svelte";
  import TokenHelp from "../shared/TokenHelp.svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import { m } from "$lib/paraglide/messages.js";
  let {
    mode = "auth",
    checking,
    token = $bindable(""),
    error,
    onconnect,
    onretry,
    onusetoken,
  }: {
    mode?: "auth" | "offline";
    checking: boolean;
    token: string;
    error: string;
    onconnect: () => void;
    onretry?: () => void;
    onusetoken?: () => void;
  } = $props();
</script>

{#if mode === "offline"}
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
      {#if onusetoken}
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
