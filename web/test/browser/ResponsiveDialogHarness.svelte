<script lang="ts">
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";

  let {
    busy = false,
    pinDrawerHeader = false,
    withFooter = false,
    onclose,
  }: {
    busy?: boolean;
    pinDrawerHeader?: boolean;
    withFooter?: boolean;
    onclose: () => void;
  } = $props();
</script>

{#snippet footerSlot()}
  <button type="button">底部确认</button>
{/snippet}

<ResponsiveDialog
  title="编辑工作区"
  description="桌面说明"
  drawerDescription="手机说明"
  {busy}
  {pinDrawerHeader}
  footer={withFooter ? footerSlot : undefined}
  {onclose}
>
  <button type="button">表单操作</button>
</ResponsiveDialog>
