import { m } from "$lib/paraglide/messages.js";
import type {
  AgentCandidate,
  AgentConnector,
  ChatEligibility,
  ChatState,
  ChatSupport,
  SessionInfo,
  SessionView,
  TransitionPhase,
} from "../../contracts/schemas";

// User-facing text for the server's chat support reason codes. The rules
// themselves stay on the server (agent profiles); only the wording lives here.
function supportReasonText(reason: NonNullable<ChatSupport["reason"]>): string {
  switch (reason) {
    case "unknown_agent":
      return m.s3_chat_support_unknown_agent();
    case "unsupported_form":
      return m.s3_chat_support_unsupported_form();
    case "session_store":
      return m.s3_chat_support_session_store();
    case "directory_arg":
      return m.s3_chat_support_directory_arg();
    case "non_interactive":
      return m.s3_chat_support_non_interactive();
    case "session_arg":
      return m.s3_chat_support_session_arg();
  }
}

export function chatSupportReason(support: ChatSupport | null) {
  if (!support || support.supported) return "";
  const reason = support.reason ?? "unknown_agent";
  if (reason === "unknown_agent" && support.supportedAgents.length > 0) {
    return m.s3_chat_support_unknown_agent_list({
      agents: support.supportedAgents.join(m.s3_list_separator()),
    });
  }
  return supportReasonText(reason);
}

export type HybridSideLabel = string;

/** Which view a session is on, as shown on cards and pages. */
export function sessionViewLabel(
  session: Pick<SessionInfo, "view" | "transition">,
): HybridSideLabel {
  if (session.transition) return m.s3_view_switching();
  if (session.view === "chat") return m.s3_view_chat();
  return m.s3_view_terminal();
}

/**
 * Loading title for the session page, from the list before that page's chunk
 * arrives. A chat-side session must not say it is loading a terminal. Unknown
 * sessions (a direct link before any list) stay neutral.
 */
export function sessionPageLabel(
  session: Pick<SessionInfo, "views" | "view" | "transition"> | undefined,
): string {
  if (!session) return m.s3_view_session();
  if (!session.views.includes("chat") || session.view === "terminal")
    return session.transition ? m.s3_view_session() : m.s3_view_terminal();
  if (session.transition) return m.s3_view_session();
  return m.s3_view_chat();
}

/** Why the chat side cannot start: its connector is not installed. */
export function connectorMissingText(connector: AgentConnector) {
  return m.s3_connector_missing({
    title: connector.title,
    executable: connector.executable,
  });
}

/** The notice that a hybrid config's chat side is installed separately. */
export function connectorNoticeText(connector: AgentConnector) {
  return m.s3_connector_notice({ executable: connector.executable });
}

/** A component running below the version WebMux was verified with. */
export function outdatedText(item: ChatState["outdated"][number]) {
  return m.s3_outdated_component({
    name: item.name,
    version: item.version,
    verified: item.verified,
  });
}

// Why switching to chat is unavailable, in the order the server checks the
// conditions (agent-session-identity); the server sends only the code.
function eligibilityReasonText(reason: string): string | undefined {
  switch (reason) {
    case "unknown_agent":
      return m.s3_chat_support_unknown_agent();
    case "not_root":
      return m.s3_elig_not_root();
    case "no_session_id":
      return m.s3_elig_no_session_id();
    case "not_listed":
      return m.s3_elig_not_listed();
    case "occupied":
      return m.s3_elig_occupied();
    case "occupied_external":
      return m.s3_elig_occupied_external();
    case "session_ambiguous":
      return m.s3_elig_session_ambiguous();
    case "checking":
      return m.s3_elig_checking();
    default:
      return undefined;
  }
}

/** The reason switching to chat is unavailable, or "" when it is. */
export function eligibilityText(eligibility: ChatEligibility) {
  if (eligibility.available) return "";
  const reason = eligibility.reason ?? "";
  if (reason === "connector_missing" && eligibility.connector)
    return connectorMissingText(eligibility.connector);
  return eligibilityReasonText(reason) ?? (reason || m.s3_elig_unavailable());
}

export type SwitchTarget = SessionView;

/**
 * Where a switch stands on this device: a server phase, "requested" before
 * the server reports one, or "arriving" once the terminal run has started
 * and the page waits for its first output.
 */
export type SwitchProgressPhase = TransitionPhase | "requested" | "arriving";

type StepPhase = Exclude<SwitchProgressPhase, "requested">;

function phaseLabel(phase: StepPhase): string {
  switch (phase) {
    case "interrupting":
    case "awaiting_choice":
    case "cancelling":
      return m.s3_phase_interrupting();
    case "stopping":
      return m.s3_phase_stopping();
    case "starting":
      return m.s3_phase_starting();
    case "loading":
      return m.s3_phase_loading();
    case "rolling_back":
      return m.s3_phase_rolling_back();
    case "arriving":
      return m.s3_phase_arriving();
  }
}
const stepOrder: Record<SwitchTarget, StepPhase[]> = {
  chat: ["interrupting", "stopping", "starting", "loading"],
  terminal: ["cancelling", "stopping", "starting", "arriving"],
};

