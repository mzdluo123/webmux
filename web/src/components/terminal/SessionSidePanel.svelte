<script lang="ts">
  import { tick, type Snippet } from "svelte";
  import IconCode from "~icons/tabler/code";
  import IconLayoutColumns from "~icons/tabler/layout-columns";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconX from "~icons/tabler/x";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";

  let {
    view = $bindable("code"),
    shown,
    codeAvailable = true,
    close,
    paneActions,
    children,
  }: {
    view?: "picker" | "code" | "split";
    shown: boolean;
    codeAvailable?: boolean;
    close: () => void;
    paneActions?: Snippet<[() => void]>;
    children: Snippet;
  } = $props();
  let chooser = $state<HTMLDivElement>();
  let panel = $state<HTMLElement>();

  function focusPicker() {
    chooser?.querySelector<HTMLButtonElement>("button")?.focus();
  }

  export function openPicker() {
    view = "picker";
    void tick().then(focusPicker);
  }

  function choose(next: "code" | "split") {
    view = next;
    void tick().then(() => {
      if (!shown) return;
      if (next === "code") {
        const target =
          panel?.querySelector<HTMLElement>(".code-panel button") ?? panel;
        target?.focus();
      } else {
        panel
          ?.querySelector<HTMLButtonElement>('[aria-label="返回面板选择"]')
          ?.focus();
      }
    });
  }

  function shortcut(event: KeyboardEvent) {
    if (event.ctrlKey || event.metaKey || event.altKey || event.isComposing)
      return;
    const key = event.key.toLowerCase();
    if (key === "c" && codeAvailable) choose("code");
    else if (key === "s" && paneActions) choose("split");
    else return;
    event.preventDefault();
    event.stopPropagation();
  }
</script>

<section
  bind:this={panel}
  class="flex h-full min-h-0 min-w-0 flex-col bg-card outline-none"
  aria-label="侧栏面板"
  tabindex="-1"
>
  {#if view !== "code"}
    <header class="flex h-10 shrink-0 items-center gap-2 border-b px-2">
      {#if view === "split"}
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="返回面板选择"
          title="返回面板选择"
          onclick={openPicker}><IconArrowLeft aria-hidden="true" /></Button
        >
        <h2 class="min-w-0 flex-1 text-sm font-medium">分屏</h2>
      {:else}<span class="flex-1"></span>{/if}
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label="关闭侧栏面板"
        title="关闭侧栏面板"
        onclick={close}><IconX aria-hidden="true" /></Button
      >
    </header>
  {/if}
  <div class="relative flex min-h-0 flex-1 flex-col overflow-y-auto">
    {#if view === "picker"}
      <div
        bind:this={chooser}
        class="mx-auto flex w-full max-w-md flex-1 flex-col justify-center px-4 py-8 sm:px-6"
        role="group"
        aria-label="面板选择"
      >
        <h2 class="mb-6 text-center text-base font-medium">打开面板</h2>
        {#if codeAvailable}
          <ListButton
            variant="action-sheet"
            class="min-h-14"
            onclick={() => choose("code")}
            onkeydown={shortcut}
          >
            <IconCode aria-hidden="true" /><span class="flex-1">代码</span>
            <kbd
              class="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted font-mono text-sm text-muted-foreground"
              aria-hidden="true">C</kbd
            >
          </ListButton>
        {/if}
        {#if paneActions}
          <ListButton
            variant="action-sheet"
            class="min-h-14"
            onclick={() => choose("split")}
            onkeydown={shortcut}
          >
            <IconLayoutColumns aria-hidden="true" /><span class="flex-1"
              >分屏</span
            >
            <kbd
              class="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted font-mono text-sm text-muted-foreground"
              aria-hidden="true">S</kbd
            >
          </ListButton>
        {/if}
      </div>
    {/if}
    {#if view === "split"}<div
        class="min-h-0 flex-1 overflow-y-auto p-4"
        data-split-actions
      >
        {#if paneActions}{@render paneActions(close)}{/if}
      </div>{/if}
    <div class={view === "code" ? "min-h-0 flex-1" : "hidden"}>
      {@render children()}
    </div>
  </div>
</section>
