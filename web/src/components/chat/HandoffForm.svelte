<script lang="ts">
  import { tick, untrack } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import { Textarea } from "$lib/components/ui/textarea";
  import * as Select from "$lib/components/ui/select";
  import { m } from "$lib/paraglide/messages.js";
  import {
    emptyHandoffLocation,
    type HandoffEditor,
  } from "../../hooks/sessions/handoff.svelte";
  import type { TargetKind } from "../../domains/sessions/new-session-defaults";
  import { visibleLaunchConfigs } from "../../domains/settings/launch-configs";
  import { handoffMaxPages } from "../../contracts/handoff-schemas";
  import { getShell, openSession } from "../../hooks/shared/shell.svelte";
  import { getFeatureContributions } from "../../features/context";
  import { featureAvailable } from "../../platform/feature-host.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { PickerField } from "$lib/components/ui/picker-field";
  import * as Command from "$lib/components/ui/command";
  import HandoffPreviewCard from "./HandoffPreviewCard.svelte";
  let { editor: e, fork }: { editor: HandoffEditor; fork: boolean } = $props();
  const shell = getShell();
  const { LocationPicker, WorktreeCreateFields } =
    getFeatureContributions().creation;
  const workspaceAvailable = featureAvailable("workspace.manage");
  const agents = $derived(
    shell.launchConfigs
      ? visibleLaunchConfigs(shell.launchConfigs.state).filter(
          (item) => item.chat,
        )
      : [],
  );
  const repositories = $derived(shell.repositories?.list ?? []);
  const agent = $derived(agents.find((item) => item.id === e.configId));
  const repository = $derived(
    repositories.find((item) => item.id === e.location.repositoryId),
  );
  let worktreeFields = $state<{ isReady: () => boolean }>();
  let managedPath = $state("");
  let existing = $state(false);
  let more = $state(false);
  let locationOpen = $state(false);
  let targetOpen = $state(false);
  let heading = $state<HTMLHeadingElement>();
  let viewport = $state<HTMLElement>();
  const reviewing = $derived(!!e.preview || e.sealed);
  const candidates = $derived(
    shell.sessions.sessions.filter(
      (item) =>
        item.id !== e.source?.id &&
        item.state === "active" &&
        item.view === "chat" &&
        !item.transition &&
        item.agent?.sessionId &&
        !item.agent.work,
    ),
  );
  const target = $derived(
    candidates.find((item) => item.id === e.targetSessionId),
  );
  const targetName = $derived(
    existing
      ? target?.title || target?.agent?.title || e.targetSessionId
      : agent?.name || "",
  );
  const worktree = $derived(
    repository?.worktrees.find((item) => item.id === e.location.worktreeId),
  );
  const destination = $derived(
    existing
      ? target?.cwd || ""
      : e.location.newWorktreeBranch ||
          e.location.cwd ||
          worktree?.path ||
          repository?.anchorPath ||
          "",
  );
  let reviewedName = $state("");
  let reviewedLocation = $state("");
  const locationReady = $derived(
    existing
      ? !!target
      : !!agent &&
          (e.locationKind === "directory"
            ? !!e.location.cwd.trim()
            : e.locationKind === "new-worktree"
              ? (worktreeFields?.isReady() ?? false)
              : !!repository),
  );
  async function focusStep() {
    await tick();
    viewport?.scrollTo({ top: 0 });
    heading?.focus();
  }
  async function generate() {
    if (!e.commitMaxPages()) {
      more = true;
      await tick();
      document.getElementById("handoff-pages")?.focus();
      return;
    }
    reviewedName = targetName;
    reviewedLocation = destination;
    await e.generate();
    if (e.preview) await focusStep();
  }
  async function edit(budget = false) {
    e.edit();
    more = budget;
    await focusStep();
    if (budget) document.getElementById("handoff-pages")?.focus();
  }
  $effect(() => {
    if (
      !e.active ||
      e.busy ||
      e.operation?.state === "awaiting_upstream" ||
      e.error
    )
      return;
    const timer = setTimeout(() => void e.refresh(), 1200);
    return () => clearTimeout(timer);
  });
  function kind(next: TargetKind) {
    e.locationKind = next;
    const old = untrack(() => e.location);
    e.location =
      next === "directory"
        ? { ...emptyHandoffLocation(), cwd: old.cwd }
        : { ...emptyHandoffLocation(), repositoryId: old.repositoryId };
  }
  function repositoryId(value: string) {
    e.location = { ...emptyHandoffLocation(), repositoryId: value };
  }
  function worktreeId(value: string) {
    e.location = {
      ...untrack(() => e.location),
      worktreeId: value,
      cwd: "",
      newWorktreeBranch: "",
      newWorktreeBase: "",
    };
  }
