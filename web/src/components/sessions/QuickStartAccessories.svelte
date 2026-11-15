<script lang="ts">
  import type { ButtonSize, ButtonVariant } from "$lib/components/ui/button";
  import type { QuickCommand } from "../../contracts/schemas";
  import type { QuickStartFeatures } from "../../features/quick-start";
  import { navigate } from "../../platform/api";
  let {
    features,
    voice,
    sending,
    size,
    variant,
    oninsert,
    onerror,
    run,
    onfocus,
  }: {
    features: QuickStartFeatures;
    voice: boolean;
    sending: boolean;
    size: ButtonSize;
    variant: ButtonVariant;
    oninsert: (text: string) => void;
    onerror: (message: string) => void;
    run: (command: QuickCommand) => void;
    onfocus: () => void;
  } = $props();
</script>

{#if features.voice && voice}
  <features.voice.default
    {size}
    {variant}
    disabled={sending}
    onresult={oninsert}
    {onerror}
  />
{/if}
{#if features.quickCommands}
  <features.quickCommands.Menu
    commands={features.quickCommands.quickCommands.list}
    target="chat"
    unavailable=""
    disabled={sending}
    side="top"
    {size}
    {variant}
    {run}
    manage={() => navigate("/settings#quick-commands")}
    onCloseAutoFocus={(event) => {
      event.preventDefault();
      onfocus();
    }}
  />
{/if}
