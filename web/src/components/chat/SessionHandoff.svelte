<script lang="ts">
  import { tick, type Snippet } from "svelte";
  import { navigate } from "../../platform/api";
  import AgentHandoff from "./AgentHandoff.svelte";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { m } from "$lib/paraglide/messages.js";
  import IconArrowsExchange from "~icons/tabler/arrows-exchange";
  import IconGitFork from "~icons/tabler/git-fork";
  let {
    sourceId,
    root,
    children,
  }: {
    sourceId: string | undefined;
    root: HTMLElement | undefined;
    children: Snippet<[Snippet | undefined]>;
  } = $props();
  let open = $state(false);
</script>

{@render children(sourceId ? menu : undefined)}

{#snippet menu()}
  <DropdownMenu.Item onSelect={() => (open = true)}
    ><IconArrowsExchange
      aria-hidden="true"
    />{m.s3_menu_switch_agent()}</DropdownMenu.Item
  >
  <DropdownMenu.Item
    onSelect={() => {
      if (sourceId) navigate(`/handoff/${sourceId}`);
    }}
    ><IconGitFork
      aria-hidden="true"
    />{m.s3_menu_handoff_new_chat()}</DropdownMenu.Item
  >
{/snippet}

{#if open && sourceId}
  {#key sourceId}<AgentHandoff
      {sourceId}
      onclose={() => {
        open = false;
        void tick().then(() =>
          root
            ?.querySelector<HTMLButtonElement>(
              `[aria-label="${m.s3_session_menu()}"], [aria-label="${m.s3_terminal_menu()}"], [aria-label="Session menu" i], [aria-label="Terminal menu" i]`,
            )
            ?.focus(),
        );
      }}
    />{/key}
{/if}
