<script lang="ts">
  import type { Remote } from "../../contracts/schemas";
  import { createGitRemotesController } from "../../hooks/workspace/git-remotes-controller.svelte";
  import GitRemotesDialogs from "./GitRemotesDialogs.svelte";
  import GitRemotesSection from "./GitRemotesSection.svelte";

  // The repository panel's remotes. The server redacts credentials, so the
  // URLs shown and copied never hold a password or token.
  let {
    repositoryId = "",
    remotes,
    error,
    onretry,
    onRemotesUpdated,
  }: {
    repositoryId?: string;
    /** Undefined while the first read runs. */
    remotes: readonly Remote[] | undefined;
    error: string;
    onretry: () => void;
    onRemotesUpdated?: (next: Remote[]) => void;
  } = $props();

  const controller = createGitRemotesController({
    getRepoId: () => repositoryId,
    getRemotes: () => remotes ?? [],
    onRemotesUpdated: (next) => {
      onRemotesUpdated?.(next);
    },
  });
</script>

<GitRemotesSection
  {remotes}
  controller={repositoryId ? controller : undefined}
  {error}
  {onretry}
/>

{#if repositoryId}
  <GitRemotesDialogs remotes={controller} />
{/if}
