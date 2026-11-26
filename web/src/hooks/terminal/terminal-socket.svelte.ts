import { SvelteMap } from "svelte/reactivity";
import { inflateSync } from "fflate";
import type { Terminal } from "@xterm/xterm";
import {
  ControlStateSchema,
  ExitSchema,
  SessionInfoSchema,
  StreamPositionSchema,
  SubmissionStatusSchema,
  type ControlState,
  type SessionInfo,
  type StreamPosition,
  type SubmissionProgress,
} from "../../contracts/schemas";
import {
  clientIdentity,
  createHoverCoalescer,
  encodeIdsQuery,
  encodeSubmission,
  encodeTerminalSize,
  formatTerminalSocketPath,
  newSubmissionId,
  preparePaste,
  resolveAppendOverlap,
  trimOverlapChunk,
} from "../../domains/terminal/terminal-protocol";
import { connect } from "../../platform/ws";
import { wsCompression } from "../../platform/ws-compression";
import type { Connection, SocketConnection } from "../../platform/ws-contracts";
import { createOutputBudget } from "../../domains/terminal/terminal-output";
import {
  resetBrowserChrome,
  setBrowserChrome,
} from "../../platform/browser-chrome";
import {
  attachTerminalGraphics,
  type TerminalGraphics,
} from "./terminal-graphics";
import {
  registerTerminalProtocolHandlers,
  restoreTerminalSnapshot,
} from "./terminal-sync";
import type {
  TerminalSocketAttachment,
  TerminalSocketView,
} from "./terminal-socket-view";
import { createTerminalInputError } from "./terminal-input-error";

