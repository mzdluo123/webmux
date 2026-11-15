<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import IconFolderOff from "~icons/tabler/folder-off";
  import { Badge } from "$lib/components/ui/badge";
  import type { HybridRelease } from "../../contracts/schemas";

  // Marks an archived record whose worktree was released to free disk space;
  // hovering or a screen reader gives the original path and saved commit.
  let { released }: { released: HybridRelease } = $props();
  const detail = $derived(
    m.s8_released_badge_detail({
      path: released.path,
      commit: released.commit.slice(0, 12),
    }),
  );
</script>

<Badge variant="secondary" title={detail}
  ><IconFolderOff aria-hidden="true" />{m.s8_released_badge_label()}<span
    class="sr-only">{m.s8_parens_value({ value: detail })}</span
  ></Badge
>
