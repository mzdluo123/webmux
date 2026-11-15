<script lang="ts">
  import StripNotice from "../shared/StripNotice.svelte";
  import { untrack, type Snippet } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import { copyText } from "../../platform/clipboard";
  import { errorMessage } from "../../platform/error";
  import {
    addForward,
    forwardLink,
    forwards,
    ignorePort,
    isIgnored,
    isPromptedPort,
    pruneIgnored,
  } from "../../domains/workspace/forwards.svelte";
  import type { ListeningPort } from "../../contracts/schemas";

  // Strips for the ports this session listens on that are neither forwarded
  // nor ignored: at most two, then one summary that opens the dialog.
  let {
    session,
    listening,
    onshowall,
  }: {
    /** The session ID the ports belong to (the source of new forwards). */
    session: string;
    /**
     * The session's current listening ports; undefined while they are not
     * known (before the session info arrives, or off the terminal side).
     */
    listening: readonly ListeningPort[] | undefined;
    /** Opens the port-forwarding dialog. */
    onshowall?: () => void;
  } = $props();

  // Forwards made with "转发", announced until "知道了"; in memory only.
  let announced = $state.raw<{ port: number; path: string }[]>([]);
  let failed = $state<Record<number, string>>({});
  let copied = $state<number | null>(null);
  const busy = new SvelteSet<number>();

  // A port that stops listening leaves the ignore set, so a restarted server
  // prompts again. Unknown ports prune nothing: a remount must not forget
  // what the user ignored before the session info arrives.
  $effect(() => {
    const ports = listening;
    if (ports) untrack(() => pruneIgnored(session, ports));
  });

  const forwarded = $derived(new Set(forwards.list.map((entry) => entry.port)));
  // Until the forward list is known, any port might be forwarded already.
  const prompts = $derived(
    forwards.ready && listening
      ? listening
          .filter(
            (entry) =>
              isPromptedPort(entry.port) &&
              !forwarded.has(entry.port) &&
              !isIgnored(session, entry.port),
          )
          .sort((a, b) => a.port - b.port)
      : [],
  );

  function forward(entry: ListeningPort, open: boolean) {
    // A tab opened after the request returns is no longer inside the click,
    // and browsers block it; open a blank one now and point it there later.
    const tab = open ? window.open("", "_blank") : null;
    void create(entry, open, tab);
  }
  async function create(
    entry: ListeningPort,
    open: boolean,
    tab: Window | null,
  ) {
    if (busy.has(entry.port)) {
      tab?.close();
      return;
    }
    busy.add(entry.port);
    delete failed[entry.port];
    try {
      const created = await addForward({
        port: entry.port,
        session,
        process: entry.process,
      });
      if (open && tab) {
        tab.opener = null;
        // Absolute: the blank tab has no address of its own to resolve from.
        tab.location.href = forwardLink(created.path);
      } else {
        // Also after a blocked pop-up, so the forward can still be opened.
        announced = [
          ...announced.filter((item) => item.port !== created.port),
          { port: created.port, path: created.path },
        ];
      }
    } catch (err) {
      tab?.close();
      failed[entry.port] = m.s4_forward_failed({ error: errorMessage(err) });
    } finally {
      busy.delete(entry.port);
    }
  }
  async function copy(port: number, path: string) {
    copied = (await copyText(forwardLink(path))) ? port : null;
  }
</script>

{#snippet strip(text: string, actions: Snippet, error = "")}
  <StripNotice {error} {actions}>{text}</StripNotice>
{/snippet}

{#each announced as item (item.port)}
  {#snippet done()}
    <Button
      href={item.path}
      target="_blank"
      rel="noopener"
      variant="outline"
      size="sm"
      class="any-pointer-coarse:min-h-11">{m.s4_open()}</Button
    >
    <Button
      variant="ghost"
      size="sm"
      class="any-pointer-coarse:min-h-11"
      onclick={() => void copy(item.port, item.path)}
      >{copied === item.port ? m.s4_copied() : m.s4_copy_link()}</Button
    >
    <Button
      variant="ghost"
      size="sm"
      class="any-pointer-coarse:min-h-11"
      onclick={() =>
        (announced = announced.filter((entry) => entry.port !== item.port))}
      >{m.s4_got_it()}</Button
    >
  {/snippet}
  {@render strip(
    m.s4_port_forwarded_to({ port: String(item.port), path: item.path }),
    done,
  )}
{/each}

{#each prompts.slice(0, 2) as entry (entry.port)}
  {#snippet choose()}
    <Button
      variant="outline"
      size="sm"
      class="any-pointer-coarse:min-h-11"
      disabled={busy.has(entry.port)}
      onclick={() => forward(entry, true)}>{m.s4_forward_and_open()}</Button
    >
    <Button
      variant="ghost"
      size="sm"
      class="any-pointer-coarse:min-h-11"
      disabled={busy.has(entry.port)}
      onclick={() => forward(entry, false)}>{m.s4_forward()}</Button
    >
    <Button
      variant="ghost"
      size="sm"
      class="any-pointer-coarse:min-h-11"
      onclick={() => ignorePort(session, entry.port)}>{m.s4_ignore()}</Button
    >
  {/snippet}
  {@render strip(
    entry.process
      ? m.s4_port_listening_with_proc({
          port: String(entry.port),
          process: entry.process,
        })
      : m.s4_port_listening({ port: String(entry.port) }),
    choose,
    failed[entry.port] ?? "",
  )}
{/each}

{#if prompts.length > 2}
  {#snippet more()}
    {#if onshowall}<Button
        variant="ghost"
        size="sm"
        class="any-pointer-coarse:min-h-11"
        onclick={() => onshowall()}>{m.s4_view_all()}</Button
      >{/if}
  {/snippet}
  {@render strip(m.s4_more_ports({ count: String(prompts.length - 2) }), more)}
{/if}
