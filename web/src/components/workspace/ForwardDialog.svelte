<script lang="ts">
  import { SvelteSet } from "svelte/reactivity";
  import * as m from "$lib/paraglide/messages.js";
  import IconAdjustments from "~icons/tabler/adjustments";
  import IconCopy from "~icons/tabler/copy";
  import IconExternalLink from "~icons/tabler/external-link";
  import IconPlugConnected from "~icons/tabler/plug-connected";
  import IconTrash from "~icons/tabler/trash";
  import { Button } from "$lib/components/ui/button";
  import * as Empty from "$lib/components/ui/empty";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import AdvancedSection from "../shared/AdvancedSection.svelte";
  import ForwardOptionsFields from "./ForwardOptionsFields.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import { copyText } from "../../platform/clipboard";
  import { errorMessage } from "../../platform/error";
  import {
    addForward,
    changeForward,
    defaultForwardOptions,
    forwardLink,
    forwards,
    isPromptedPort,
    removeForward,
  } from "../../domains/workspace/forwards.svelte";
  import type {
    Forward,
    ForwardOptions,
    SessionInfo,
  } from "../../contracts/schemas";

  // The "端口转发" dialog: add a forward, manage the server's forwards and
  // forward ports the sessions listen on. The list itself comes from the
  // live forwards store; every change goes to the server.
  let {
    sessions,
    detection,
    onclose,
  }: {
    /** Sessions whose listening ports are offered for forwarding. */
    sessions: readonly SessionInfo[];
    /** Whether the server detects listening ports on its platform. */
    detection: boolean;
    onclose: () => void;
  } = $props();

  let port = $state("");
  let attempted = $state(false);
  let adding = $state(false);
  // The server's answer to the last submit; cleared as the port is edited.
  let addError = $state("");
  let addOptions = $state<ForwardOptions>({ ...defaultForwardOptions });
  let editingPort = $state<number | null>(null);
  let draftLabel = $state("");
  let draftOptions = $state<ForwardOptions>({ ...defaultForwardOptions });
  let copied = $state<number | null>(null);
  // Failures of row actions, keyed like `busy`: `forward:5173`, `detected:…`.
  let rowErrors = $state<Record<string, string>>({});
  const busy = new SvelteSet<string>();

  const taken = $derived(new Set(forwards.list.map((entry) => entry.port)));
  const inputError = $derived.by(() => {
    const text = port.trim();
    if (!text) return attempted ? m.s8_fwd_err_empty_port() : "";
    const value = Number(text);
    if (!/^\d{1,5}$/.test(text) || value < 1 || value > 65535)
      return m.s8_fwd_err_port_range();
    return taken.has(value) ? m.s8_fwd_err_port_taken() : "";
  });
  const shownError = $derived(inputError || addError);
  const validPort = $derived(
    port.trim() && !inputError ? Number(port.trim()) : null,
  );
  const titles = $derived(
    new Map(sessions.map((session) => [session.id, session.title])),
  );
  const detected = $derived(
    sessions
      .flatMap((session) =>
        session.listeningPorts
          .filter(
            (entry) => isPromptedPort(entry.port) && !taken.has(entry.port),
          )
          .map((entry) => ({
            ...entry,
            session: session.id,
            title: session.title || session.id,
          })),
      )
      .sort((a, b) => a.port - b.port),
  );

  function source(forward: Forward) {
    if (forward.label) return forward.label;
    const parts = [titles.get(forward.session), forward.process].filter(
      Boolean,
    );
    return parts.length ? parts.join(" · ") : m.s8_fwd_source_manual();
  }
  // Runs one row action, keeping its failure beside the row.
  async function run(key: string, action: () => Promise<unknown>) {
    if (busy.has(key)) return false;
    busy.add(key);
    delete rowErrors[key];
    try {
      await action();
      return true;
    } catch (err) {
      rowErrors[key] = errorMessage(err);
      return false;
    } finally {
      busy.delete(key);
    }
  }
  async function submit(event: SubmitEvent) {
    event.preventDefault();
    attempted = true;
    if (inputError || adding) return;
    adding = true;
    addError = "";
    try {
      await addForward({
        port: Number(port.trim()),
        options: $state.snapshot(addOptions),
      });
      port = "";
      attempted = false;
    } catch (err) {
      addError = errorMessage(err);
    } finally {
      adding = false;
    }
  }
  function toggleEdit(forward: Forward) {
    if (editingPort === forward.port) {
      editingPort = null;
      return;
    }
    editingPort = forward.port;
    draftLabel = forward.label;
    draftOptions = { ...forward.options };
    delete rowErrors[`edit:${forward.port}`];
  }
  async function save(target: number) {
    const saved = await run(`edit:${target}`, () =>
      changeForward(target, {
        label: draftLabel.trim(),
        options: $state.snapshot(draftOptions),
      }),
    );
    if (saved) editingPort = null;
  }
  async function copy(forward: Forward) {
    const link = forwardLink(forward.path);
    if (await copyText(link)) {
      copied = forward.port;
      delete rowErrors[`forward:${forward.port}`];
    } else {
      copied = null;
      rowErrors[`forward:${forward.port}`] = m.s8_fwd_copy_failed({ link });
    }
  }
