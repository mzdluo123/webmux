<script lang="ts">
  import { onMount, untrack } from "svelte";
  import NewSessionDialog from "../src/components/sessions/NewSessionDialog.svelte";
  import LocationPicker from "../src/components/workspace/LocationPicker.svelte";
  import ProjectPicker from "../src/components/workspace/ProjectPicker.svelte";
  import LaunchPicker from "../src/components/sessions/LaunchPicker.svelte";
  import { setPluginContributions } from "../src/plugins/context";
  import {
    setPluginCapabilities,
    resetPluginCapabilities,
  } from "../src/platform/plugin-host.svelte";
  import type { Repository } from "../src/contracts/schemas";
  import { worktree, longPath, longError } from "./fixtures";
  let {
    scene = "plain",
  }: { scene?: "plain" | "git" | "unavailable" | "launch-unavailable" } =
    $props();
  const initial = untrack(() => scene);
  let ready = $state(false);
  let open = $state(true);
  const repository: Repository = {
    id: worktree.repositoryId,
    name: "webmux",
    anchorPath: "/home/demo/projects/webmux",
    commonDir: "/home/demo/projects/webmux/.git",
    availability: initial === "unavailable" ? "unavailable" : "available",
    error: initial === "unavailable" ? longError : "",
    checkedAt: "2026-11-15T00:00:00Z",
    lastFetchedAt: null,
    initInstruction: "",
    worktrees: [
      {
        ...worktree,
        path: "/home/demo/projects/webmux",
        branch: "main",
        kind: "main",
        available: true,
        locked: false,
        prunable: false,
        error: "",
        operation: null,
      },
    ],
  };
  const git = initial === "git" || initial === "unavailable";
  onMount(() => {
    setPluginCapabilities({
      version: 1,
      plugins: [
        { id: "workspace", enabled: true, status: "ready" },
        { id: "launch", enabled: true, status: "ready" },
      ],
      capabilities: [
        { id: "workspace.projects", available: true },
        { id: "workspace.manage", available: true },
        { id: "launch.configs", available: true },
      ],
    });
    setPluginContributions({
      creation: { ProjectPicker, LocationPicker, LaunchPicker },
      settings: [],
      network: {},
    });
    ready = true;
    return () => {
      setPluginContributions({ creation: {}, settings: [], network: {} });
      resetPluginCapabilities();
    };
  });
</script>

{#if ready && open}
  <NewSessionDialog
    defaultCwd="/home/demo/projects"
    repositories={git ? [repository] : []}
    entry={git
      ? {
          kind: "worktree",
          repositoryId: repository.id,
          worktreeId: worktree.id,
        }
      : { kind: "directory", cwd: longPath, workspace: null }}
    scope={git ? { repositoryId: repository.id } : undefined}
    launchConfigs={initial === "launch-unavailable"
      ? { status: "unavailable", error: "启动配置存储不可用，可填写自定义命令" }
      : {
          status: "ready",
          configs: [
            {
              id: "abcdefgh",
              name: "Shell",
              cmd: ["/bin/sh", "-i"],
              envNames: [],
              chat: false,
              chatSupport: { supported: false, supportedAgents: [] },
              chatOptions: [],
              createdAt: "2026-11-15T00:00:00Z",
              updatedAt: "2026-11-15T00:00:00Z",
            },
          ],
        }}
    oncreated={() => (open = false)}
    onclose={() => (open = false)}
    onchange={() => {}}
    onworktreecreated={() => {}}
  />
{/if}
