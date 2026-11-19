import { m } from "$lib/paraglide/messages.js";

type TerminalInputError =
  | { kind: "send" }
  | { kind: "uncertain" }
  | { kind: "text"; message: string };

// Known browser errors retain semantic identity, not a captured translation.
// Arbitrary server error text is preserved verbatim.
export function createTerminalInputError() {
  return {
    capture(message: string): TerminalInputError {
      return message === m.relay_sent_input_uncertain()
        ? { kind: "uncertain" }
        : { kind: "text", message };
    },
    render(error: TerminalInputError): string {
      if (error.kind === "send") return m.s4_terminal_input_send_error();
      if (error.kind === "uncertain") return m.relay_sent_input_uncertain();
      return error.message;
    },
    sent(sent: boolean, error: TerminalInputError): TerminalInputError {
      if (!sent) return { kind: "send" };
      // Only a matching send error is cleared by a later successful input.
      // Uncertainty and unrelated terminal errors remain intact.
      return error.kind === "send" ? { kind: "text", message: "" } : error;
    },
    clearUncertain(error: TerminalInputError): TerminalInputError {
      return error.kind === "uncertain" ? { kind: "text", message: "" } : error;
    },
  };
}