</script>

<main bind:this={viewport} class="min-h-0 flex-1 overflow-y-auto">
  <div
    data-handoff-form
    class="mx-auto w-full max-w-2xl space-y-6 px-4 py-6 sm:px-6"
  >
    <div
      class={reviewing
        ? "sticky top-0 z-10 space-y-1 border-b bg-background py-3"
        : "space-y-1"}
    >
      <h2 bind:this={heading} tabindex="-1" class="text-lg font-semibold">
        {reviewing ? reviewedName : m.handoff_setup()}
      </h2>
      <p class="break-all text-sm text-muted-foreground">
        {reviewing
          ? reviewedLocation
          : m.handoff_origin({
              name: e.source?.title || e.source?.agent?.title || "",
            })}
      </p>
    </div>
    {#if e.error}<InlineNotice tone="danger">{e.error}</InlineNotice>{/if}
    {#if reviewing}
      <HandoffPreviewCard
        editor={e}
        name={reviewedName}
        onadjust={() => edit(true)}
      />
    {:else if e.source}
      <fieldset class="min-w-0 space-y-5" disabled={e.busy}>
        {#if !fork}
          <SegmentedControl
            value={existing ? "existing" : "new"}
            aria-label={m.handoff_receiver()}
            size="form"
            wrap
            onValueChange={(v) => {
              existing = v === "existing";
              e.targetSessionId = "";
              targetOpen = false;
            }}
          >
            <SegmentedControlItem value="new"
              >{m.handoff_new_target()}</SegmentedControlItem
            >
            <SegmentedControlItem value="existing"
              >{m.handoff_existing()}</SegmentedControlItem
            >
          </SegmentedControl>
          <p class="text-sm text-muted-foreground">{m.handoff_preserve()}</p>
        {/if}
        {#if existing}
          <div class="space-y-2">
            <Label for="handoff-target">{m.handoff_receiver()}</Label>
            <PickerField id="handoff-target" bind:open={targetOpen}>
              {#snippet trigger()}{targetName || m.handoff_search()}{/snippet}
              <Command.Root label={m.handoff_receiver()}
                ><Command.Input
                  aria-label={m.handoff_search()}
                  placeholder={m.handoff_search()}
                />
                <Command.List
                  ><Command.Empty>{m.handoff_no_targets()}</Command.Empty>
                  {#each candidates as item (item.id)}<Command.Item
                      value={item.id}
                      keywords={[
                        item.title,
                        item.agent?.name || "",
                        item.agent?.title || "",
                        item.cwd,
                      ]}
                      onSelect={() => {
                        e.targetSessionId = item.id;
                        targetOpen = false;
                      }}
                    >
                      <span class="min-w-0"
                        ><span class="block truncate"
                          >{item.title || item.id}</span
                        ><span
                          class="block truncate text-xs text-muted-foreground"
                          >{item.agent?.title} · {item.cwd}</span
                        ></span
                      ></Command.Item
                    >{/each}
                </Command.List></Command.Root
              >
            </PickerField>
          </div>
        {:else}
          <div class="space-y-2">
            <Label for="handoff-agent">{m.s3_handoff_target_agent()}</Label>
            <Select.Root type="single" bind:value={e.configId}>
              <Select.Trigger id="handoff-agent"
                >{agent?.name ??
                  m.s3_handoff_select_target_agent()}</Select.Trigger
              >
              <Select.Content
                >{#each agents as item (item.id)}<Select.Item
                    value={item.id}
                    label={item.name}>{item.name}</Select.Item
                  >{/each}</Select.Content
              >
            </Select.Root>
          </div>

          <details bind:open={locationOpen}>
            <summary class="cursor-pointer break-all text-sm hover:underline"
              >{m.s3_working_directory()} · {destination} · {m.handoff_change()}</summary
            >
            <div class="mt-3 space-y-3">
              {#if workspaceAvailable && LocationPicker}
                <LocationPicker
                  {repositories}
                  recent={[]}
                  preferredRepositoryId={e.source?.workspace?.repositoryId ??
                    ""}
                  createdTree={null}
                  newWorktreeSummary={e.location.newWorktreeBranch ||
                    m.s3_handoff_create_worktree_on_send()}
                  bind:kind={() => e.locationKind, kind}
                  bind:cwd={
                    () => e.location.cwd,
                    (value) =>
                      (e.location = {
                        ...untrack(() => e.location),
                        cwd: value,
                      })
                  }
                  bind:repositoryId={
                    () => e.location.repositoryId, repositoryId
                  }
                  bind:worktreeId={() => e.location.worktreeId, worktreeId}
                  disabled={e.busy || e.active}
                />
              {:else}
                <div class="min-w-0 space-y-2">
                  <Label for="handoff-cwd">{m.s3_working_directory()}</Label>
                  <Input
                    id="handoff-cwd"
                    value={e.location.cwd}
                    oninput={(event) => {
                      e.location = {
                        ...emptyHandoffLocation(),
                        cwd: event.currentTarget.value,
                      };
                      e.locationKind = "directory";
                    }}
                    placeholder={m.s3_enter_abs_dir_placeholder()}
                    autocomplete="off"
                    disabled={e.busy || e.active}
                  />
                </div>
              {/if}
              {#if workspaceAvailable && WorktreeCreateFields && e.locationKind === "new-worktree" && repository}
                {#key repository.id}<WorktreeCreateFields
                    bind:this={worktreeFields}
                    {repository}
                    deferred
                    preset={{
                      mode: "new",
                      name: e.location.newWorktreeBranch,
                      base: e.location.newWorktreeBase,
                    }}
                    mode="new"
                    bind:path={managedPath}
                    bind:branch={
                      () => e.location.newWorktreeBranch,
                      (value) =>
                        (e.location = {
                          ...untrack(() => e.location),
                          newWorktreeBranch: value,
                        })
                    }
                    bind:base={
                      () => e.location.newWorktreeBase,
                      (value) =>
                        (e.location = {
                          ...untrack(() => e.location),
                          newWorktreeBase: value,
                        })
                    }
                    disabled={e.busy || e.active}
                  />{/key}
                <p class="text-xs text-muted-foreground">
                  {m.s3_handoff_worktree_deferred_hint()}
                </p>
              {/if}
            </div>
          </details>
        {/if}
        <div class="space-y-2">
          <Label for="handoff-task">{m.handoff_next_task()}</Label>
          <Textarea
            id="handoff-task"
            rows={4}
            placeholder={m.handoff_task_hint()}
            bind:value={e.task}
          />
        </div>
        <details bind:open={more}>
          <summary class="cursor-pointer text-sm hover:underline"
            >{m.handoff_more()}</summary
          >
          <div class="mt-3 space-y-3">
            {#if !existing}
              <div class="space-y-2">
                <Label for="handoff-model"
                  >{m.s3_handoff_model_optional()}</Label
                ><Input id="handoff-model" bind:value={e.model} />
              </div>
            {/if}
            <div class="space-y-2 pb-3">
              <Label for="handoff-pages"
                >{m.handoff_max_pages({ max: handoffMaxPages })}</Label
              >
              <Input
                id="handoff-pages"
                inputmode="numeric"
                autocomplete="off"
                bind:value={e.maxPagesDraft}
                onblur={() => e.commitMaxPages()}
                aria-invalid={!!e.maxPagesError}
                aria-describedby={e.maxPagesError
                  ? "handoff-pages-help handoff-pages-error"
                  : "handoff-pages-help"}
              />
              <p id="handoff-pages-help" class="text-xs text-muted-foreground">
                {m.handoff_image_reselect()}
              </p>
            </div>
            {#if e.maxPagesError}
              <p
                id="handoff-pages-error"
                class="pb-3 text-xs text-destructive"
                role="alert"
              >
                {e.maxPagesError}
              </p>
            {/if}
          </div>
        </details>
      </fieldset>
    {/if}
  </div>
</main>
<footer class="shrink-0 border-t bg-background">
  <div class="mx-auto flex max-w-2xl flex-wrap justify-end gap-2 p-3 sm:px-6">
    {#if !reviewing}
      <Button disabled={!locationReady || e.busy} onclick={generate}
        >{m.handoff_review()}</Button
      >
    {:else if !e.sealed || e.operation?.state === "failed"}
      <Button variant="outline" disabled={e.busy} onclick={() => edit()}
        >{e.preview?.capability === "unsupported" ||
        e.operation?.state === "failed"
          ? m.handoff_reselect()
          : m.handoff_edit()}</Button
      >
      {#if e.operation?.targetSessionId}<Button
          variant="outline"
          onclick={() => openSession({ id: e.operation!.targetSessionId })}
          >{m.handoff_open_target()}</Button
        >{/if}
      {#if !e.sealed}<Button
          disabled={e.busy || !!e.preview?.blockedReason || e.choiceChanged}
          onclick={() => e.send()}
          >{existing
            ? m.handoff_send_existing()
            : m.handoff_create_send()}</Button
        >{/if}
    {:else if e.operation?.targetSessionId}
      <Button onclick={() => openSession({ id: e.operation!.targetSessionId })}
        >{m.handoff_open_target()}</Button
      >
    {:else}<Button
        variant="outline"
        disabled={e.busy}
        onclick={() => e.refresh()}>{m.handoff_refresh_result()}</Button
      >{/if}
  </div>
</footer>
