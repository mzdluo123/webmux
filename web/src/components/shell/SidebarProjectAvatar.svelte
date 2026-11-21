<script lang="ts">
  import IconFolder from "~icons/tabler/folder";
  import { untrack } from "svelte";
  import { getSidebarProjectAvatars } from "../../hooks/shared/sidebar-project-avatars";

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
  const avatars = getSidebarProjectAvatars();
  const src = $derived(repositoryId ? avatars.src(repositoryId) : null);

  $effect(() => {
    const id = repositoryId;
    if (id) untrack(() => avatars.load(id));
  });
</script>

<span
  class="relative grid size-6 shrink-0 place-items-center overflow-hidden rounded-md bg-muted text-xs font-semibold tracking-tight text-muted-foreground ring-1 ring-inset ring-foreground/10"
  aria-hidden="true"
>
  {#if !repositoryId}
    <IconFolder class="size-3.5" />
  {:else if src}
    <img
      {src}
      alt=""
      width="24"
      height="24"
      class="absolute inset-0 size-full object-cover"
    />
  {:else}
    {initials}
  {/if}
</span>
