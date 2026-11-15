import { untrack } from "svelte";
import { api } from "../../platform/api";
import type { ConfigValue } from "../../platform/chat-api";
import {
  LaunchOptionsRequestSchema,
  LaunchOptionsSchema,
  type ChatConfig as ChatOption,
  type LaunchConfigSummary,
  type Repository,
} from "../../contracts/schemas";

type Probe = { agentId: string; options?: ChatOption[]; done: boolean };

export function useQuickStartAgentOptions(
  getAgent: () => LaunchConfigSummary | undefined,
  getRepository: () => Repository | undefined,
  getEdits: () => Record<string, Record<string, ConfigValue>>,
) {
  let probe = $state<Probe>();
  const agent = $derived(getAgent());
  const agentId = $derived(agent?.id);
  const repositoryKnown = $derived(!!getRepository());
  $effect(() => {
    const id = agentId;
    if (!id || !repositoryKnown) return;
    const repo = untrack(() => getRepository()?.id) ?? "";
    probe = { agentId: id, done: false };
    const controller = new AbortController();
    api(
      `/launch-configs/${id}/options`,
      LaunchOptionsSchema,
      "POST",
      { repositoryId: repo },
      controller.signal,
      LaunchOptionsRequestSchema,
    )
      .then((options) => (probe = { agentId: id, options, done: true }))
      .catch(() => {
        if (!controller.signal.aborted) probe = { agentId: id, done: true };
      });
    return () => controller.abort();
  });
  const probing = $derived(
    !!agent && probe?.agentId === agent.id && !probe.done,
  );
  // The page's option values: the agent's catalog plus edits.
  const catalog = $derived(
    probe?.agentId === agent?.id && probe?.options
      ? probe.options
      : (agent?.chatOptions ?? []),
  );
  const options = $derived<ChatOption[]>(
    catalog.map((option) => {
      const edit = agent ? getEdits()[agent.id]?.[option.id] : undefined;
      return edit ? { ...option, ...edit } : option;
    }),
  );
  return {
    get probing() {
      return probing;
    },
    get options() {
      return options;
    },
  };
}
