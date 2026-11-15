<script lang="ts">
  import { untrack } from "svelte";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import { api } from "../../platform/api";
  import {
    ChatSupportSchema,
    type ChatSupport,
    type LaunchConfig,
  } from "../../contracts/schemas";
  import {
    connectorNoticeText,
    chatSupportReason,
  } from "../../domains/sessions/hybrid";
  import ConnectorInstallHint from "../sessions/ConnectorInstallHint.svelte";

  let {
    config,
    cmd,
    hybrid = $bindable(false),
    disabled = false,
  }: {
    config: LaunchConfig | null;
    cmd: string[] | undefined;
    hybrid: boolean;
    disabled?: boolean;
  } = $props();
  let support = $state<ChatSupport | null>(null);
  let supportFor = "";
  const reason = $derived(chatSupportReason(support));
  const supportCmd = $derived(cmd ? JSON.stringify(cmd) : "");

  $effect.pre(() => {
    support = config?.chatSupport ?? null;
    supportFor = config ? JSON.stringify(config.cmd) : "";
  });
  $effect(() => {
    if (!supportCmd || supportCmd === supportFor) return;
    const asked = supportCmd;
    const command = untrack(() => cmd);
    if (!command) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      void api(
        "/launch-configs/chat-support",
        ChatSupportSchema,
        "POST",
        { cmd: command },
        controller.signal,
      )
        .then((result) => {
          support = result;
          supportFor = asked;
        })
        .catch(() => {
          // Saving remains authoritative when the support probe is unavailable.
        });
    }, 300);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  });
</script>

<div class="space-y-2">
  <div class="flex items-center gap-2">
    <Checkbox
      id="launch-config-hybrid"
      bind:checked={hybrid}
      disabled={disabled || (!hybrid && !support?.supported)}
      aria-describedby="launch-config-hybrid-help"
    />
    <Label for="launch-config-hybrid">混合模式（聊天 ↔ 终端）</Label>
  </div>
  <p id="launch-config-hybrid-help" class="text-xs text-muted-foreground">
    {#if reason}{reason}{:else}从这个配置启动的会话可以在网页聊天视图与终端之间切换{support
        ? `，适用于 ${support.supportedAgents.join("、")}`
        : ""}。{/if}
  </p>
  {#if support?.supported && support.connector}
    <ConnectorInstallHint
      connector={support.connector}
      message={connectorNoticeText(support.connector)}
      tone="info"
    />
  {/if}
</div>
