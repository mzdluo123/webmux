<script lang="ts">
  import { tick, untrack } from "svelte";
  import { SvelteMap, SvelteSet } from "svelte/reactivity";
  import { m } from "$lib/paraglide/messages.js";
  import { parseJson, type JsonNode } from "../../domains/code/code-json";
  import type { CodeJsonState } from "../../contracts/schemas";
  import { Button } from "$lib/components/ui/button";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import { paneEnter } from "../../hooks/shared/pane-enter";
  import IconChevronRight from "~icons/tabler/chevron-right";
  let {
    text,
    onsource,
    restore,
  }: { text: string; onsource: () => void; restore?: CodeJsonState } = $props();
  const result = $derived(parseJson(text));
  // Large collections reveal their members in batches.
  const batch = 200;
  // Per-collection state keyed by its path; the first two levels start open.
  const initial = untrack(() => restore);
  const opened = new SvelteMap<string, boolean>(initial?.opened);
  const shown = new SvelteMap<string, number>(initial?.shown);
  const manuallyOpened = new SvelteSet<string>();
  const isOpen = (key: string, depth: number) => opened.get(key) ?? depth < 2;
  export function capture(): CodeJsonState {
    return { opened: [...opened], shown: [...shown] };
  }
  function members(node: JsonNode) {
    return node.kind === "object"
      ? node.entries.map((entry) => ({ name: entry.key, value: entry.value }))
      : node.kind === "array"
        ? node.items.map((value, index) => ({ name: String(index), value }))
        : [];
  }
  function toggle(key: string, open: boolean) {
    opened.set(key, open);
    if (open) manuallyOpened.add(key);
    else manuallyOpened.delete(key);
  }
  function reveal(node: HTMLElement) {
    let attached = true;
    let animation: ReturnType<typeof paneEnter> | undefined;
    // The collapsible updates its hidden attribute in the same flush.
    void tick().then(() => {
      if (attached && !node.contains(node.ownerDocument.activeElement))
        animation = paneEnter(node, true, 128);
    });
    const settle = () => animation?.update(false);
    node.addEventListener("focusin", settle, { once: true });
    return () => {
      attached = false;
      animation?.destroy();
      node.removeEventListener("focusin", settle);
    };
  }
  async function showMore(button: HTMLElement, key: string, limit: number) {
    const last = button.previousElementSibling;
    const document = button.ownerDocument;
    const focused = document.activeElement === button;
    shown.set(key, limit + batch);
    await tick();
    if (
      !focused ||
      (document.activeElement !== button &&
        document.activeElement !== document.body)
    )
      return;
    const next = last?.nextElementSibling;
    if (!(next instanceof HTMLElement)) return;
    const row =
      next.querySelector<HTMLElement>('[data-slot="collapsible-trigger"]') ??
      next;
    row.focus({ preventScroll: true });
    row.scrollIntoView({ block: "nearest", inline: "nearest" });
  }
</script>

{#snippet tree(node: JsonNode, key: string, depth: number, name?: string)}
  {#if node.kind === "object" || node.kind === "array"}
    {@const all = members(node)}
    {@const open = isOpen(key, depth)}
    {@const limit = shown.get(key) ?? batch}
    <Collapsible.Root
      open={isOpen(key, depth)}
      onOpenChange={(value) => toggle(key, value)}
    >
      <Collapsible.Trigger
        class="flex items-center gap-1 rounded-sm text-left hover:bg-muted"
      >
        <IconChevronRight
          aria-hidden="true"
          class={open ? "size-3.5 shrink-0 rotate-90" : "size-3.5 shrink-0"}
        />
        {#if name !== undefined}<span class="text-muted-foreground">{name}</span
          >{/if}
        <span
          >{node.kind === "object" ? "{}" : "[]"}
          <span class="text-muted-foreground"
            >{m.s5_json_items_count({ count: String(all.length) })}</span
          ></span
        >
      </Collapsible.Trigger>
      <Collapsible.Content class="pl-4">
        <!-- Closed collections render nothing, so large documents stay light. -->
        {#if open}
          <div {@attach manuallyOpened.has(key) ? reveal : undefined}>
            {#each all.slice(0, limit) as member, index (index)}
              {@render tree(
                member.value,
                `${key}/${index}`,
                depth + 1,
                member.name,
              )}
            {/each}
            {#if all.length > limit}<Button
                variant="ghost"
                size="sm"
                onclick={(event) =>
                  void showMore(event.currentTarget, key, limit)}
                >{m.s5_json_show_more_items({ count: String(batch) })}</Button
              >{/if}
          </div>
        {/if}
      </Collapsible.Content>
    </Collapsible.Root>
  {:else}
    <p tabindex="-1" class="break-all whitespace-pre-wrap">
      {#if name !== undefined}<span class="text-muted-foreground">{name}:</span
        >{/if}
      {#if node.kind === "string"}<span class="text-primary"
          >"{node.value}"</span
        >{:else if node.kind === "number"}<span>{node.raw}</span
        >{:else if node.kind === "boolean"}<span>{node.value}</span>{:else}<span
          class="text-muted-foreground">null</span
        >{/if}
    </p>
  {/if}
{/snippet}

<div class="min-h-0 flex-1 overflow-auto p-2 font-mono text-xs">
  {#if result.ok}
    <section aria-label={m.s5_json_preview()}>
      {@render tree(result.value, "", 0)}
    </section>
  {:else}
    <div class="space-y-2 font-sans text-sm">
      <p class="text-destructive" role="alert">
        {m.s5_json_syntax_error_at({
          line: String(result.line),
          column: String(result.column),
        })}
      </p>
      <p class="text-muted-foreground">{result.detail}</p>
      <Button variant="outline" size="sm" onclick={onsource}
        >{m.s5_back_to_source()}</Button
      >
    </div>
  {/if}
</div>