/**
 * Which way a running switch goes: its phase names it for the steps only one
 * direction has, otherwise the view it left does. Null when neither tells.
 */
export function switchTarget(
  phase: TransitionPhase | undefined,
  from: SwitchTarget | undefined,
): SwitchTarget | null {
  if (phase === "interrupting" || phase === "awaiting_choice") return "chat";
  if (phase === "loading") return "chat";
  if (phase === "cancelling") return "terminal";
  if (from === "terminal") return "chat";
  if (from === "chat") return "terminal";
  return null;
}

export type SwitchStep = {
  label: string;
  state: "done" | "current" | "pending";
};

/**
 * The steps of a switch in progress, with the current phase marked; a
 * request the server has not reported yet stands at the first step. A phase
 * outside the direction's steps (a rollback, or an unknown direction) is
 * shown on its own.
 */
export function switchSteps(
  to: SwitchTarget | null,
  phase: SwitchProgressPhase,
): SwitchStep[] {
  const order = to ? stepOrder[to] : [];
  if (phase === "requested")
    return order.map((step, i) => ({
      label: phaseLabel(step),
      state: i === 0 ? "current" : "pending",
    }));
  const at = order.indexOf(
    phase === "awaiting_choice" ? "interrupting" : phase,
  );
  if (at < 0) return [{ label: phaseLabel(phase), state: "current" }];
  return order.map((step, i) => ({
    label: phaseLabel(step),
    state: i < at ? "done" : i === at ? "current" : "pending",
  }));
}

/** An agent session ID shortened for notices. */
export function shortId(id: string) {
  return id.length > 12 ? `${id.slice(0, 8)}…` : id;
}

/**
 * What the pane shows: the terminal (also for ordinary sessions), the chat
 * view, a switch in progress, or the resume choices of a session that is
 * gone but recoverable.
 */
export type HybridView = "terminal" | "chat" | "switching" | "recovery";

/** A dismissible page strip. */
export type HybridNotice = {
  key: string;
  tone: "info" | "warning" | "danger";
  text: string;
};

/** A switch that waits for the user to confirm interrupting a turn. */
export type HybridConfirm = { to: SwitchTarget; title: string; body: string };

/** A switch to chat that found several agent sessions to continue. */
export type HybridAmbiguous = {
  message: string;
  candidates: AgentCandidate[];
};

export const switchConfirmations: Record<
  SwitchTarget,
  Omit<HybridConfirm, "to">
> = {
  terminal: {
    get title() {
      return m.s3_switch_to_terminal_title();
    },
    get body() {
      return m.s3_switch_to_terminal_body();
    },
  },
  chat: {
    get title() {
      return m.s3_switch_to_chat_title();
    },
    get body() {
      return m.s3_switch_to_chat_body();
    },
  },
};

export function buildHybridNotices(
  state: ChatState | undefined,
  where: SessionView | undefined,
  error: string,
  notice: string,
  dropped: readonly string[],
  dismissed: ReadonlySet<string>,
): HybridNotice[] {
  const list: HybridNotice[] = [];
  const failure = state?.switchError || error;
  if (failure)
    list.push({ key: `failure:${failure}`, tone: "danger", text: failure });
  if (notice)
    list.push({ key: `notice:${notice}`, tone: "info", text: notice });
  if (dropped.length)
    list.push({
      key: `dropped:${dropped.join(" ")}`,
      tone: "warning",
      text: m.s3_hybrid_dropped_args({ args: dropped.join(" ") }),
    });
  const change = state?.idChange;
  if (change && where === "terminal")
    list.push({
      key: `id:${change.from}>${change.to}`,
      tone: "info",
      text: m.s3_hybrid_id_changed({
        to: shortId(change.to),
        from: shortId(change.from),
      }),
    });
  if (state?.queueLost)
    list.push({
      key: "queue-lost",
      tone: "warning",
      text: m.s3_hybrid_queue_lost({ count: state.queueLost }),
    });
  return list.filter((item) => !dismissed.has(item.key));
}

export function deriveHybridHeaderInfo(
  where: SessionView | undefined,
  fromServer: boolean,
  current: SessionInfo | undefined,
  terminalInfo: SessionInfo | undefined,
  state: ChatState | undefined,
): SessionInfo | undefined {
  const restInfo = fromServer ? current : undefined;
  if (where === "terminal" || !restInfo) return terminalInfo ?? current;
  const running = state?.turn.running ?? false;
  const status = running
    ? "running"
    : state?.requests.length
      ? "waiting"
      : restInfo.status === "running"
        ? "idle"
        : restInfo.status;
  return { ...restInfo, status } satisfies SessionInfo;
}
