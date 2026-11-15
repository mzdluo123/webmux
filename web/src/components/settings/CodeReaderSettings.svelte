<script lang="ts">
  import {
    settings,
    changeSettings,
  } from "../../domains/settings/settings.svelte";
  import { defaultCodeReader } from "../../domains/settings/settings-intent";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import * as Card from "$lib/components/ui/card";

  const code = $derived(settings.values.codeReader ?? defaultCodeReader);
</script>

<Card.Root flat id="code-reader" class="scroll-mt-4">
  <Card.Header>
    <Card.Title>代码阅读</Card.Title>
    <Card.Description
      >文件与 Diff 的阅读偏好保存在服务端，在已连接的设备间同步。</Card.Description
    >
  </Card.Header>
  <Card.Content>
    <fieldset disabled={!settings.ready} class="space-y-6">
      <div class="flex items-center justify-between gap-3">
        <Label for="settings-lines">代码行号</Label>
        <Switch
          id="settings-lines"
          checked={code.lineNumbers === "show"}
          onCheckedChange={(value) =>
            changeSettings({
              codeReader: { ...code, lineNumbers: value ? "show" : "hide" },
            })}
        />
      </div>
      <div class="flex items-center justify-between gap-3">
        <Label for="settings-wrap">Diff 自动换行</Label>
        <Switch
          id="settings-wrap"
          checked={code.diffOverflow === "wrap"}
          onCheckedChange={(value) =>
            changeSettings({
              codeReader: { ...code, diffOverflow: value ? "wrap" : "scroll" },
            })}
        />
      </div>
      <Button
        variant="outline"
        size="sm"
        onclick={() => changeSettings({ codeReader: null })}
        >恢复默认阅读设置</Button
      >
    </fieldset>
  </Card.Content>
</Card.Root>
