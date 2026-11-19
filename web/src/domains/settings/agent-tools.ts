import type {
  AgentToolStatus,
  AgentToolsReport,
  LaunchConfigSummary,
} from "../../contracts/schemas";
import {
  isLaunchConfigHiddenByAgentInstall,
  matchLaunchConfigTool,
} from "./launch-configs";

export type MergedLaunchRow =
  | {
      key: string;
      kind: "config";
      config: LaunchConfigSummary;
      tool?: AgentToolStatus;
      showToolDetails: boolean;
    }
  | {
      key: string;
      kind: "tool";
      tool: AgentToolStatus;
    };

/**
 * Merges saved launch configs with local CLI agent tool statuses:
 * - Installed or non-agent launch configs stay in `active`.
 * - Uninstalled cataloged agent launch configs fold into `uninstalled`.
 * - Cataloged tools without a matching saved launch config join `active` when
 *   installed (or when their ACP connector is installed) and `uninstalled`
 *   otherwise, so every tool appears at most once.
 */
export function partitionLaunchAndAgentRows(
  configs: readonly LaunchConfigSummary[],
  tools: readonly AgentToolStatus[] | undefined,
): { active: MergedLaunchRow[]; uninstalled: MergedLaunchRow[] } {
  const representedToolIds = new Set<string>();
  const detailedToolIds = new Set<string>();
  const active: MergedLaunchRow[] = [];
  const uninstalled: MergedLaunchRow[] = [];

  for (const config of configs) {
    const tool = tools ? matchLaunchConfigTool(config, tools) : undefined;
    if (tool) representedToolIds.add(tool.id);
    const hidden =
      isLaunchConfigHiddenByAgentInstall(config, tools) &&
      !tool?.connector?.installed;
    const firstForTool = tool ? !detailedToolIds.has(tool.id) : false;
    if (tool && firstForTool) detailedToolIds.add(tool.id);
    const entry: MergedLaunchRow = {
      key: `config:${config.id}`,
      kind: "config",
      config,
      tool,
      showToolDetails: firstForTool,
    };
    if (hidden) uninstalled.push(entry);
    else active.push(entry);
  }

  for (const tool of tools ?? []) {
    if (representedToolIds.has(tool.id)) continue;
    if (tool.installed || Boolean(tool.connector?.installed)) {
      active.push({ key: `tool:${tool.id}`, kind: "tool", tool });
    } else {
      uninstalled.push({ key: `tool:${tool.id}`, kind: "tool", tool });
    }
  }

  return { active, uninstalled };
}

export function summarizeAgentTools(report: AgentToolsReport | undefined): {
  installedCount: number;
  updatesCount: number;
  hasCheckableTargets: boolean;
} {
  const tools = report?.tools ?? [];
  let installedCount = 0;
  let updatesCount = 0;
  let hasCheckableTargets = false;
  for (const tool of tools) {
    if (tool.installed) installedCount++;
    if (tool.installed || Boolean(tool.connector?.installed)) {
      hasCheckableTargets = true;
    }
    if (tool.updateStatus === "update_available") updatesCount++;
    if (tool.connector?.updateStatus === "update_available") updatesCount++;
  }
  return { installedCount, updatesCount, hasCheckableTargets };
}
