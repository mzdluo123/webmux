import type {
  ChatAttachment,
  ChatContent,
  ChatEvent,
  ChatFrame,
  ChatOption,
  ChatRequest,
  StopReason,
  ToolStatus,
} from "../../contracts/schemas";
import { diffStats, summarizeTurnDiff, type TurnDiffState } from "./chat-diff";

export { diffStats };

// Pure reducer from the chat channel's frames to the timeline the chat view
// renders. Non-timeline state (plan, config, queue, requests...) arrives as
// whole `state` objects and is not tracked here.

type MessageEntry = {
  type: "message";
  id: string;
  role: "user" | "agent" | "thought";
  text: string;
  truncated: boolean;
  attachments?: ChatAttachment[];
};
export type ToolEntry = {
  type: "tool";
  id: string;
  kind: string;
  title: string;
  status: ToolStatus;
  content: ChatContent[];
  locations: { path: string; line?: number }[];
  /** Owning live turn; late patches keep it until an explicit tool restart. */
  diffTurnId?: string;
  /** Created by a patch whose opening event fell out of the cached log. */
  partial: boolean;
  /**
   * Outcome of the permission request attached to this call, if answered;
   * the chosen option's name and kind when the request was seen.
   */
  permission?: {
    outcome: string;
    optionId?: string;
    name?: string;
    kind?: ChatOption["kind"];
  };
};
export type TimelineEntry =
  | MessageEntry
  | ToolEntry
  | {
      type: "compaction";
      id: string;
      status: "running" | "completed" | "failed";
      summary: string;
    }
  | {
      type: "notice";
      id: string;
      level: "info" | "warning" | "error";
      text: string;
    }
  | {
      type: "turnEnd";
      id: string;
      stopReason: StopReason;
      error?: string;
      changes?: readonly ChatContent[];
    }
  | {
      type: "marker";
      id: string;
      kind: "loaded" | "switched" | "truncated" | "reconnected";
      view?: string;
    };

export type ChatModel = {
  generation: string;
  /** Sequence expected next; events below it were already applied. */
  next: number;
  /** Older events were dropped by the server's byte limit. */
  truncated: boolean;
  title?: string;
  entries: TimelineEntry[];
  /** Entry position by "type:id", for upserts. */
  index: ReadonlyMap<string, number>;
  /** Diff-bearing tools from the current turn, ordered by latest content update. */
  turnDiff?: TurnDiffState;
  /** Tool call and options of each permission request on a tool call. */
  requestTools: ReadonlyMap<
    string,
    { toolCallId: string; options: readonly ChatOption[] }
  >;
};

export function emptyChat(): ChatModel {
  return {
    generation: "",
    next: 0,
    truncated: false,
    entries: [],
    index: new Map(),
    requestTools: new Map(),
  };
}

/**
 * Where to resume the stream: the generation and the last applied sequence,
 * after which the server resends events.
 */
export function resumePoint(model: ChatModel) {
  return model.generation
    ? { generation: model.generation, seq: Math.max(0, model.next - 1) }
    : undefined;
}

export function applyFrame(model: ChatModel, frame: ChatFrame): ChatModel {
  switch (frame.type) {
    case "snapshot": {
      const generation = frame.generation ?? "";
      const keep = !frame.reset && generation === model.generation;
      const base = keep ? model : { ...emptyChat(), generation };
      return {
        ...base,
        next: keep ? model.next : (frame.from ?? 0),
        truncated: !!frame.truncated,
      };
    }
    case "events": {
      if (frame.generation && frame.generation !== model.generation)
        return model;
      const items = frame.items;
      if (!items?.length) return model;
      return applyEventsBatch(model, items);
    }
    case "state":
      return model;
  }
}

type BatchDraft = {
  next: number;
  title: string | undefined;
  entries: TimelineEntry[];
  entriesCloned: boolean;
  ownedEntries: Set<number>;
  index: Map<string, number> | ReadonlyMap<string, number>;
  indexCloned: boolean;
  turnDiff: ChatModel["turnDiff"];
  turnToolsCloned: boolean;
  requestTools:
    | Map<string, { toolCallId: string; options: readonly ChatOption[] }>
    | ChatModel["requestTools"];
  requestToolsCloned: boolean;
  changed: boolean;
};

function applyEventsBatch(
  model: ChatModel,
  items: readonly ChatEvent[],
): ChatModel {
  const draft: BatchDraft = {
    next: model.next,
    title: model.title,
    entries: model.entries,
    entriesCloned: false,
    ownedEntries: new Set(),
    index: model.index,
    indexCloned: false,
    turnDiff: model.turnDiff,
    turnToolsCloned: false,
    requestTools: model.requestTools,
    requestToolsCloned: false,
    changed: false,
  };
  for (const event of items) applyEventInto(draft, event);
  if (!draft.changed) return model;
  return {
    ...model,
    next: draft.next,
    title: draft.title,
    entries: draft.entries,
    index: draft.index,
    turnDiff: draft.turnDiff,
    requestTools: draft.requestTools,
  };
}

