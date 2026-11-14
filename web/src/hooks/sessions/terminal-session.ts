import type { HybridSession, HybridTerminal } from "./hybrid-types";

function unavailable(): never {
  throw new Error("聊天插件未启用");
}

/** PTY-only projection: no archive lookup, chat provider, or switch lifecycle. */
export function terminalSession(terminal: HybridTerminal): HybridSession {
  return {
    known: false,
    view: "terminal",
    get info() {
      return terminal.info;
    },
    get missing() {
      return terminal.connection === "missing";
    },
    get chat() {
      return unavailable();
    },
    get actions() {
      return unavailable();
    },
    side: "terminal",
    target: null,
    steps: [],
    chatUnavailable: "聊天插件未启用",
    confirming: null,
    awaitingChoice: false,
    answering: false,
    notices: [],
    recoverable: undefined,
    connector: undefined,
    ambiguous: null,
    request: unavailable,
    confirm: unavailable,
    cancelConfirm: unavailable,
    choose: unavailable,
    pick: unavailable,
    cancelPick: unavailable,
    dismissConnector: unavailable,
    dismiss: unavailable,
  };
}
