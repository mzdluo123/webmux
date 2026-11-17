<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import type { PushCheck } from "../../contracts/schemas";
  import { InlineNotice } from "$lib/components/ui/inline-notice";

  // Whether the commit a removal or release keeps is contained in a
  // remote-tracking ref. The server reads local refs only, so an unpushed
  // commit warns without blocking, and a missing remote or a failed check
  // stays silent.
  let { push }: { push: PushCheck } = $props();
  const branches = $derived.by(() => {
    const joined = push.branches.join(m.s2_list_sep());
    return push.more
      ? m.s2_push_check_branches_more({
          branches: joined,
          total: String(push.more + push.branches.length),
        })
      : joined;
  });
</script>

{#if push.state === "unpushed"}
  <InlineNotice tone="warning">
    {m.s2_push_check_unpushed()}
  </InlineNotice>
{:else if push.state === "pushed"}
  <p class="text-xs text-muted-foreground">
    {m.s2_push_check_pushed({ branches })}
  </p>
{/if}
