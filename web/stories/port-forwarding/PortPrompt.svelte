<script lang="ts">
  import { untrack } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import ForwardDialog from "./ForwardDialog.svelte";
  import { forwardPath, type DetectedPort, type Forward } from "./data";

  // Prototype of the terminal page port prompt (openspec change
  // `port-forwarding`): one strip per unforwarded, unignored listening port
  // of this session, at most two, then a summary strip.
  type Scenario = "new" | "multiple" | "forwarded" | "failed";
  let { scenario = "new" }: { scenario?: Scenario } = $props();

  const session = "web";
  const initial = untrack(() => scenario);
  const listening: DetectedPort[] =
    initial === "multiple"
      ? [3000, 5173, 6006, 9229].map((port) => ({
          port,
          process: "node",
          session,
        }))
      : [{ port: 5173, process: "node", session }];

  let forwards = $state<Forward[]>([]);
  let ignored = $state<number[]>([]);
  let announced = $state<number[]>(initial === "forwarded" ? [5173] : []);
  let failed = $state<Record<number, string>>(
    initial === "failed"
      ? { 5173: "转发失败：服务端无法添加转发（示例错误）" }
      : {},
  );
  let opened = $state("");
  let dialog = $state(false);

  const forwarded = $derived(
    new Set([...forwards.map((forward) => forward.port), ...announced]),
  );
  const prompts = $derived(
    listening.filter(
      (entry) => !forwarded.has(entry.port) && !ignored.includes(entry.port),
    ),
  );

  function forward(entry: DetectedPort, open: boolean) {
    forwards = [
      ...forwards,
      {
        port: entry.port,
        label: "",
        session,
        process: entry.process,
        upstream: "listening",
        options: {
          rewriteHost: true,
          rewriteOrigin: true,
          forwardedHeaders: true,
          rewriteResponse: true,
          prefix: "keep",
        },
      },
    ];
    delete failed[entry.port];
    if (open) opened = `原型：已在新标签页打开 ${forwardPath(entry.port)}`;
    else announced = [...announced, entry.port];
  }
</script>

{#snippet strip(text: string, actions: import("svelte").Snippet, error = "")}
  <div
    class="flex shrink-0 flex-wrap items-center gap-x-2 gap-y-1 border-b bg-muted px-4 py-1.5 text-sm"
    role="status"
  >
    <span class="min-w-0 flex-1 wrap-anywhere max-sm:basis-full">
      {text}
      {#if error}<span class="block text-xs text-destructive">{error}</span
        >{/if}
    </span>
    {@render actions()}
  </div>
{/snippet}

<div class="flex h-dvh flex-col bg-background">
  <div
    class="flex h-8 shrink-0 items-center gap-2 border-b bg-card px-3 text-sm"
  >
    <span class="status-dot status-running" aria-hidden="true"></span>
    <span class="min-w-0 truncate font-medium">web</span>
    <span class="truncate text-xs text-muted-foreground">~/projects/shop</span>
  </div>

  {#each announced as port (port)}
    {#snippet done()}
      <Button
        variant="outline"
        size="sm"
        class="any-pointer-coarse:min-h-11"
        onclick={() => (opened = `原型：已在新标签页打开 ${forwardPath(port)}`)}
        >打开</Button
      >
      <Button variant="ghost" size="sm" class="any-pointer-coarse:min-h-11"
        >复制链接</Button
      >
      <Button
        variant="ghost"
        size="sm"
        class="any-pointer-coarse:min-h-11"
        onclick={() => (announced = announced.filter((p) => p !== port))}
        >知道了</Button
      >
    {/snippet}
    {@render strip(`端口 ${port} 已转发到 ${forwardPath(port)}`, done)}
  {/each}

  {#each prompts.slice(0, 2) as entry (entry.port)}
    {#snippet choose()}
      <Button
        variant="outline"
        size="sm"
        class="any-pointer-coarse:min-h-11"
        onclick={() => forward(entry, true)}>转发并打开</Button
      >
      <Button
        variant="ghost"
        size="sm"
        class="any-pointer-coarse:min-h-11"
        onclick={() => forward(entry, false)}>转发</Button
      >
      <Button
        variant="ghost"
        size="sm"
        class="any-pointer-coarse:min-h-11"
        onclick={() => (ignored = [...ignored, entry.port])}>忽略</Button
      >
    {/snippet}
    {@render strip(
      `检测到端口 ${entry.port}（${entry.process}）正在监听`,
      choose,
      failed[entry.port] ?? "",
    )}
  {/each}

  {#if prompts.length > 2}
    {#snippet more()}
      <Button
        variant="ghost"
        size="sm"
        class="any-pointer-coarse:min-h-11"
        onclick={() => (dialog = true)}>查看全部</Button
      >
    {/snippet}
    {@render strip(`另有 ${prompts.length - 2} 个端口`, more)}
  {/if}

  {#if opened}
    {#snippet ok()}
      <Button
        variant="ghost"
        size="sm"
        class="any-pointer-coarse:min-h-11"
        onclick={() => (opened = "")}>知道了</Button
      >
    {/snippet}
    {@render strip(opened, ok)}
  {/if}

  <pre
    class="min-h-0 flex-1 overflow-auto p-3 font-mono text-xs leading-5">$ pnpm dev --base /forward/5173/

  VITE v6.2.0  ready in 412 ms

  ➜  Local:   http://localhost:5173/forward/5173/
  ➜  Network: use --host to expose
  ➜  press h + enter to show help</pre>
</div>

{#if dialog}
  <ForwardDialog
    bind:forwards
    detected={listening}
    detection="supported"
    onclose={() => (dialog = false)}
  />
{/if}
