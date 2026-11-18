<script lang="ts">
  import { untrack } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
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
  let probe = $state<{ command: string; support: ChatSupport }>();
  const supportCmd = $derived(cmd ? JSON.stringify(cmd) : "");
  const savedCmd = $derived(config ? JSON.stringify(config.cmd) : "");
  const support = $derived(
    supportCmd && supportCmd === savedCmd
      ? (config?.chatSupport ?? null)
      : probe?.command === supportCmd
        ? probe.support
        : null,
  );
  const reason = $derived(chatSupportReason(support));

  $effect(() => {
    if (!supportCmd || supportCmd === savedCmd) return;
    const asked = supportCmd;
    if (untrack(() => probe?.command) === asked) return;
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
          if (!controller.signal.aborted && asked === supportCmd)
            probe = { command: asked, support: result };
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
    <Label for="launch-config-hybrid">{m.s2_hybrid_mode_label()}</Label>
  </div>
  <ScrollArea class="max-h-36">
    <p
      id="launch-config-hybrid-help"
      role="status"
      class="text-xs text-muted-foreground wrap-anywhere"
    >
      {#if reason}{reason}{:else}{support
          ? m.s2_hybrid_help_supported({
              agents: support.supportedAgents.join(m.s2_list_sep()),
            })
          : m.s2_hybrid_help()}{/if}
    </p>
    {#if support?.supported && support.connector}
      <ConnectorInstallHint
        connector={support.connector}
        message={connectorNoticeText(support.connector)}
        tone="info"
        class="mt-2"
      />
    {/if}
  </ScrollArea>
</div>
