<script lang="ts">
  import type { PendingUpdate } from "../../contracts/schemas";
  import {
    dismissUpdate,
    dismissedUpdate,
    homeUpdate,
  } from "../../platform/pending-update";
  import PendingUpdateNotice from "./PendingUpdateNotice.svelte";

  let { update, inset = false }: { update?: PendingUpdate; inset?: boolean } =
    $props();
  let dismissedVersion = $state(dismissedUpdate());
  const notice = $derived(homeUpdate(update, dismissedVersion));
</script>

{#if notice}
  {@const version = notice.installedVersion}
  <div
    class={inset ? "mx-auto max-w-7xl px-4 pt-5 sm:px-8 sm:pt-6" : undefined}
  >
    <PendingUpdateNotice
      update={notice}
      ondismiss={() => {
        dismissUpdate(version);
        dismissedVersion = version;
      }}
    />
  </div>
{/if}
