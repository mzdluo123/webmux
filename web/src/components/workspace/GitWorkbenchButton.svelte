<script lang="ts">
  import IconGitBranch from "~icons/tabler/git-branch";
  import { Button } from "$lib/components/ui/button";

  // The standalone "Git 工作台" entry used by the home toolbar and the
  // repository panel header: icon-only on phones, icon + label on wider
  // screens.
  let { ongit }: { ongit: () => void } = $props();
</script>

<Button
  variant="outline"
  size="sm"
  class="max-sm:size-(--control-default)"
  aria-label="Git 工作台"
  title="Git 工作台"
  onclick={ongit}
  ><IconGitBranch aria-hidden="true" /><span class="hidden sm:inline"
    >Git 工作台</span
  ></Button
>
