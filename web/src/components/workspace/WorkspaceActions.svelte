<script lang="ts">
  import type { Snippet } from "svelte";
  import IconDots from "~icons/tabler/dots";
  import IconRefresh from "~icons/tabler/refresh";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import SessionViewControls from "../sessions/SessionViewControls.svelte";
  import GitWorkbenchButton from "./GitWorkbenchButton.svelte";

  // The icon controls of the home page: list density, a Git refresh, the Git
  // workbench entry and the repository menu. Phones show them in the top bar,
  // wider screens in the toolbar, so the title and these controls never take
  // two rows. The entry is present from first render only when its combined
  // workspace/code capability is available.
  let {
    compact,
    refreshing,
    onchange,
    onrefresh,
    ongit,
    menu,
  }: {
    compact: boolean;
    refreshing: boolean;
    onchange: (compact: boolean) => void;
    onrefresh: () => void;
    ongit?: () => void;
    // The "⋯" menu of the repository in scope; a disabled button stands in
    // while none is.
    menu?: Snippet;
  } = $props();
</script>

<SessionViewControls {compact} {onchange} />
<Button
  variant="ghost"
  size="icon-sm"
  class="min-h-9 min-w-9 sm:min-h-8 sm:min-w-8"
  aria-label={m.s8_refresh_git_status()}
  title={refreshing ? m.s8_refreshing_git_status() : m.s8_refresh_git_status()}
  aria-busy={refreshing}
  onclick={() => onrefresh()}
  ><IconRefresh
    class={refreshing ? "animate-spin motion-reduce:animate-none" : ""}
    aria-hidden="true"
  /></Button
>
{#if ongit}<GitWorkbenchButton {ongit} />{/if}
{#if menu}{@render menu()}{:else}
  <!-- Keeps the toolbar's width while repositories are still loading. -->
  <Button
    variant="ghost"
    size="icon-sm"
    class="min-h-9 min-w-9 sm:min-h-8 sm:min-w-8"
    aria-label={m.s8_repo_menu()}
    title={m.s8_repo_menu()}
    disabled><IconDots aria-hidden="true" /></Button
  >
{/if}
