<script lang="ts">
  import { type Snippet } from "svelte";
  import { navigate } from "../../platform/api";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { m } from "$lib/paraglide/messages.js";
  import IconArrowsExchange from "~icons/tabler/arrows-exchange";
  import IconGitFork from "~icons/tabler/git-fork";
  let {
    sourceId,
    children,
  }: {
    sourceId: string | undefined;
    children: Snippet<[Snippet | undefined]>;
  } = $props();
</script>

{@render children(sourceId ? menu : undefined)}

{#snippet menu()}
  <DropdownMenu.Item
    onSelect={() => navigate(`/handoff/${sourceId}?target=switch`)}
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
