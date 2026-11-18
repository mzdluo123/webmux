<script lang="ts">
  import IconGitBranch from "~icons/tabler/git-branch";

  let {
    title,
    location,
    locationId,
    locationIsBranch = false,
    archived = false,
  }: {
    title: string;
    location?: string;
    locationId?: string;
    locationIsBranch?: boolean;
    archived?: boolean;
  } = $props();
</script>

{#if location}
  <span class="min-w-0 flex-1">
    <span
      data-sidebar-session-text
      class={["block truncate", archived && "text-muted-foreground"]}
      >{title}</span
    >
    <span
      class={[
        "text-xs font-normal text-muted-foreground",
        locationIsBranch ? "flex min-w-0 items-center gap-1" : "block truncate",
      ]}
      data-slot="sidebar-session-location"
      id={locationId}
    >
      {#if locationIsBranch}
        <IconGitBranch class="size-3 shrink-0" aria-hidden="true" />
      {/if}
      <span
        data-sidebar-session-text
        class="min-w-0 truncate"
        title={locationIsBranch ? undefined : location}>{location}</span
      >
    </span>
  </span>
{:else}
  <span class={["min-w-0 flex-1 truncate", archived && "text-muted-foreground"]}
    >{title}</span
  >
{/if}
