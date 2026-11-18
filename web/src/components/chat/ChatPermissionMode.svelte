<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import IconShield from "~icons/tabler/shield";
  import IconShieldCheck from "~icons/tabler/shield-check";
  import {
    changedPermissionMode,
    permissionChoices,
  } from "../../domains/chat/chat-permission";
  import type { ChatPermissionMode } from "../../contracts/schemas";
  import ChatSelect from "./ChatSelect.svelte";

  // WebMux's permission mode beside the agent's options: ask shows every
  // permission request, auto has the server approve it. Forms always ask.
  let {
    mode,
    disabled,
    preventScroll,
    onchange,
  }: {
    mode: ChatPermissionMode;
    disabled: boolean;
    preventScroll?: boolean;
    onchange: (mode: ChatPermissionMode) => void;
  } = $props();
</script>

<ChatSelect
  name={m.s3_permission_name()}
  {preventScroll}
  value={mode}
  choices={permissionChoices}
  {disabled}
  labeled
  title={mode === "auto"
    ? m.s3_permission_auto_title()
    : m.s3_permission_ask_title()}
  onchange={(value) => {
    const changed = changedPermissionMode(value, mode);
    if (changed) onchange(changed);
  }}
>
  {#snippet icon()}
    {#if mode === "auto"}
      <IconShieldCheck aria-hidden="true" />
    {:else}
      <IconShield aria-hidden="true" />
    {/if}
  {/snippet}
</ChatSelect>
