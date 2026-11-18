<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import IconExternalLink from "~icons/tabler/external-link";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import * as Card from "$lib/components/ui/card";
  import { voicePreference } from "../../hooks/shared/voice-preference.svelte";
  import { useVoiceSettings } from "../../hooks/settings/voice-settings.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import VoiceConfiguration from "./VoiceConfiguration.svelte";

  const protocolDoc = $derived(m.s2_voice_protocol_doc_url());

  const voice = useVoiceSettings();
  const view = $derived(voice.state.view);
</script>

<Card.Root flat id="voice" class="scroll-mt-4">
  <Card.Header>
    <Card.Title>{m.s2_voice_title()}</Card.Title>
    <Card.Description>{m.s2_voice_desc()}</Card.Description>
  </Card.Header>
  <Card.Content class="space-y-4">
    <div class="flex flex-wrap items-center gap-2 text-sm" role="status">
      <span class="text-muted-foreground">{m.s2_voice_recognizer_label()}</span>
      {#if !view}
        <Badge variant="outline">{m.s2_voice_badge_loading()}</Badge>
      {:else if view.enabled}
        <Badge variant="success">{m.s2_voice_badge_configured()}</Badge>
      {:else}
        <Badge variant="outline">{m.s2_voice_badge_unconfigured()}</Badge>
      {/if}
      {#if view}<span class="text-xs text-muted-foreground"
          >{view.source === "web"
            ? m.s2_voice_from_web()
            : m.s2_voice_from_config()}</span
        >{/if}
    </div>
    <div class="flex items-center justify-between gap-4">
      <Label for="voice-enabled">{m.s2_voice_enable_device()}</Label>
      <Switch
        id="voice-enabled"
        checked={voicePreference.enabled}
        onCheckedChange={(value) => (voicePreference.enabled = value)}
        disabled={!view?.enabled}
      />
    </div>
    {#if view && !view.storageAvailable}
      <InlineNotice tone="warning"
        >{m.s2_voice_storage_warn({
          error: view.storageError ?? "",
        })}</InlineNotice
      >
    {/if}
    {#if view}
      <VoiceConfiguration
        current={view}
        disabled={voice.state.busy || !view.storageAvailable}
        onsave={(draft) => void voice.save(draft)}
        onreset={() => void voice.reset()}
      />
    {/if}
    {#if voice.state.error}
      <InlineNotice tone="danger">{voice.state.error}</InlineNotice>
    {/if}
    {#if voice.state.notice}
      <InlineNotice tone="success">{voice.state.notice}</InlineNotice>
    {/if}
    <Button
      variant="outline"
      href={protocolDoc}
      target="_blank"
      rel="noreferrer"
      >{m.s2_voice_protocol_doc()}<IconExternalLink
        aria-hidden="true"
      /></Button
    >
  </Card.Content>
</Card.Root>
