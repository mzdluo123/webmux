<script lang="ts">
  import type { ComponentProps } from "svelte";
  import WorkbenchTree from "../../src/components/shell/WorkbenchTree.svelte";
  import { provideSidebarProjectAvatars } from "../../src/hooks/shared/sidebar-project-avatars";

  let props: ComponentProps<typeof WorkbenchTree> = $props();
  provideSidebarProjectAvatars();
</script>

<WorkbenchTree {...props} />
