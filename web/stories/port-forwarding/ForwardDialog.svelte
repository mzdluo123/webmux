<script lang="ts">
  import { untrack } from "svelte";
  import IconAdjustments from "~icons/tabler/adjustments";
  import IconCopy from "~icons/tabler/copy";
  import IconExternalLink from "~icons/tabler/external-link";
  import IconTrash from "~icons/tabler/trash";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import AdvancedSection from "../../src/components/AdvancedSection.svelte";
  import ResponsiveDialog from "../../src/components/ResponsiveDialog.svelte";
  import ForwardOptionsFields from "./ForwardOptionsFields.svelte";
  import {
    defaultOptions,
    forwardPath,
    portError,
    type DetectedPort,
    type Forward,
    type ForwardOptions,
  } from "./data";

  // Prototype of the home page "端口转发" dialog (openspec change
  // `port-forwarding`). State is local; adding, editing and removing only
  // change the sample list.
  let {
    forwards = $bindable(),
    detected,
    detection,
    editing = null,
    onclose,
  }: {
    forwards: Forward[];
    detected: DetectedPort[];
    detection: "supported" | "unsupported";
    editing?: number | null;
    onclose: () => void;
  } = $props();

  let port = $state("");
  let attempted = $state(false);
  let addOptions = $state<ForwardOptions>({ ...defaultOptions });
  const initial = untrack(() =>
    forwards.find((forward) => forward.port === editing),
  );
  let editingPort = $state<number | null>(initial?.port ?? null);
  let draftLabel = $state(initial?.label ?? "");
  let draftOptions = $state<ForwardOptions>({
    ...(initial?.options ?? defaultOptions),
  });
  let copied = $state<number | null>(null);

  const taken = $derived(forwards.map((forward) => forward.port));
  const error = $derived(portError(port, taken));
  const shownError = $derived(
    error || (attempted && !port.trim() ? "请输入端口号" : ""),
  );
  const pending = $derived(
    detected.filter((entry) => !taken.includes(entry.port)),
  );

  function toggleEdit(forward: Forward) {
    if (editingPort === forward.port) {
      editingPort = null;
      return;
    }
    editingPort = forward.port;
    draftLabel = forward.label;
    draftOptions = { ...forward.options };
  }
  function add(next: number, source?: DetectedPort) {
    const entry: Forward = {
      port: next,
      label: "",
      session: source?.session ?? "",
      process: source?.process ?? "",
      upstream: source ? "listening" : "unreachable",
      options: { ...(source ? defaultOptions : addOptions) },
    };
    forwards = [...forwards, entry].sort((a, b) => a.port - b.port);
  }
  function submit(event: SubmitEvent) {
    event.preventDefault();
    attempted = true;
    if (shownError) return;
    add(Number(port));
    port = "";
    attempted = false;
  }
  function save(target: number) {
    forwards = forwards.map((forward) =>
      forward.port === target
        ? { ...forward, label: draftLabel.trim(), options: { ...draftOptions } }
        : forward,
    );
    editingPort = null;
  }
  async function copy(target: number) {
    await navigator.clipboard
      ?.writeText(new URL(forwardPath(target), location.origin).href)
      .catch(() => {});
    copied = target;
  }
  function title(forward: Forward) {
    if (forward.label) return forward.label;
    if (forward.session) return `${forward.session} · ${forward.process}`;
    return "手动添加";
  }
</script>

<ResponsiveDialog
  title="端口转发"
  description="通过 WebMux 访问服务器本机端口。转发不会保存，服务重启后清空。"
  pinDrawerHeader
  {onclose}
