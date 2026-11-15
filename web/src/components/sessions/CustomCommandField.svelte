<script lang="ts">
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";

  let {
    command = $bindable(""),
    showLabel = false,
    disabled = false,
    oninput,
  }: {
    command: string;
    showLabel?: boolean;
    disabled?: boolean;
    oninput?: () => void;
  } = $props();
</script>

<div class="space-y-2">
  {#if showLabel}<Label for="custom-command">自定义启动命令</Label>{/if}
  <Input
    id="custom-command"
    aria-label="自定义启动命令"
    aria-describedby="custom-command-help"
    bind:value={command}
    {disabled}
    {oninput}
    mono
    placeholder="agent --resume 或 bash -lc 'npm run dev'"
    autocomplete="off"
    autocapitalize="off"
    autocorrect="off"
    spellcheck={false}
  />
  <p id="custom-command-help" class="text-xs text-muted-foreground">
    直接执行，不展开变量或管道；需要 shell 语法请用 bash -lc '命令'。
  </p>
</div>
