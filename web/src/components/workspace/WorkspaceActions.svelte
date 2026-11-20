<script lang="ts">
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconRefresh from "~icons/tabler/refresh";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";

  let {
    refreshing,
    onrefresh,
    ongit,
  }: {
    refreshing: boolean;
    onrefresh: () => void;
    ongit?: () => void;
  } = $props();
</script>

{#if ongit}
  <Button
    variant="ghost"
    size="sm-touch"
    aria-label={m.s7_git_workbench()}
    title={m.s7_git_workbench()}
    onclick={ongit}
  >
    <IconGitBranch aria-hidden="true" /> Git
  </Button>
{/if}
<!-- When the expanded desktop sidebar already exposes workspace refresh,
     avoid showing a second identical refresh button on the same screen. -->
<Button
  variant="quiet"
  size="icon-touch-sm"
  class="lg:group-data-[collapsed=false]/shell:hidden"
  aria-label={m.s8_refresh_git_status()}
  title={refreshing ? m.s8_refreshing_git_status() : m.s8_refresh_git_status()}
  aria-busy={refreshing}
  onclick={onrefresh}
  ><IconRefresh
    class={refreshing ? "animate-spin motion-reduce:animate-none" : ""}
    aria-hidden="true"
  /></Button
>