>
  <div class="space-y-6">
    <form class="space-y-3" onsubmit={submit} novalidate>
      <div class="space-y-2">
        <Label for="forward-port">端口</Label>
        <div class="flex gap-2">
          <Input
            id="forward-port"
            class="max-w-40"
            inputmode="numeric"
            autocomplete="off"
            placeholder="例如 5173"
            bind:value={port}
            aria-invalid={shownError ? true : undefined}
            aria-describedby="forward-port-error"
          />
          <Button type="submit">添加转发</Button>
        </div>
        <p
          id="forward-port-error"
          class="min-h-4 text-xs text-destructive"
          role={shownError ? "alert" : undefined}
        >
          {shownError}
        </p>
      </div>
      <AdvancedSection title="兼容选项" summary="默认适合 Vite 等开发服务器">
        <ForwardOptionsFields
          id="forward-new"
          port={error || !port ? null : Number(port)}
          bind:options={addOptions}
        />
      </AdvancedSection>
    </form>

    <section class="space-y-2" aria-labelledby="forward-list-title">
      <h3 id="forward-list-title" class="text-sm font-medium">
        已转发
        <span class="text-muted-foreground tabular-nums">{forwards.length}</span
        >
      </h3>
      {#if forwards.length}
        <ul class="divide-y">
          {#each forwards as forward (forward.port)}
            {@const open = editingPort === forward.port}
            <li
              class="grid grid-cols-[3.5rem_minmax(0,1fr)] items-center gap-x-3 gap-y-1 py-2 sm:grid-cols-[3.5rem_minmax(0,1fr)_auto]"
            >
              <span class="font-mono text-sm font-medium tabular-nums"
                >{forward.port}</span
              >
              <div class="min-w-0">
                <p class="truncate text-sm" title={title(forward)}>
                  {title(forward)}
                </p>
                <p
                  class="flex min-w-0 items-center gap-2 text-xs text-muted-foreground"
                >
                  <span
                    class={`status-dot ${forward.upstream === "listening" ? "status-running" : "status-idle"}`}
                    aria-hidden="true"
                  ></span>
                  <span class="shrink-0"
                    >{forward.upstream === "listening"
                      ? "可访问"
                      : "无法连接"}</span
                  >
                  <span class="min-w-0 truncate font-mono"
                    >{forwardPath(forward.port)}</span
                  >
                </p>
              </div>
              <div class="col-start-2 flex items-center gap-1 sm:col-start-3">
                <Button
                  href={forwardPath(forward.port)}
                  target="_blank"
                  rel="noopener"
                  variant="ghost"
                  size="icon-touch"
                  aria-label={`在新标签页打开端口 ${forward.port}`}
                  title="在新标签页打开"
                  ><IconExternalLink aria-hidden="true" /></Button
                >
                <Button
                  variant="ghost"
                  size="icon-touch"
                  aria-label={`复制端口 ${forward.port} 的链接`}
                  title={copied === forward.port ? "已复制" : "复制链接"}
                  onclick={() => copy(forward.port)}
                  ><IconCopy aria-hidden="true" /></Button
                >
                <Button
                  variant="ghost"
                  size="icon-touch"
                  aria-label={`端口 ${forward.port} 的设置`}
                  title="设置"
                  aria-expanded={open}
                  onclick={() => toggleEdit(forward)}
                  ><IconAdjustments aria-hidden="true" /></Button
                >
                <Button
                  variant="ghost"
                  size="icon-touch"
                  aria-label={`移除端口 ${forward.port} 的转发`}
                  title="移除转发"
                  onclick={() =>
                    (forwards = forwards.filter(
                      (entry) => entry.port !== forward.port,
                    ))}><IconTrash aria-hidden="true" /></Button
                >
              </div>
              {#if open}
                <form
                  class="col-span-full mt-2 space-y-4 rounded-lg border p-4"
                  onsubmit={(event) => {
                    event.preventDefault();
                    save(forward.port);
                  }}
                >
                  <div class="space-y-2">
                    <Label for={`forward-${forward.port}-label`}
                      >名称（可选）</Label
                    >
                    <Input
                      id={`forward-${forward.port}-label`}
                      maxlength={64}
                      placeholder={forward.session || "例如 前端预览"}
                      bind:value={draftLabel}
                    />
                  </div>
                  <ForwardOptionsFields
                    id={`forward-${forward.port}`}
                    port={forward.port}
                    bind:options={draftOptions}
                  />
                  <div class="flex justify-end gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      onclick={() => (editingPort = null)}>取消</Button
                    >
                    <Button type="submit">保存</Button>
                  </div>
                </form>
              {/if}
            </li>
          {/each}
        </ul>
      {:else}
        <p class="text-sm text-muted-foreground">
          还没有转发。输入端口，或转发下方检测到的端口。
        </p>
      {/if}
    </section>

    <section class="space-y-2" aria-labelledby="forward-detected-title">
      <h3 id="forward-detected-title" class="text-sm font-medium">
        检测到的端口
      </h3>
      {#if detection === "unsupported"}
        <p class="text-sm text-muted-foreground">
          当前平台不支持自动检测，可以手动输入端口。
        </p>
      {:else if pending.length}
        <ul class="divide-y">
          {#each pending as entry (entry.port)}
            <li
              class="grid grid-cols-[3.5rem_minmax(0,1fr)_auto] items-center gap-3 py-2"
            >
              <span class="font-mono text-sm font-medium tabular-nums"
                >{entry.port}</span
              >
              <span class="min-w-0 truncate text-sm text-muted-foreground"
                >{entry.process} · 会话 {entry.session}</span
              >
              <Button
                variant="outline"
                size="sm-touch"
                onclick={() => add(entry.port, entry)}>转发</Button
              >
            </li>
          {/each}
        </ul>
      {:else}
        <p class="text-sm text-muted-foreground">
          会话中没有其他正在监听的端口。
        </p>
      {/if}
    </section>

    <p class="text-xs text-muted-foreground">
      转发页面与 WebMux 同源，页面脚本可以访问 WebMux
      登录状态。只转发你信任的开发服务器。
    </p>
  </div>
</ResponsiveDialog>
