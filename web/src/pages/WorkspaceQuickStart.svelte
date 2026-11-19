<script lang="ts">
  import { openStartedSession } from "../hooks/sessions/open-started-session";
  import { onMount, tick } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import IconLoader2 from "~icons/tabler/loader-2";
  import { stored, uploadQuickStartFile } from "../platform/api";
  import {
    setBrowserChrome,
    resetBrowserChrome,
  } from "../platform/browser-chrome";
  import type { ConfigValue } from "../platform/chat-api";
  import ChatUploadButton from "../components/chat/ChatUploadButton.svelte";
  import ComposerCard from "../components/chat/ComposerCard.svelte";
  import QuickStartAgentToolbar from "../components/sessions/QuickStartAgentToolbar.svelte";
  import QuickStartBanners from "../components/sessions/QuickStartBanners.svelte";
  import QuickStartInitInstructionBar from "../components/sessions/QuickStartInitInstructionBar.svelte";
  import QuickStartLocationBar from "../components/sessions/QuickStartLocationBar.svelte";
  import { voicePreference } from "../hooks/shared/voice-preference.svelte";
  import FileDropZone from "../components/shared/FileDropZone.svelte";
  import SessionSwitcherTrigger from "../components/shell/SessionSwitcherTrigger.svelte";
  import SidebarButton from "../components/shell/SidebarButton.svelte";
  import { useChatUpload } from "../hooks/shared/upload.svelte";
  import { useViewport } from "../hooks/shared/viewport.svelte";
  import {
    executeQuickStart,
    pickDefault,
    quickStartMemory,
    saveQuickStartDraftFromPage,
    type QuickStartBranches,
    type QuickStartTargetMode,
  } from "../hooks/sessions/quick-start";
  import {
    usesArtifactDir,
    usesWebmuxBrief,
  } from "../domains/settings/quick-commands";
  import { visibleLaunchConfigs } from "../domains/settings/launch-configs";
  import { insertAtSelection } from "../lib/text-insertion";
  import { useQuickStartAgentOptions } from "../hooks/sessions/quick-start-agent-options.svelte";
  import type {
    ChatAttachment,
    ChatPermissionMode as PermissionMode,
    CreateSessionFailure,
    Repository,
    SessionView,
    Worktree,
  } from "../contracts/schemas";
  import { getShell } from "../hooks/shared/shell.svelte";
  import { enterSendIntent } from "../hooks/shared/enter-send";
  import { allowAutomaticInputFocus } from "../hooks/shared/input-focus";
  import {
    loadQuickStartFeatures,
    type QuickStartFeatures,
  } from "../features/quick-start";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import QuickStartAccessories from "../components/sessions/QuickStartAccessories.svelte";
  import {
    featureAvailable,
    featureCapabilityRevision,
  } from "../platform/feature-host.svelte";
  let features = $state.raw<QuickStartFeatures>({ errors: [] });
  const shell = getShell();

  const repositories = $derived(shell.repositories?.list ?? []);
  const allHybridConfigs = $derived(
    shell.launchConfigs?.state.status === "ready"
      ? shell.launchConfigs.state.configs.filter((config) => config.chat)
      : [],
  );
  const agents = $derived(
    shell.launchConfigs
      ? visibleLaunchConfigs(shell.launchConfigs.state).filter(
          (config) => config.chat,
        )
      : [],
  );
  const initialDraft = quickStartMemory.loadDraft();
  const remembered = quickStartMemory.choice();
  let repositoryChoice = $state(
    initialDraft.repositoryId ?? remembered?.repositoryId,
  );
  let agentChoice = $state(initialDraft.configId ?? remembered?.configId);
  const usableRepository = (repo: Repository) =>
    repo.availability === "available";
  const repository = $derived(
    pickDefault(repositories, repositoryChoice, usableRepository),
  );
  const agent = $derived(pickDefault(agents, agentChoice, () => true));

  let startSide = $state<SessionView>(
    stored("quickStartSide") === "tui" ||
      stored("quickStartSide") === "terminal"
      ? "terminal"
      : "chat",
  );
  const chat = $derived(startSide === "chat");

  let text = $state(initialDraft.text);
  let attachments = $state<ChatAttachment[]>(initialDraft.attachments ?? []);
  let permissionMode = $state<PermissionMode>(
    initialDraft.permissionMode ?? "auto",
  );
  let edits = $state<Record<string, Record<string, ConfigValue>>>(
    initialDraft.edits ?? {},
  );
  let baseChoice = $state<{ repositoryId: string; name: string } | undefined>(
    initialDraft.baseChoice,
  );
  let targetModeChoice = $state<QuickStartTargetMode | undefined>(
    initialDraft.targetMode ?? remembered?.targetMode,
  );
  let worktreeChoice = $state<
    { repositoryId: string; worktreeId: string } | undefined
  >(initialDraft.worktreeChoice ?? remembered?.worktreeChoice);
  let instructionChoice = $state(initialDraft.instructionChoice);

  function persistDraft() {
    saveQuickStartDraftFromPage({
      text,
      attachments,
      repositoryChoice,
      agentChoice,
      targetModeChoice,
      baseChoice,
      worktreeChoice,
      permissionMode,
      edits,
      instructionChoice,
      remembered,
    });
  }
  $effect(() => {
    persistDraft();
  });

  const agentOptions = useQuickStartAgentOptions(
    () => agent,
    () => repository,
    () => edits,
  );
  const probing = $derived(agentOptions.probing);
  const options = $derived(agentOptions.options);
  function setOption(id: string, value: ConfigValue) {
    if (!agent) return;
    edits = { ...edits, [agent.id]: { ...edits[agent.id], [id]: value } };
    persistDraft();
  }

  let targetMode = $state<QuickStartTargetMode>("new");
  let selectedWorktree = $state<Worktree | undefined>();
  let branchData = $state<QuickStartBranches | undefined>();
  let branchError = $state<string | undefined>();
  let base = $state("");

  const launchSettled = $derived(
    shell.launchConfigs?.state.status === "ready" &&
      !shell.launchConfigs.state.agentToolsLoading,
  );
  const missing = $derived<"" | "agent" | "agent-uninstalled" | "repository">(
    launchSettled && !agents.length
      ? allHybridConfigs.length
        ? "agent-uninstalled"
        : "agent"
      : shell.repositories?.settled && !repositories.some(usableRepository)
        ? "repository"
        : "",
  );
  const useInstruction = $derived(
    instructionChoice && instructionChoice.repositoryId === repository?.id
      ? instructionChoice.use
      : chat,
  );
  let sending = $state(false);
  let failure = $state<CreateSessionFailure>();
  let uploadError = $state("");
  const uploads = useChatUpload(
    uploadQuickStartFile,
    (message) => (uploadError = message),
    {
      get: () => attachments,
      set: (next) => {
        attachments = next;
        persistDraft();
      },
    },
  );
  const locked = $derived(sending || (chat && probing));
  const hasTarget = $derived(
    targetMode === "existing" ? !!selectedWorktree : !!base,
  );
  const canSend = $derived(
    !failure?.sessionId &&
      !locked &&
      !uploads.progress &&
      !!repository &&
      !!agent &&
      hasTarget &&
      text.trim() !== "",
  );

  let input = $state<HTMLTextAreaElement | null>(null);
  const viewport = useViewport();
  $effect(() => {
    setBrowserChrome(m.s1_chrome_new_chat(), false);
  });
  onMount(() => {
    if (allowAutomaticInputFocus() && window.innerWidth >= 640) input?.focus();
    return resetBrowserChrome;
  });
  $effect(() => {
    featureCapabilityRevision();
    let disposed = false;
    void loadQuickStartFeatures().then((loaded) => {
      if (!disposed) features = loaded;
    });
    return () => {
      disposed = true;
    };
  });
  async function send() {
    if (!canSend || !repository || !agent) return;
    if (targetMode === "new" && !branchData) return;
    if (targetMode === "existing" && !selectedWorktree) return;
    sending = true;
    failure = undefined;
    const staged = uploads.take();
    try {
      failure = await executeQuickStart({
        repository,
        agentId: agent.id,
        startSide,
        text,
        useInstruction,
        attachments: staged,
        options,
        permissionMode,
        targetMode,
        selectedWorktree,
        branchData,
        base,
        commitAttachments: (items) => uploads.commit(items),
        restoreAttachments: (items) => uploads.restore(items),
        onSuccess: (sessionId) => {
          return openStartedSession(sessionId, input, chat);
        },
      });
    } finally {
      sending = false;
      void shell.repositories?.refresh();
    }
  }
  async function insertTextAtCursor(snippet: string) {
    const result = insertAtSelection(
      text,
      input?.selectionStart ?? text.length,
      input?.selectionEnd ?? text.length,
      snippet,
    );
    text = result.text;
    persistDraft();
    await tick();
    input?.focus();
    input?.setSelectionRange(result.caret, result.caret);
  }
  function keydown(event: KeyboardEvent) {
    if (!enterSendIntent(event)) return;
    event.preventDefault();
    void send();
  }
