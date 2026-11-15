<script lang="ts">
  import { onMount } from "svelte";
  import { Button } from "../../src/lib/components/ui/button";
  import { Textarea } from "../../src/lib/components/ui/textarea";
  import * as Dialog from "../../src/lib/components/ui/dialog";
  import { tvMode } from "../../src/hooks/shared/tv-mode.svelte";
  import { useTvNavigation } from "../../src/hooks/shared/tv-navigation.svelte";
  let open = $state(true);
  let text = $state("");
  let submitted = $state("");
  tvMode.enabled = true;
  useTvNavigation(() => false);
  onMount(() => () => {
    tvMode.enabled = false;
  });
</script>

<main>
  <Button>表单外导航</Button>
  <output aria-label="提交结果">{submitted}</output>
  <Dialog.Root bind:open>
    <Dialog.Content>
      <Dialog.Header>
        <Dialog.Title>多行填写</Dialog.Title>
        <Dialog.Description>保留换行并完成遥控器导航。</Dialog.Description>
      </Dialog.Header>
      <form
        class="grid gap-4"
        onsubmit={(event) => {
          event.preventDefault();
          submitted = text;
          open = false;
        }}
      >
        <Textarea aria-label="多行内容" bind:value={text} />
        <Button type="submit">保存表单</Button>
      </form>
    </Dialog.Content>
  </Dialog.Root>
</main>