// Terminal stream of one session (`/ws/s/{id}`, docs/protocol/websocket.md).
export function createTerminalSocket(id: string, manageChrome = true) {
  let info = $state<SessionInfo>();
  let connection = $state<Connection>("connecting");
  let restoring = $state(true);
  let exitCode = $state<number | null>(null);
  const inputError = createTerminalInputError();
  let error = $state(inputError.capture(""));
  let historyTruncated = $state(false);
  let control = $state<ControlState>();
  // The run whose output this page has drawn: a hybrid switch to the
  // terminal waits for the new run's first bytes before showing the pane.
  let drawn = $state<string>();
  let terminal: Terminal | undefined;
  let socket: SocketConnection | undefined;
  let replaying = false;
  const encoder = new TextEncoder();
  const decoder = new TextDecoder();
  const identity = clientIdentity();
  let generation = $state<string>();
  let offset = 0;
  let synced = $state(false);
  const controlling = () =>
    !!control && control.controller?.clientId === identity.id;
  // Unconfirmed submissions, re-queried after every reconnect.
  const submissions = new SvelteMap<
    string,
    (state: SubmissionProgress) => void
  >();
  const accepting = () =>
    connection === "connected" &&
    !restoring &&
    exitCode === null &&
    (generation === undefined || synced);
  const submission = () => {
    const run = generation ?? info?.generation;
    if (!terminal || !socket || !run || !accepting()) return undefined;
    return { term: terminal, ws: socket, run };
  };
  const hover = createHoverCoalescer((report) => {
    if (!replaying && accepting() && controlling())
      socket?.send(6, encoder.encode(report));
  });

  function attach(
    term: Terminal,
    view: TerminalSocketView,
  ): TerminalSocketAttachment {
    let disposed = false;
    let stale = false;
    let exited = false;
    let restoreFrame = 0;
    let restoreVersion = 0;
    terminal = term;
    replaying = false;
    let graphics: TerminalGraphics | undefined;
    const writeOutput = (data: Uint8Array, parsed: () => void) => {
      if (graphics) graphics.write(data, parsed);
      else term.write(data, parsed);
    };
    const output = createOutputBudget(
      (data, parsed) => writeOutput(data, parsed),
      () => {
        // A scheduled reconnect already brings a fresh snapshot.
        if (connection !== "connected") return;
        if (!ws.send(2, new Uint8Array())) ws.reconnect();
      },
    );
    const restore = (payload: Uint8Array, run: string | undefined) => {
      output.replay();
      const releaseGraphics = graphics?.reset();
      const version = ++restoreVersion;
      cancelAnimationFrame(restoreFrame);
      restoreTerminalSnapshot(
        {
          term,
          graphics,
          writeOutput,
          resetFind: () => view.resetFind(),
          scrollToBottom: () => view.scrollToBottom(),
          isDisposed: () => disposed,
          isCurrent: (v) => v === restoreVersion,
          setReplaying: (v) => (replaying = v),
          setRestoring: (v) => (restoring = v),
          setDrawn: (v) => (drawn = v),
          isRecovering: () => output.recovering,
        },
        payload,
        run,
        version,
        releaseGraphics,
        (id) => (restoreFrame = id),
      );
    };
    let sentSize = "";
    const sendSize = () => {
      const size = `${term.cols}:${term.rows}`;
      if (
        controlling() &&
        view.isVisible() &&
        term.cols > 0 &&
        term.rows > 0 &&
        sentSize !== size &&
        ws.send(1, encodeTerminalSize(term.cols, term.rows))
      )
        sentSize = size;
    };
    // DEC mode 2031 reports: CSI ? 997 ; 1 n is dark, 2 is light.
    const root = document.documentElement;
    const reportScheme = () => {
      const dark = root.classList.contains("dark");
      term.input(`\x1b[?997;${dark ? 1 : 2}n`, false);
    };
    let resetPosition: StreamPosition | undefined;
    let appending = false;
    let skipOverlap = 0;
    let schemeReports = false;
    const ws = connect(
      () =>
        formatTerminalSocketPath(
          id,
          identity,
          synced,
          generation,
          offset,
          wsCompression(),
        ),
      (data) => {
        if (typeof data === "string") return;
        let frame = new Uint8Array(data);
        if (frame[0] === 7) frame = inflateSync(frame.subarray(1));
        let payload = frame.subarray(1);
        if (frame[0] === 0) {
          const trimmed = trimOverlapChunk(payload, skipOverlap);
          skipOverlap = trimmed.remainingSkip;
          if (!trimmed.slice) return;
          payload = trimmed.slice;
          const viewportY =
            appending && view.preserveViewport()
              ? term.buffer.active.viewportY
              : undefined;
          if (document.hidden || stale) stale = true;
          else if (output.write(payload)) {
            offset += payload.length;
            const run = generation ?? info?.generation;
            if (run && drawn !== run) drawn = run;
            if (appending) {
              // Searches keep their viewport; ordinary resumes show the tail.
              appending = false;
              term.write("", () => {
                if (disposed) return;
                if (viewportY !== undefined) term.scrollToLine(viewportY);
                else if (!view.preserveViewport()) view.scrollToBottom();
              });
            }
          } else {
            stale = true;
            restoring = true;
          }
        } else if (frame[0] === 4) {
          const position = StreamPositionSchema.parse(
            JSON.parse(decoder.decode(payload)),
          );
          historyTruncated = position.truncated;
          resetPosition = undefined;
          appending = false;
          skipOverlap = 0;
          if (position.mode === "reset") resetPosition = position;
          else {
            const decision = resolveAppendOverlap(
              synced,
              generation,
              offset,
              position,
            );
            if (decision.accept) {
              skipOverlap = decision.skipBytes;
              output.replay();
              stale = false;
              restoring = false;
              appending = decision.hasTail;
              if (!appending && !view.preserveViewport()) view.scrollToBottom();
            } else {
              synced = false;
              restoring = true;
              ws.send(2, new Uint8Array());
            }
          }
        } else if (frame[0] === 1) {
          const position = resetPosition;
          resetPosition = undefined;
          // A hidden page keeps its terminal and resume point unchanged.
          if (document.hidden) stale = true;
          else {
            stale = false;
            restore(payload, position?.generation ?? info?.generation);
            synced = !!position;
            generation = position?.generation;
            offset = position?.end ?? 0;
          }
        } else if (frame[0] === 5) {
          const status = SubmissionStatusSchema.parse(
            JSON.parse(decoder.decode(payload)),
          );
          submissions.get(status.id)?.(status.state);
          if (status.state !== "received") submissions.delete(status.id);
        } else if (frame[0] === 6) {
          const wasControlling = controlling();
          control = ControlStateSchema.parse(
            JSON.parse(decoder.decode(payload)),
          );
          if (!wasControlling) sentSize = "";
          // A new controller answers queries using this device's theme.
          if (
            control.colorSchemeReports &&
            (!schemeReports || (controlling() && !wasControlling))
          )
            reportScheme();
          schemeReports = control.colorSchemeReports;
          view.fit();
          sendSize();
        } else if (frame[0] === 2) {
          info = SessionInfoSchema.parse(JSON.parse(decoder.decode(payload)));
          if (manageChrome)
            setBrowserChrome(
              info.title + " · WebMux",
              info.status === "running",
            );
        } else if (frame[0] === 3) {
          exited = true;
          exitCode = ExitSchema.parse(
            JSON.parse(decoder.decode(payload)),
          ).exitCode;
          if (stale && !document.hidden) restoreExited();
        }
      },
      (state) => {
        connection = state;
        if (manageChrome && (state === "reconnecting" || state === "missing"))
          resetBrowserChrome();
        if (state === "connected") {
          sentSize = "";
          view.fit();
          if (submissions.size) ws.send(4, encodeIdsQuery(submissions.keys()));
        } else hover.cancel();
        if (state === "connecting" || state === "reconnecting") {
          // The ring may have wrapped since the last position was received.
          // Exited recovery must obtain the server's final safe reset, not raw HTTP bytes.
          if (exited && stale) {
            synced = false;
            stale = false;
            exited = false;
          }
          // A synced terminal stays visible and is extended in place; the view
          // still shows a non-blocking reconnect hint from the connection state.
          if (!synced) restoring = true;
          schemeReports = false;
          for (const report of submissions.values()) report("unknown");
        }
        if (state === "missing") restoring = false;
      },
      (message) => {
        error = inputError.capture(message);
      },
      {
        resume(send) {
          return !stale && send(4, encodeIdsQuery([newSubmissionId()]));
        },
        canHandoff(position) {
          return (
            position.mode === "reset" ||
            resolveAppendOverlap(synced, generation, offset, position).accept
          );
        },
      },
    );
    socket = ws;
    graphics = attachTerminalGraphics(term, (text) => {
      if (!replaying) ws.send(6, encoder.encode(text));
    });
    const resize = term.onResize(sendSize);
    const disposeProtocol = registerTerminalProtocolHandlers(
      term,
      () => replaying,
      (text) => view.copy(text),
      reportScheme,
    );
    function restoreExited() {
      if (disposed) return;
      // Force a reset even if this terminal used to have a valid resume point.
      synced = false;
      ws.reconnect();
    }
    const visible = () => {
      // Live sessions reattach automatically; exited ones need explicit recovery.
      if (!document.hidden && stale && exited) {
        restoreExited();
      }
    };
    document.addEventListener("visibilitychange", visible);

    return {
      dispose() {
        disposed = true;
        hover.cancel();
        cancelAnimationFrame(restoreFrame);
        document.removeEventListener("visibilitychange", visible);
        graphics?.dispose();
        resize.dispose();
        disposeProtocol();
        output.dispose();
        ws.close();
        if (manageChrome) resetBrowserChrome();
        terminal = undefined;
        socket = undefined;
        synced = false;
      },
    };
  }

  const dismissError = () => {
    socket?.clearUncertainty();
    error = inputError.clearUncertain(error);
  };
  // Reattaches after a hybrid 4409 switch from a fresh snapshot.
  const reconnect = () => {
    dismissError();
    synced = false;
    socket?.reconnect();
  };

  return {
    get info() {
      return info;
    },
    get connection() {
      return connection;
    },
    get restoring() {
      return restoring;
    },
    get drawn() {
      return drawn;
    },
    get exitCode() {
      return exitCode;
    },
    get error() {
      return inputError.render(error);
    },
    get historyTruncated() {
      return historyTruncated;
    },
    get control() {
      return control;
    },
    get controlling() {
      return controlling();
    },
    // Until the server reports control, fit locally without resizing the PTY.
    get observing() {
      return !!control && !controlling();
    },
    // Connected, restored and still running: input can reach the program.
    get accepting() {
      return accepting();
    },
    get submittable() {
      return !!submission();
    },
    get clientId() {
      return identity.id;
    },
    attach,
    // Sends user input. A rejected send is reported, never queued.
    sendInput(input: string) {
      hover.flush();
      const sent =
        accepting() && (socket?.send(0, encoder.encode(input)) ?? false);
      error = inputError.sent(sent, error);
      if (sent) socket?.probe();
      return sent;
    },
    // Sends one of xterm's automatic replies to a device query.
    sendReply(reply: string) {
      if (!replaying) socket?.send(6, encoder.encode(reply));
    },
    // Coalesces controller pointer hover on the reply lane without taking control.
    sendHover(report: string) {
      if (!replaying && accepting() && controlling()) hover.schedule(report);
    },
    // Takes input and grid control for this browser tab.
    claim() {
      if (control && !controlling()) socket?.send(5, new Uint8Array());
    },
    // Sends text and an optional Enter as one acknowledged submission. The
    // body is prepared like xterm's paste; `report` receives each state.
    submit(
      submissionId: string,
      text: string,
      enter: boolean,
      report: (state: SubmissionProgress) => void,
    ) {
      const target = submission();
      if (!target) return false;
      hover.flush();
      const body = preparePaste(text, target.term.modes.bracketedPasteMode);
      submissions.set(submissionId, report);
      if (
        !target.ws.send(
          3,
          encodeSubmission(submissionId, target.run, body, enter ? "\r" : ""),
        )
      ) {
        submissions.delete(submissionId);
        return false;
      }
      report("sending");
      target.term.scrollToBottom();
      return true;
    },
    // Follows a submission whose acknowledgement may have been lost.
    track(submissionId: string, report: (state: SubmissionProgress) => void) {
      submissions.set(submissionId, report);
      if (connection === "connected")
        socket?.send(4, encodeIdsQuery([submissionId]));
    },
    untrack(submissionId: string) {
      submissions.delete(submissionId);
    },
    dismissError,
    // Every reset, including after exit, uses the server's safe replay path.
    resync() {
      dismissError();
      if (exitCode === null) socket?.send(2, new Uint8Array());
      else reconnect();
    },
    reconnect,
  };
}