function ensureEntries(draft: BatchDraft): TimelineEntry[] {
  if (!draft.entriesCloned) {
    draft.entries = draft.entries.slice();
    draft.entriesCloned = true;
  }
  return draft.entries;
}

function ensureIndex(draft: BatchDraft): Map<string, number> {
  if (!draft.indexCloned) {
    draft.index = new Map(draft.index);
    draft.indexCloned = true;
  }
  return draft.index as Map<string, number>;
}

function upsertInto(
  draft: BatchDraft,
  key: string,
  build: (prev: TimelineEntry | undefined, owned: boolean) => TimelineEntry,
): TimelineEntry {
  const at = draft.index.get(key);
  const entries = ensureEntries(draft);
  if (at !== undefined) {
    const entry = build(entries[at], draft.ownedEntries.has(at));
    entries[at] = entry;
    draft.ownedEntries.add(at);
    return entry;
  }
  const pos = entries.length;
  const entry = build(undefined, true);
  ensureIndex(draft).set(key, pos);
  entries.push(entry);
  draft.ownedEntries.add(pos);
  return entry;
}

/** Applies one event into a frame-scoped draft; stale sequences are ignored. */
function applyEventInto(draft: BatchDraft, event: ChatEvent) {
  if (event.seq < draft.next) return;
  draft.changed = true;
  draft.next = event.seq + 1;
  switch (event.kind) {
    case "message.append": {
      const { messageId, role, text, truncated, attachments } = event.message;
      upsertInto(draft, `message:${messageId}`, (prev, owned) => {
        if (prev?.type === "message") {
          if (owned) {
            prev.text += text;
            if (truncated) prev.truncated = true;
            if (attachments !== undefined) prev.attachments = attachments;
            return prev;
          }
          return {
            ...prev,
            text: prev.text + text,
            truncated: prev.truncated || !!truncated,
            attachments: attachments ?? prev.attachments,
          };
        }
        return {
          type: "message",
          id: messageId,
          role,
          text,
          truncated: !!truncated,
          attachments,
        };
      });
      return;
    }
    case "tool.upsert": {
      const patch = event.tool;
      const updated = upsertInto(draft, `tool:${patch.toolCallId}`, (prev) => {
        const base: ToolEntry =
          prev?.type === "tool"
            ? prev
            : {
                type: "tool",
                id: patch.toolCallId,
                kind: "other",
                title: "",
                status: "pending",
                content: [],
                locations: [],
                partial: patch.title === undefined,
                diffTurnId: draft.turnDiff?.id,
              };
        return {
          ...base,
          kind: patch.kind ?? base.kind,
          title: patch.title ?? base.title,
          status: patch.status ?? base.status,
          content: patch.content ?? base.content,
          locations: patch.locations ?? base.locations,
          diffTurnId:
            (patch.status === "pending" || patch.status === "in_progress") &&
            base.status !== "pending" &&
            base.status !== "in_progress"
              ? draft.turnDiff?.id
              : base.diffTurnId,
        };
      });
      if (updated.type === "tool")
        trackTurnDiffInto(draft, patch, updated.status, updated.diffTurnId);
      return;
    }
    case "info.set":
      if (event.info.title !== undefined) draft.title = event.info.title;
      return;
    case "compaction.upsert": {
      const { id, status, summary } = event.compaction;
      upsertInto(draft, `compaction:${id}`, (prev) => ({
        type: "compaction",
        id,
        status,
        summary: summary ?? (prev?.type === "compaction" ? prev.summary : ""),
      }));
      return;
    }
    case "notice":
      ensureEntries(draft).push({
        type: "notice",
        id: String(event.seq),
        level: event.notice.level,
        text: event.notice.text,
      });
      return;
    case "turn.start":
      draft.turnDiff = { id: event.turnStart.turnId, tools: new Map() };
      draft.turnToolsCloned = true;
      return;
    case "turn.end": {
      const id = event.turnEnd.turnId;
      const turn = draft.turnDiff?.id === id ? draft.turnDiff : undefined;
      const changes = turn && summarizeTurnDiff(turn);
      if (turn) {
        draft.turnDiff = undefined;
        draft.turnToolsCloned = false;
      }
      ensureEntries(draft).push({
        type: "turnEnd",
        id,
        stopReason: event.turnEnd.stopReason,
        ...(event.turnEnd.error ? { error: event.turnEnd.error } : {}),
        ...(changes?.length ? { changes } : {}),
      });
      return;
    }
    case "marker":
      ensureEntries(draft).push({
        type: "marker",
        id: String(event.seq),
        kind: event.marker.kind,
        ...(event.marker.view ? { view: event.marker.view } : {}),
      });
      return;
    case "request.open":
      trackRequestInto(draft, event.request);
      return;
    case "request.close": {
      const { requestId, outcome, optionId } = event.resolved;
      const request = draft.requestTools.get(requestId);
      const at = request
        ? draft.index.get(`tool:${request.toolCallId}`)
        : undefined;
      const tool = at === undefined ? undefined : draft.entries[at];
      if (tool?.type !== "tool") return;
      const option = request?.options.find((o) => o.optionId === optionId);
      const entries = ensureEntries(draft);
      entries[at!] = {
        ...tool,
        permission: {
          outcome,
          ...(optionId ? { optionId } : {}),
          ...(option ? { name: option.name, kind: option.kind } : {}),
        },
      };
      draft.ownedEntries.add(at!);
      return;
    }
    case "plan.set":
    case "commands.set":
    case "config.set":
    case "usage.set":
      // State frames carry these; the timeline does not show them.
      return;
  }
}

