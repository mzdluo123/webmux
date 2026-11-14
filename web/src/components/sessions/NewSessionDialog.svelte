<script lang="ts">
  import {
    projectTarget,
    projectRepositories,
    contextProject,
    scopeForProject,
    type ProjectSelection,
  } from "../../domains/sessions/project-selection";
  import { errorMessage } from "../../platform/error";
  import { untrack, tick } from "svelte";
  import { navigate, recentDirs, stored } from "../../platform/api";
  import {
    pluginAvailable,
    pluginEnabled,
  } from "../../platform/plugin-host.svelte";
  import { getPluginContributions } from "../../plugins/context";
  import type { LaunchConfigsState } from "../../domains/settings/launch-configs";
  import {
    isCommandReady,
    isLocationTargetReady,
    resolveFallbackAfterWorktreeRemoved,
    resolveInitialLaunch,
    resolveInitialTarget,
    resolveInstructionRepository,
    storedView,
    submitNewSessionRequest,
    validateNewSessionSubmit,
    type LaunchEntry,
    type LaunchScope,
    type LaunchSource,
  } from "../../domains/sessions/new-session-defaults";
  import {
    sessionConflict,
    isDirectoryNotFoundError,
  } from "../../domains/sessions/session-conflict";
  import type {
    AgentConnector,
    Repository,
    SessionInfo,
    SessionView,
    Worktree,
  } from "../../contracts/schemas";
  import NewSessionRecoveryDialogs from "./NewSessionRecoveryDialogs.svelte";
  import NewSessionForm from "./NewSessionForm.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import type { SplitDirection } from "../../domains/terminal/terminal-layout";

  interface Props {
    launchConfigs: LaunchConfigsState;
    defaultCwd: string;
    repositories?: Repository[];
    /** The contextual entry the dialog was opened from, if any. */
    entry?: LaunchEntry;
    /** Narrows the places offered; see LaunchScope. */
    scope?: LaunchScope;
    /** The home page's selected repository; read only. */
    selectedRepositoryId?: string;
    intent?: "session" | "worktree";
    split?: boolean;
    oncreated?: (session: SessionInfo, direction: SplitDirection) => void;
    onchange: () => void;
    onworktreecreated: (tree: Worktree) => void;
    /** A worktree was deleted from the location list. */
    onworktreeremoved?: (tree: Worktree) => void;
    onclose: () => void;
  }

  let {
    launchConfigs,
    defaultCwd,
    repositories: hostRepositories = [],
    entry,
    scope,
    selectedRepositoryId = "",
    intent = "session",
    split = false,
    oncreated,
    onchange,
    onworktreecreated,
    onworktreeremoved,
    onclose,
  }: Props = $props();
  const providers = getPluginContributions().creation;
  let projectSelection = $state.raw<ProjectSelection | null>(null);
  const repositories = $derived(
    projectRepositories(hostRepositories, projectSelection),
  );
  const projectEnabled = $derived(
    pluginAvailable("workspace.projects") && !!providers.ProjectPicker,
  );
  const projectRestricted = $derived(!!scope || intent === "worktree");
  let choosingProject = $state(
    untrack(
      () =>
        projectEnabled &&
        !entry &&
        !projectRestricted &&
        !hostRepositories.some((r) => r.id === selectedRepositoryId),
    ),
  );
  let pickerMounted = $state(untrack(() => choosingProject));
  let projectPicker = $state<{
    back: () => boolean;
    showProjects: () => void;
  }>();

  const workspaceEnabled = $derived(
    pluginAvailable("workspace.manage") && !!providers.LocationPicker,
  );
  const launchEnabled = pluginEnabled("launch") && !!providers.LaunchPicker;
  const chatEnabled = $derived(
    pluginAvailable("chat.acp") && !!providers.LaunchStartSideFields,
  );
  const filesEnabled = $derived(
    pluginAvailable("files.upload") && !!providers.DirectoryPicker,
  );
  const recent = recentDirs();
  const initial = untrack(() =>
    resolveInitialTarget({
      workspaceAvailable: workspaceEnabled || projectEnabled,
      entry,
      scope,
      selectedRepositoryId: scope?.repositoryId ?? selectedRepositoryId,
      repositories,
      recent,
      defaultCwd,
    }),
  );
  const preferredRepositoryId = initial.repositoryId;
  // A split keeps the pane's workspace while its directory is unchanged.
  const entryDirectory = untrack(() =>
    entry?.kind === "directory"
      ? {
          ...entry,
          workspace:
            workspaceEnabled || projectEnabled ? entry.workspace : null,
        }
      : null,
  );
  let kind = $state(initial.kind);
  let cwd = $state(initial.cwd);
  let repositoryId = $state(initial.repositoryId);
  let worktreeId = $state(initial.worktreeId);
  let withSession = $state(untrack(() => intent === "session"));
  let direction = $state<SplitDirection>("horizontal");
  const treeOnly = $derived(
    workspaceEnabled && intent === "worktree" && !withSession,
  );

  const lastLaunch = stored("lastLaunch");
  const startsCustom =
    lastLaunch === "custom" ||
    (!launchEnabled && !lastLaunch.startsWith("config:"));
  let source = $state<LaunchSource>(startsCustom ? "custom" : "unset");
  let configId = $state("");
  // Defaults apply once, and never after the user picked a source.
  let launchResolved = lastLaunch === "custom";
  let launchTouched = false;
  // A remembered command belongs to the remembered launch method: starting on
  // a config or an unresolved choice must not prefill it when the user later
  // switches to a custom command.
  let command = $state(startsCustom ? stored("customCommand") : "");
  let startSide = $state<SessionView>(storedView(stored("hybridStartSide")));
  let name = $state("");
  let prompt = $state(
    untrack(() =>
      workspaceEnabled && entry?.kind === "worktree"
        ? (entry.prompt ?? "")
        : "",
    ),
  );
  let error = $state("");
  // Starting on the chat view failed: its connector is not installed.
  let missingConnector = $state.raw<AgentConnector>();
  let busy = $state(false);
  let mkdirOpen = $state(false);
  // Avoid initializing an unused portal on every open; keep it mounted after
  // first use so closing animations and focus restoration run.
  let mkdirMounted = $state(false);

  const configs = $derived(
    launchEnabled && launchConfigs.status === "ready"
      ? launchConfigs.configs
      : [],
  );
  const chosenConfig = $derived(
    source === "config" ? configs.find((c) => c.id === configId) : undefined,
  );
  $effect(() => {
    if (!launchResolved && !launchTouched) {
      const resolved = resolveInitialLaunch(
        lastLaunch,
        launchConfigs,
        launchEnabled,
      );
      if (resolved) {
        launchResolved = true;
        source = resolved.source;
        configId = resolved.configId;
      }
    }
    // Never silently replace a config deleted while the dialog is open.
    if (
      launchConfigs.status !== "loading" &&
      source === "config" &&
      !chosenConfig
    )
      source = "unset";
    if (!chatEnabled && chosenConfig?.chat) source = "unset";
    if (chatEnabled && chosenConfig?.chat) {
      startSide = storedView(
        stored("hybridStartSide:" + chosenConfig.id) ||
          stored("hybridStartSide"),
      );
    }
  });
  const repository = $derived(repositories.find((r) => r.id === repositoryId));
  const currentProject = $derived(
    projectEnabled
      ? (projectSelection?.project ??
          contextProject(repository, entryDirectory?.cwd))
      : undefined,
  );
  const projectScope = $derived(scopeForProject(scope, currentProject));
  async function changeProject() {
    pickerMounted = true;
    choosingProject = true;
    await tick();
    projectPicker?.showProjects();
  }
  function selectedProject(selection: ProjectSelection) {
    if (
      selection.project.id !== currentProject?.id ||
      (selection.openedPath &&
        projectTarget(selection).worktreeId !== worktreeId)
    ) {
      projectSelection = selection;
      ({ kind, cwd, repositoryId, worktreeId } = projectTarget(selection));
      newPath = branch = base = startBranch = fieldsRepositoryId = "";
      branchMode = "new";
      createdTree = null;
      instructionChoice = undefined;
    }
    choosingProject = false;
    error = "";
    onchange();
  }
  function back() {
    if (choosingProject) return projectPicker?.back() ?? false;
    if (projectEnabled && !projectRestricted) {
      changeProject();
      return true;
    }
    return false;
  }
  const instructionRepository = $derived(
    resolveInstructionRepository(
      kind,
      cwd,
      projectSelection ? null : entryDirectory,
      repositories,
      repository,
    ),
  );
  const hasInstruction = $derived(!!instructionRepository?.initInstruction);
  const startsOnChat = $derived(
    chatEnabled && !!chosenConfig?.chat && startSide === "chat",
  );
  let instructionChoice = $state<boolean | undefined>();
  const useInstruction = $derived(instructionChoice ?? startsOnChat);
  const commandReady = $derived(isCommandReady(source, chosenConfig, command));

  let newPath = $state("");
  let branchMode = $state<"new" | "existing">("new");
  let branch = $state("");
  let base = $state("");
  let startBranch = $state("");
  let worktreeFields = $state<{ isReady: () => boolean }>();
  let fieldsRepositoryId = $state(
    initial.kind === "new-worktree" ? initial.repositoryId : "",
  );
  const fieldsRepository = $derived(
    repositories.find((r) => r.id === fieldsRepositoryId),
  );
  let createdTree = $state<Worktree | null>(null);
  const worktreeReady = $derived(worktreeFields?.isReady() ?? false);
  const worktree = $derived(
    createdTree?.id === worktreeId
      ? createdTree
      : repository?.worktrees.find((w) => w.id === worktreeId),
  );
  let removing = $state<Worktree | null>(null);
  function removed(tree: Worktree) {
    onworktreeremoved?.(tree);
    if (createdTree?.id === tree.id) createdTree = null;
    ({ kind, worktreeId } = resolveFallbackAfterWorktreeRemoved(
      repositories,
      kind,
      worktreeId,
      tree,
      projectEnabled ? projectScope : undefined,
    ));
  }
  const treeTargetMissing = $derived(treeOnly && kind !== "new-worktree");
  const targetReady = $derived(
    currentProject?.availability !== "unavailable" &&
      (!currentProject?.repositoryId || workspaceEnabled) &&
      isLocationTargetReady(
        treeTargetMissing,
        kind,
        cwd,
        worktree,
        repository,
        worktreeReady,
      ),
  );
  // A worktree made on the way becomes the chosen location.
  function adoptTree(tree: Worktree) {
    createdTree = tree;
    repositoryId = tree.repositoryId;
    worktreeId = tree.id;
    kind = "worktree";
    onworktreecreated(tree);
    onchange();
  }

  async function create(makeDirectory = false) {
    if (busy) return;
    const onlyTree = treeOnly;
    const validationError = validateNewSessionSubmit(
      onlyTree,
      kind,
      commandReady,
    );
    if (validationError) {
      error = validationError;
      return;
    }
    busy = true;
    error = "";
    missingConnector = undefined;
    try {
      const outcome = await submitNewSessionRequest({
        onlyTree,
        kind,
        repositoryId,
        worktreeId,
        newPath,
        branchMode,
        branch,
        base,
        cwd,
        entryDirectory: projectSelection ? null : entryDirectory,
        prompt,
        hasInstruction,
        useInstruction,
        chosenConfig,
        name,
        startSide: chatEnabled ? startSide : "terminal",
        source,
        command,
        makeDirectory: filesEnabled && makeDirectory,
        recent,
        adoptTree,
      });
      if (outcome.kind === "tree") {
        onclose();
        return;
      }
      if (oncreated) oncreated(outcome.session, direction);
      else navigate("/s/" + outcome.session.id);
    } catch (err) {
      if (kind !== "directory") onchange();
      const conflict = sessionConflict(err);
      if (
        filesEnabled &&
        kind === "directory" &&
        !makeDirectory &&
        isDirectoryNotFoundError(err)
      ) {
        mkdirMounted = true;
        mkdirOpen = true;
      } else if (conflict?.reason === "connector_missing" && conflict.connector)
        missingConnector = conflict.connector;
      else error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<ResponsiveDialog
  title={treeOnly ? "新建工作树" : "新建会话"}
  description={treeOnly
    ? "创建独立的代码副本，可选择同时启动会话。"
    : projectEnabled
      ? "先选择项目，再设置运行方式。"
      : "浏览器断开后，会话仍会继续。"}
  class="p-5 sm:max-w-3xl"
  pinDrawerHeader
  {busy}
  {onclose}
  onback={back}
>
  {#if projectEnabled && providers.ProjectPicker && pickerMounted}
    <div hidden={!choosingProject}>
      <providers.ProjectPicker
        bind:this={projectPicker}
        {defaultCwd}
        {recent}
        onselect={selectedProject}
      />
    </div>
  {/if}
  <div hidden={projectEnabled && choosingProject}>
    <NewSessionForm
      {providers}
      {repositories}
      scope={projectScope}
      project={currentProject}
      onprojectchange={projectRestricted ? undefined : changeProject}
      {recent}
      {preferredRepositoryId}
      {createdTree}
      onremove={(tree) => (removing = tree)}
      bind:kind={
        () => kind,
        (next) => {
          kind = next;
          if (kind === "new-worktree") fieldsRepositoryId = repositoryId;
        }
      }
      bind:cwd
      bind:repositoryId={
        () => repositoryId,
        (next) => {
          repositoryId = next;
          if (kind === "new-worktree") fieldsRepositoryId = repositoryId;
        }
      }
      bind:worktreeId
      {busy}
      {fieldsRepository}
      bind:worktreeFields
      {entry}
      bind:newPath
      bind:branchMode
      bind:branch
      bind:base
      bind:startBranch
      {workspaceEnabled}
      {intent}
      bind:withSession
      {treeOnly}
      {launchConfigs}
      bind:source
      bind:configId
      bind:command
      bind:startSide
      onlaunchchoose={() => (launchTouched = true)}
      {lastLaunch}
      {instructionRepository}
      {hasInstruction}
      bind:useInstruction={
        () => useInstruction, (next) => (instructionChoice = next)
      }
      bind:prompt
      {startsOnChat}
      {split}
      bind:direction
      bind:name
      {error}
      {missingConnector}
      {targetReady}
      {commandReady}
      oncreate={() => void create()}
      onterminalcreate={() => {
        startSide = "terminal";
        void create();
      }}
      {onclose}
    />
  </div>
</ResponsiveDialog>

<NewSessionRecoveryDialogs
  {workspaceEnabled}
  {filesEnabled}
  {removing}
  {onchange}
  onremoved={removed}
  onremovalclose={() => (removing = null)}
  bind:mkdirOpen
  {mkdirMounted}
  {cwd}
  onmkdir={() => void create(true)}
/>
