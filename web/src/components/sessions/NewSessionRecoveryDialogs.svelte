<script lang="ts">
  import type { Worktree } from "../../contracts/schemas";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import LazyDialog from "../shared/LazyDialog.svelte";
  let {
    workspaceEnabled,
    filesEnabled,
    removing,
    onchange,
    onremoved,
    onremovalclose,
    mkdirOpen = $bindable(false),
    mkdirMounted,
    cwd,
    onmkdir,
  }: {
    workspaceEnabled: boolean;
    filesEnabled: boolean;
    removing: Worktree | null;
    onchange: () => void;
    onremoved: (tree: Worktree) => void;
    onremovalclose: () => void;
    mkdirOpen: boolean;
    mkdirMounted: boolean;
    cwd: string;
    onmkdir: () => void;
  } = $props();
</script>

{#if workspaceEnabled && removing}
  {@const target = removing}
  <LazyDialog
    load={() => import("../workspace/RemoveWorktreeDialog.svelte")}
    label="删除确认"
    onclose={onremovalclose}
  >
    {#snippet children(page)}<page.default
        worktree={target}
        {onchange}
        onremoved={() => onremoved(target)}
        onclose={onremovalclose}
      />{/snippet}
  </LazyDialog>
{/if}

{#if filesEnabled && mkdirMounted}<AlertDialog.Root bind:open={mkdirOpen}>
    <AlertDialog.Content>
      <AlertDialog.Header
        ><AlertDialog.Title>目录不存在，是否创建？</AlertDialog.Title
        ><AlertDialog.Description class="break-all"
          >将在 {cwd} 创建目录，然后启动会话。</AlertDialog.Description
        ></AlertDialog.Header
      >
      <AlertDialog.Footer>
        <AlertDialog.Cancel>取消</AlertDialog.Cancel>
        <AlertDialog.Action onclick={onmkdir}>创建目录并启动</AlertDialog.Action
        >
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>{/if}
