<script lang="ts">
  import type { ButtonSize, ButtonVariant } from "$lib/components/ui/button";
  import type { QuickCommand } from "../../contracts/schemas";
  import type { QuickStartPlugins } from "../../plugins/quick-start";
  import { navigate } from "../../platform/api";
  let {
    plugins,
    voice,
    sending,
    size,
    variant,
    oninsert,
    onerror,
    run,
    onfocus,
  }: {
    plugins: QuickStartPlugins;
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

{#if plugins.voice && voice}
  <plugins.voice.default
    {size}
    {variant}
    disabled={sending}
    onresult={oninsert}
    {onerror}
  />
{/if}
{#if plugins.quickCommands}
  <plugins.quickCommands.Menu
    commands={plugins.quickCommands.quickCommands.list}
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
