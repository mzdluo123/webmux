import type {
  ChatAttachment,
  ChatContent,
  ChatOption,
  StopReason,
  ToolStatus,
} from "../../contracts/schemas";
import type { TurnDiffState } from "./chat-diff";

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
  /** True while earlier tail-first backfill chunks are still in flight. */
  backfillPending?: boolean;
};

export type WorkStep = ToolEntry | (MessageEntry & { role: "thought" });
export type WorkEntry = {
  type: "work";
  id: string;
  steps: WorkStep[];
  calls: ToolEntry[];
};
export type DisplayEntry = TimelineEntry | WorkEntry;
