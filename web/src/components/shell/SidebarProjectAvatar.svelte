<script lang="ts">
  import IconFolder from "~icons/tabler/folder";
  import { repositoryRemotes } from "../../platform/api";

  let {
    name,
    repositoryId,
  }: {
    name: string;
    repositoryId?: string;
  } = $props();

  const initials = $derived(
    Array.from(name.toUpperCase()).slice(0, 2).join(""),
  );
  let src = $state("");
  let loaded = $state(false);

  $effect(() => {
    src = "";
    loaded = false;
    if (!repositoryId) return;
    const controller = new AbortController();
    void repositoryRemotes(repositoryId, controller.signal)
      .then(({ remotes }) => {
        if (controller.signal.aborted) return;
        const origin = remotes.find((remote) => remote.name === "origin");
        // Resolve only GitHub's public owner avatar. The repository path and
        // any user info or credentials in the remote stay in WebMux.
        const owner = origin?.fetchUrl.match(
          /^(?:https?:\/\/(?:[^/@]+@)?github\.com\/|ssh:\/\/git@github\.com(?::22)?\/|git@github\.com:)([a-z\d][a-z\d-]{0,38})\/[\w.-]+\/?$/i,
        )?.[1];
        if (owner) src = `https://avatars.githubusercontent.com/${owner}?s=48`;
      })
      .catch(() => {
        // Missing or unavailable remotes retain the initials.
      });
    return () => controller.abort();
  });
</script>

<span
  class="relative grid size-6 shrink-0 place-items-center overflow-hidden rounded-md bg-muted text-xs font-semibold text-muted-foreground"
  aria-hidden="true"
>
  {#if !repositoryId}
    <IconFolder class="size-3.5" />
  {:else if !loaded}
    {initials}
  {/if}
  {#if src}
    <img
      {src}
      alt=""
      width="24"
      height="24"
      crossorigin="anonymous"
      referrerpolicy="no-referrer"
      class={[
        "absolute inset-0 size-full object-cover",
        !loaded && "invisible",
      ]}
      onload={() => (loaded = true)}
      onerror={() => (src = "")}
    />
  {/if}
</span>