</script>

<ResponsiveDialog
  title={m.s8_fwd_title()}
  description={m.s8_fwd_desc()}
  pinDrawerHeader
  {onclose}
>
  <div class="space-y-6">
    <form class="space-y-3" onsubmit={submit} novalidate>
      <div class="space-y-2">
        <Label for="forward-port">{m.s8_fwd_port_label()}</Label>
        <div class="flex gap-2">
          <Input
            id="forward-port"
            class="max-w-40"
            inputmode="numeric"
            autocomplete="off"
            placeholder={m.s8_fwd_port_placeholder()}
            bind:value={port}
            oninput={() => (addError = "")}
            aria-invalid={shownError ? true : undefined}
            aria-describedby="forward-port-error"
          />
          <Button type="submit" disabled={adding}>{m.s8_fwd_add_btn()}</Button>
        </div>
        <p
          id="forward-port-error"
          class="min-h-4 text-xs text-destructive"
          role={shownError ? "alert" : undefined}
        >
          {shownError}
        </p>
      </div>
      <AdvancedSection
        title={m.s8_fwd_compat_title()}
        summary={m.s8_fwd_compat_summary()}
      >
        <ForwardOptionsFields
          id="forward-new"
          port={validPort}
          bind:options={addOptions}
        />
      </AdvancedSection>
    </form>

    <section class="space-y-2" aria-labelledby="forward-list-title">
      <h3 id="forward-list-title" class="text-sm font-medium">
        {m.s8_fwd_forwarded_heading()}
        <span class="text-muted-foreground tabular-nums"
          >{forwards.list.length}</span
        >
      </h3>
      {#if forwards.list.length}
        <ul class="divide-y">
          {#each forwards.list as forward (forward.port)}
            {@const open = editingPort === forward.port}
            {@const key = `forward:${forward.port}`}
            <li
              class="grid grid-cols-[3.5rem_minmax(0,1fr)] items-center gap-x-3 gap-y-1 py-2 sm:grid-cols-[3.5rem_minmax(0,1fr)_auto]"
            >
              <span class="font-mono text-sm font-medium tabular-nums"
                >{forward.port}</span
              >
              <div class="min-w-0">
                <p class="truncate text-sm" title={source(forward)}>
                  {source(forward)}
                </p>
                <p
                  class="flex min-w-0 items-center gap-2 text-xs text-muted-foreground"
                >
                  <span
                    class={`status-dot ${forward.status === "listening" ? "status-running" : "status-idle"}`}
                    aria-hidden="true"
                  ></span>
                  <span class="shrink-0"
                    >{forward.status === "listening"
                      ? m.s8_fwd_status_listening()
                      : m.s8_fwd_status_unreachable()}</span
                  >
                  <span class="min-w-0 truncate font-mono" title={forward.path}
                    >{forward.path}</span
                  >
                </p>
              </div>
              <div class="col-start-2 flex items-center gap-1 sm:col-start-3">
                <Button
                  href={forward.path}
                  target="_blank"
                  rel="noopener"
                  variant="ghost"
                  size="icon-touch"
                  aria-label={m.s8_fwd_open_port_aria({
                    port: String(forward.port),
                  })}
                  title={m.s8_fwd_open_new_tab()}
                  ><IconExternalLink aria-hidden="true" /></Button
                >
                <Button
                  variant="ghost"
                  size="icon-touch"
                  aria-label={m.s8_fwd_copy_port_aria({
                    port: String(forward.port),
                  })}
                  title={copied === forward.port
                    ? m.s8_copied()
                    : m.s8_fwd_copy_link()}
                  onclick={() => void copy(forward)}
                  ><IconCopy aria-hidden="true" /></Button
                >
                <Button
                  variant="ghost"
                  size="icon-touch"
                  aria-label={m.s8_fwd_settings_port_aria({
                    port: String(forward.port),
                  })}
                  title={m.s8_fwd_settings()}
                  aria-expanded={open}
                  onclick={() => toggleEdit(forward)}
                  ><IconAdjustments aria-hidden="true" /></Button
                >
                <Button
                  variant="ghost"
                  size="icon-touch"
                  aria-label={m.s8_fwd_remove_port_aria({
                    port: String(forward.port),
                  })}
                  title={m.s8_fwd_remove()}
                  disabled={busy.has(key)}
                  onclick={() =>
                    void run(key, () => removeForward(forward.port))}
                  ><IconTrash aria-hidden="true" /></Button
                >
              </div>
              {#if rowErrors[key]}<p
                  class="col-span-full text-xs text-destructive wrap-anywhere"
                  role="alert"
                >
                  {rowErrors[key]}
                </p>{/if}
              {#if open}
                {@const editKey = `edit:${forward.port}`}
                <form
                  class="col-span-full mt-2 space-y-4 rounded-lg border p-4"
                  aria-label={m.s8_fwd_settings_port_aria({
                    port: String(forward.port),
                  })}
                  onsubmit={(event) => {
                    event.preventDefault();
                    void save(forward.port);
                  }}
                >
                  <div class="space-y-2">
                    <Label for={`forward-${forward.port}-label`}
                      >{m.s8_fwd_name_optional()}</Label
                    >
                    <Input
                      id={`forward-${forward.port}-label`}
                      maxlength={64}
                      placeholder={titles.get(forward.session) ||
                        m.s8_fwd_name_placeholder()}
                      bind:value={draftLabel}
                    />
                  </div>
                  <ForwardOptionsFields
                    id={`forward-${forward.port}`}
                    port={forward.port}
                    bind:options={draftOptions}
                  />
                  {#if rowErrors[editKey]}<p
                      class="text-xs text-destructive wrap-anywhere"
                      role="alert"
                    >
                      {rowErrors[editKey]}
                    </p>{/if}
                  <div class="flex justify-end gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      onclick={() => (editingPort = null)}
                      >{m.s8_cancel()}</Button
                    >
                    <Button type="submit" disabled={busy.has(editKey)}
                      >{m.s8_save()}</Button
                    >
                  </div>
                </form>
              {/if}
            </li>
          {/each}
        </ul>
      {:else}
        <Empty.Root variant="outline">
          <Empty.Header>
            <Empty.Media variant="icon"
              ><IconPlugConnected aria-hidden="true" /></Empty.Media
            >
            <Empty.Title>{m.s8_fwd_empty_title()}</Empty.Title>
            <Empty.Description>{m.s8_fwd_empty_desc()}</Empty.Description>
          </Empty.Header>
        </Empty.Root>
      {/if}
      <p class="sr-only" role="status">
        {copied === null ? "" : m.s8_fwd_copied_sr({ port: String(copied) })}
      </p>
    </section>

    <section class="space-y-2" aria-labelledby="forward-detected-title">
      <h3 id="forward-detected-title" class="text-sm font-medium">
        {m.s8_fwd_detected_heading()}
      </h3>
      {#if !detection}
        <p class="text-sm text-muted-foreground">
          {m.s8_fwd_no_detection()}
        </p>
      {:else if detected.length}
        <ul class="divide-y">
          {#each detected as entry (`${entry.session}:${entry.port}`)}
            {@const key = `detected:${entry.session}:${entry.port}`}
            {@const text = [entry.process, entry.title]
              .filter(Boolean)
              .join(" · ")}
            <li
              class="grid grid-cols-[3.5rem_minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 py-2"
            >
              <span class="font-mono text-sm font-medium tabular-nums"
                >{entry.port}</span
              >
              <span
                class="min-w-0 truncate text-sm text-muted-foreground"
                title={text}>{text}</span
              >
              <Button
                variant="outline"
                size="sm-touch"
                aria-label={m.s8_fwd_port_aria({ port: String(entry.port) })}
                disabled={busy.has(key)}
                onclick={() =>
                  void run(key, () =>
                    addForward({
                      port: entry.port,
                      session: entry.session,
                      process: entry.process,
                    }),
                  )}>{m.s8_fwd_action()}</Button
              >
              {#if rowErrors[key]}<p
                  class="col-span-full text-xs text-destructive wrap-anywhere"
                  role="alert"
                >
                  {rowErrors[key]}
                </p>{/if}
            </li>
          {/each}
        </ul>
      {:else}
        <p class="text-sm text-muted-foreground">
          {m.s8_fwd_no_listening_ports()}
        </p>
      {/if}
    </section>

    <p class="text-xs text-muted-foreground">
      {m.s8_fwd_same_origin_security_note()}
    </p>
  </div>
</ResponsiveDialog>
