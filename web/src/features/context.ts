import type { Component, Snippet } from "svelte";
import type {
  AgentConnector,
  AttentionSettings,
  ChatSupport,
  LaunchConfig,
  Repository,
  SessionView,
  Worktree,
} from "../contracts/schemas";
import type {
  LaunchConfigsState,
  StartupShell,
} from "../domains/settings/launch-configs";
import type {
  LaunchScope,
  LaunchSource,
  TargetKind,
} from "../domains/sessions/new-session-defaults";
import type { ProjectSelection } from "../domains/sessions/project-selection";
import type { WorktreePreset } from "../domains/sessions/worktree-preset";

export interface CreationLocationProps {
  repositories: readonly Repository[];
  recent: readonly string[];
  /** Listed first: the home page's or contextual entry's repository. */
  preferredRepositoryId: string;
  /** A tree created by this dialog that the repository list may lack. */
  createdTree: Worktree | null;
  newWorktreeSummary: string;
  onremove?: (tree: Worktree) => void;
  scope?: LaunchScope;
  kind: TargetKind;
  cwd: string;
  repositoryId: string;
  worktreeId: string;
  disabled?: boolean;
  projectScoped?: boolean;
}

export interface CreationContributions {
  ProjectPicker?: Component<
    {
      defaultCwd: string;
      recent?: readonly string[];
      onselect: (selection: ProjectSelection) => void;
    },
    { back: () => boolean; showProjects: () => void }
  >;

  LaunchPicker?: Component<
    {
      launchConfigs: LaunchConfigsState;
      source: LaunchSource;
      configId: string;
      command: string;
      startSide: SessionView;
      disabled?: boolean;
      onchoose: () => void;
    },
    {},
    "source" | "configId" | "command" | "startSide"
  >;
  LocationPicker?: Component<
    CreationLocationProps,
    {},
    "kind" | "cwd" | "repositoryId" | "worktreeId"
  >;
  WorktreeCreateFields?: Component<
    {
      repository: Repository;
      deferred?: boolean;
      path: string;
      mode: "new" | "existing";
      branch: string;
      base: string;
      startBranch?: string;
      disabled?: boolean;
      preferredStartBranch?: string;
      preset?: WorktreePreset;
    },
    { isReady: () => boolean },
    "path" | "mode" | "branch" | "base" | "startBranch"
  >;
  ConnectorInstallHint?: Component<{
    connector: AgentConnector;
    message?: string;
    detail?: string;
    tone?: "info" | "warning";
    actions?: Snippet;
    ondismiss?: () => void;
    class?: string;
  }>;
  DirectoryPicker?: Component<{ value: string }, {}, "value">;
  LaunchAttentionFields?: Component<
    {
      config: LaunchConfig | null;
      attention: AttentionSettings | null;
      attentionError?: string;
    },
    {},
    "attention"
  >;
  LaunchChatFields?: Component<
    {
      config: LaunchConfig | null;
      cmd: string[] | undefined;
      hybrid: boolean;
      disabled?: boolean;
    },
    {},
    "hybrid"
  >;
  LaunchStartSideFields?: Component<
    {
      startSide: SessionView;
      support: ChatSupport;
      disabled?: boolean;
    },
    {},
    "startSide"
  >;
}

export type SettingsContribution =
  | {
      id: "launch-configs";
      label: string;
      Component: Component<{ defaultShell?: "" | StartupShell }>;
    }
  | {
      id:
        | "code-reader"
        | "quick-commands"
        | "voice"
        | "agent-tools"
        | "access"
        | "tailscale"
        | "push"
        | "messaging";
      label: string;
      Component: Component;
    };

export interface FeatureContributions {
  creation: CreationContributions;
  settings: SettingsContribution[];
  network: {
    P2PSettings?: Component;
    ForwardSettings?: Component;
  };
}

// The host replaces this snapshot before mounting authenticated views and
// clears it when changing hosts. No implementation is imported by this module.
let contributions: FeatureContributions = {
  creation: {},
  settings: [],
  network: {},
};

export function getFeatureContributions(): FeatureContributions {
  return contributions;
}

export function setFeatureContributions(next: FeatureContributions): void {
  contributions = next;
}