function trackTurnDiffInto(
  draft: BatchDraft,
  patch: Extract<ChatEvent, { kind: "tool.upsert" }>["tool"],
  status: ToolStatus,
  owner: string | undefined,
) {
  const turn = draft.turnDiff;
  if (!turn || turn.id !== owner) return;
  const previous = turn.tools.get(patch.toolCallId);
  const diffs = patch.content
    ? patch.content.filter((block) => block.type === "diff" && !!block.path)
    : previous?.diffs;
  if (
    (!previous && !diffs?.length) ||
    (previous && !patch.content && status === previous.status)
  )
    return;
  let tools: Map<string, { status: ToolStatus; diffs: ChatContent[] }>;
  if (draft.turnToolsCloned) {
    tools = turn.tools as Map<
      string,
      { status: ToolStatus; diffs: ChatContent[] }
    >;
  } else {
    tools = new Map(turn.tools);
    draft.turnDiff = { id: turn.id, tools };
    draft.turnToolsCloned = true;
  }
  if (patch.content) tools.delete(patch.toolCallId);
  if (diffs?.length) tools.set(patch.toolCallId, { status, diffs });
}

function trackRequestInto(draft: BatchDraft, request: ChatRequest) {
  if (!request.toolCallId) return;
  let requestTools: Map<
    string,
    { toolCallId: string; options: readonly ChatOption[] }
  >;
  if (draft.requestToolsCloned) {
    requestTools = draft.requestTools as Map<
      string,
      { toolCallId: string; options: readonly ChatOption[] }
    >;
  } else {
    requestTools = new Map(draft.requestTools);
    draft.requestTools = requestTools;
    draft.requestToolsCloned = true;
  }
  requestTools.set(request.requestId, {
    toolCallId: request.toolCallId,
    options: request.options ?? [],
  });
}

export type WorkStep = ToolEntry | (MessageEntry & { role: "thought" });
type WorkEntry = {
  type: "work";
  id: string;
  steps: WorkStep[];
  calls: ToolEntry[];
};
export type DisplayEntry = TimelineEntry | WorkEntry;

const isWorkStep = (entry: TimelineEntry): entry is WorkStep =>
  entry.type === "tool" ||
  (entry.type === "message" && entry.role === "thought");

// Preserves object identity for unchanged completed work runs across streaming
// updates so Svelte skips re-evaluating finished tool groups.
const workGroupCache = new WeakMap<WorkStep, WorkEntry>();

/**
 * Folds each run of the agent's work (thinking and tool calls) into one
 * "work" entry, keyed by its first step so the group keeps its identity while
 * the run grows. Anything else (an agent reply, a plan, a notice, the user's
 * next message) ends the run, so the transcript order never changes.
 */
export function groupTimeline(entries: readonly TimelineEntry[]) {
  const out: DisplayEntry[] = [];
  let run: WorkStep[] = [];
  const flush = () => {
    if (run.length) {
      const first = run[0]!;
      const cached = workGroupCache.get(first);
      if (
        cached &&
        cached.steps.length === run.length &&
        cached.steps.every((step, i) => step === run[i])
      ) {
        out.push(cached);
      } else {
        const group: WorkEntry = {
          type: "work",
          id: first.id,
          steps: run,
          calls: run.filter((step) => step.type === "tool"),
        };
        workGroupCache.set(first, group);
        out.push(group);
      }
    }
    run = [];
  };
  for (const entry of entries) {
    if (isWorkStep(entry)) {
      run.push(entry);
      continue;
    }
    flush();
    out.push(entry);
  }
  flush();
  return out;
}
