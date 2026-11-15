import * as m from "$lib/paraglide/messages.js";
import type { HybridSession, HybridTerminal } from "./hybrid-types";

function unavailable(): never {
  throw new Error(m.s3_chat_plugin_disabled());
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
    get chatUnavailable() {
      return m.s3_chat_plugin_disabled();
    },
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