</script>

{#snippet agentToolbar(compact = false)}
  {#if agent}
    <QuickStartAgentToolbar
      {compact}
      {agents}
      {agent}
      {startSide}
      {chat}
      {permissionMode}
      {options}
      {locked}
      {probing}
      {sending}
      onagentchange={(id) => {
        agentChoice = id;
        persistDraft();
      }}
      onsidechange={(side) => {
        startSide = side;
      }}
      onmodechange={(mode) => {
        permissionMode = mode;
        persistDraft();
      }}
      onoptionchange={setOption}
    />
  {/if}
{/snippet}

<main
  class="flex h-(--viewport-height) flex-col overflow-hidden"
  style:--viewport-height={`${viewport.height}px`}
>
  <header
    class="app-header flex min-h-(--app-header-height) shrink-0 items-center gap-1 border-b bg-card px-1 sm:px-4"
  >
    <SidebarButton class="sm:hidden" />
    <h1 class="flex-1 text-sm font-medium">{m.s1_new_session()}</h1>
    <SessionSwitcherTrigger size="icon-sm" />
  </header>

  <!-- Files dropped anywhere below the header become attachments. -->
  <FileDropZone
    class="relative flex min-h-0 flex-1 flex-col overflow-y-auto"
    {uploads}
    blocked={sending}
    label={m.s1_drop_to_attach_message()}
  >
    <section
      class="grid w-full flex-1 shrink-0 grid-rows-[minmax(min-content,1fr)_auto_1fr] gap-4 py-4 sm:gap-6"
    >
      <div
        class="flex flex-col items-center justify-end gap-2 px-4 text-center"
      >
        <h2 class="text-2xl font-medium">{m.s1_quick_start_heading()}</h2>
        <p class="max-w-md text-sm text-muted-foreground">
          {targetMode === "existing"
            ? chat
              ? m.s1_quick_start_sub_existing_chat()
              : m.s1_quick_start_sub_existing_terminal()
            : chat
              ? m.s1_quick_start_sub_new_chat()
              : m.s1_quick_start_sub_new_terminal()}
        </p>
      </div>

      <footer
        class="mx-auto flex min-h-0 w-full max-w-[46rem] flex-col gap-1.5 px-3 pb-safe-2 sm:px-4 sm:pb-6 [&>[data-chat-composer]]:shrink-0"
        data-quick-start-composer
      >
        <div
          class="min-h-0 space-y-1.5 overflow-y-auto overscroll-contain empty:hidden"
        >
          {#if sending}
            <p
              class="flex items-center gap-1.5 px-1 text-xs text-muted-foreground wrap-anywhere"
              role="status"
            >
              <IconLoader2
                class="size-3.5 shrink-0 animate-spin motion-reduce:animate-none"
                aria-hidden="true"
              />{targetMode === "existing"
                ? m.s1_starting_agent({ name: agent?.name ?? "" })
                : m.s1_creating_worktree_starting_agent({
                    name: agent?.name ?? "",
                  })}
            </p>
          {/if}
          <QuickStartBanners
            task={text}
            {missing}
            {targetMode}
            {branchError}
            {repository}
            {failure}
            {shell}
          />
          {#each features.errors as message (message)}<InlineNotice
              tone="warning">{message}</InlineNotice
            >{/each}
        </div>

        <ComposerCard
          bind:textarea={input}
          value={text}
          oninput={(event) => {
            text = event.currentTarget.value;
            persistDraft();
          }}
          placeholder={targetMode === "existing"
            ? m.s1_quick_start_placeholder_existing()
            : m.s1_quick_start_placeholder_new()}
          label={m.s1_task_description()}
          disabled={sending}
          {uploads}
          uploadsDisabled={sending}
          busy={Boolean(
            text || attachments.length > 0 || uploads.progress || sending,
          )}
          action={{
            kind: "send",
            label: m.s1_send(),
            disabled: !canSend,
            onclick: () => void send(),
          }}
          textareaProps={{ onkeydown: keydown }}
        >
          {#snippet location({ compact })}
            <QuickStartLocationBar
              {compact}
              {repositories}
              {repository}
              {sending}
              {targetModeChoice}
              {worktreeChoice}
              {baseChoice}
              {uploadError}
              bind:targetMode
              bind:selectedWorktree
              bind:branchData
              bind:branchError
              bind:base
              onrepositorychange={(id) => {
                repositoryChoice = id;
                instructionChoice = undefined;
                persistDraft();
              }}
              ontargetmodechange={(next) => {
                if (targetModeChoice !== next) {
                  targetModeChoice = next;
                  persistDraft();
                }
              }}
              onselectworktree={(worktreeId) => {
                targetModeChoice = "existing";
                worktreeChoice = {
                  repositoryId: repository?.id ?? "",
                  worktreeId,
                };
                persistDraft();
              }}
              onselectbranch={(name) => {
                targetModeChoice = "new";
                baseChoice = { repositoryId: repository?.id ?? "", name };
                persistDraft();
              }}
            />
          {/snippet}
          {#snippet menu({ compact })}
            {#if repository?.initInstruction}
              <QuickStartInitInstructionBar
                {compact}
                {repository}
                {sending}
                {useInstruction}
                onchange={(next) => {
                  instructionChoice = repository
                    ? { repositoryId: repository.id, use: next }
                    : undefined;
                  persistDraft();
                }}
              />
            {/if}
          {/snippet}
          {#snippet hints()}
            {#if usesArtifactDir(text)}
              <p class="px-1 pt-1 text-xs text-muted-foreground">
                {m.s1_hint_artifact_dir({ token: "{{dir}}" })}
              </p>
            {/if}
            {#if usesWebmuxBrief(text)}
              <p class="px-1 pt-1 text-xs text-muted-foreground">
                {m.s1_hint_webmux_brief({ token: "{{webmux}}" })}
              </p>
            {/if}
          {/snippet}
          {#snippet leading()}
            <ChatUploadButton {uploads} disabled={sending} />
            <span class="contents" inert={sending}>
              {@render agentToolbar(true)}
            </span>
          {/snippet}
          {#snippet accessories({ size, variant })}
            <QuickStartAccessories
              {features}
              voice={featureAvailable("voice.recognize") &&
                voicePreference.enabled}
              {sending}
              {size}
              {variant}
              oninsert={(text) => void insertTextAtCursor(text)}
              onerror={(message) => (uploadError = message)}
              run={(command) => void insertTextAtCursor(command.command)}
              onfocus={() => input?.focus()}
            />
          {/snippet}
          {#snippet below()}
            {@render agentToolbar(false)}
          {/snippet}
        </ComposerCard>
      </footer>
    </section>
  </FileDropZone>
</main>
