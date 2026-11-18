<script lang="ts">
  let {
    title,
    location,
    locationId,
    archived = false,
  }: {
    title: string;
    location?: string;
    locationId?: string;
    archived?: boolean;
  } = $props();
</script>

{#if location}
  <span class="min-w-0 flex-1">
    <span class={["block truncate", archived && "text-muted-foreground"]}
      >{title}</span
    >
    <span
      class="block truncate text-xs font-normal text-muted-foreground"
      data-slot="sidebar-session-location"
      id={locationId}
      title={location}>{location}</span
    >
  </span>
{:else}
  <span class={["min-w-0 flex-1 truncate", archived && "text-muted-foreground"]}
    >{title}</span
  >
{/if}
