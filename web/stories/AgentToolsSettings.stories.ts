import type { Meta, StoryObj } from "@storybook/svelte-vite";
import AgentToolsSettings from "../src/components/settings/AgentToolsSettings.svelte";
import type { AgentToolsReport } from "../src/contracts/schemas";

const baseTools: AgentToolsReport["tools"] = [
  {
    id: "claudecode",
    title: "Claude Code",
    icon: "claudecode",
    kind: "hybrid",
    installed: true,
    executable: "claude",
    executables: ["claude"],
    path: "/home/dev/.local/bin/claude",
    installedVersion: "2.1.300",
    verifiedVersion: "2.1.316",
    belowVerified: true,
    updateStatus: "unchecked",
    installCommand: "npm install -g @anthropic-ai/claude-code",
    upgradeCommand: "claude update",
    docsUrl: "https://code.claude.com/docs/en/overview",
    connector: {
      executable: "claude-agent-acp",
      title: "Claude Code ACP 适配器",
      installed: true,
      path: "/home/dev/.local/share/pnpm/claude-agent-acp",
      installedVersion: "0.99.1",
      verifiedVersion: "0.99.1",
      updateStatus: "unchecked",
      installCommand: "npm install -g @agentclientprotocol/claude-agent-acp",
      upgradeCommand:
        "npm install -g @agentclientprotocol/claude-agent-acp@latest",
      docsUrl: "https://github.com/agentclientprotocol/claude-agent-acp",
    },
  },
  {
    id: "codex",
    title: "Codex CLI",
    icon: "codex",
    kind: "hybrid",
    installed: true,
    executable: "codex",
    executables: ["codex"],
    path: "/usr/local/bin/codex",
    installedVersion: "0.165.0",
    verifiedVersion: "0.165.0",
    updateStatus: "unchecked",
    installCommand: "npm install -g @openai/codex",
    upgradeCommand: "npm install -g @openai/codex@latest",
    docsUrl: "https://developers.openai.com/codex/cli/",
    connector: {
      executable: "codex-acp",
      title: "Codex ACP 适配器",
      installed: false,
      verifiedVersion: "2.3.0",
      updateStatus: "not_installed",
      installCommand: "npm install -g @agentclientprotocol/codex-acp",
      upgradeCommand: "npm install -g @agentclientprotocol/codex-acp@latest",
      docsUrl: "https://github.com/agentclientprotocol/codex-acp",
    },
  },
  {
    id: "agy",
    title: "Antigravity CLI",
    icon: "antigravity",
    kind: "hybrid",
    installed: true,
    executable: "agy",
    executables: ["agy"],
    path: "/opt/antigravity/current/resources/app/extensions/very-long-install-directory-for-narrow-viewport-check/bin/agy",
    installedVersion: "1.4.12",
    verifiedVersion: "1.4.12",
    updateStatus: "unchecked",
    upgradeCommand: "agy update",
    docsUrl: "https://antigravity.google/docs",
  },
  {
    id: "opencode",
    title: "OpenCode",
    icon: "opencode",
    kind: "hybrid",
    installed: false,
    executable: "opencode",
    executables: ["opencode"],
    updateStatus: "not_installed",
    installCommand: "npm install -g opencode-ai",
    upgradeCommand: "opencode upgrade",
    docsUrl: "https://opencode.ai/docs/",
  },
  {
    id: "copilot",
    title: "GitHub Copilot CLI",
    icon: "githubcopilot",
    kind: "terminal",
    installed: false,
    executable: "copilot",
    executables: ["copilot"],
    updateStatus: "not_installed",
    installCommand: "npm install -g @github/copilot",
    upgradeCommand: "npm install -g @github/copilot@latest",
    docsUrl:
      "https://docs.github.com/en/copilot/how-tos/copilot-cli/use-copilot-cli/overview",
  },
];

const localReport: AgentToolsReport = {
  checkedAt: "2026-04-15T08:30:00Z",
  shell: "/bin/bash",
  tools: baseTools,
};

const checkedReport: AgentToolsReport = {
  checkedAt: "2026-04-15T08:30:00Z",
  remoteCheckedAt: "2026-04-15T08:31:00Z",
  shell: "/bin/bash",
  tools: baseTools.map((tool) => {
    if (tool.id === "claudecode") {
      return {
        ...tool,
        latestVersion: "2.1.320",
        updateStatus: "update_available",
        connector: tool.connector
          ? {
              ...tool.connector,
              latestVersion: "0.99.1",
              updateStatus: "up_to_date",
            }
          : undefined,
      };
    }
    if (tool.id === "codex") {
      return {
        ...tool,
        latestVersion: "0.165.0",
        updateStatus: "up_to_date",
      };
    }
    if (tool.id === "agy") {
      return {
        ...tool,
        updateStatus: "source_unavailable",
      };
    }
    return tool;
  }),
};

const meta = {
  title: "Settings/Agent Tools",
  component: AgentToolsSettings,
  args: {
    fixture: {
      report: localReport,
      state: "ready",
    },
  },
} satisfies Meta<typeof AgentToolsSettings>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Loading: Story = {
  args: {
    fixture: {
      state: "loading",
    },
  },
};

export const LocalScanned: Story = {};

export const UpdatesAvailable: Story = {
  args: {
    fixture: {
      report: checkedReport,
      state: "ready",
    },
  },
};

export const EmptyInstalled: Story = {
  args: {
    fixture: {
      report: {
        checkedAt: "2026-04-15T08:30:00Z",
        shell: "/bin/bash",
        tools: baseTools.filter((tool) => !tool.installed),
      },
      state: "ready",
    },
  },
};

export const Unavailable: Story = {
  args: {
    fixture: {
      state: "unavailable",
      error: "startup shell probe timed out",
    },
  },
};

export const Phone: Story = {
  ...UpdatesAvailable,
  globals: { viewport: { value: "mobile1", isRotated: false } },
};

export const Tablet: Story = {
  ...UpdatesAvailable,
  globals: { viewport: { value: "tablet", isRotated: false } },
};
