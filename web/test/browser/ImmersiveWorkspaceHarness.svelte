<script lang="ts">
  import { Button } from "../../src/lib/components/ui/button";
  import { Textarea } from "../../src/lib/components/ui/textarea";
  import TerminalWorkspace from "../../src/components/terminal/TerminalWorkspace.svelte";
  let immersive = $state(false);
  let text = $state("");
</script>

<Button onclick={() => (immersive = !immersive)}
  >{immersive ? "退出 VR 布局" : "进入 VR 布局"}</Button
>
<div style="width:800px;height:480px">
  <TerminalWorkspace codeShown desktop={false} draftOpen={false} {immersive}>
    {#snippet terminalPane()}<Textarea
        aria-label="聊天输入"
        bind:value={text}
      />{/snippet}
    {#snippet draftPane()}<div>终端草稿</div>{/snippet}
    {#snippet codePane()}<div>代码阅读</div>{/snippet}
  </TerminalWorkspace>
</div>
