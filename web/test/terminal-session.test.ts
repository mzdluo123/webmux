import { expect, test, vi } from "vite-plus/test";
import { terminalSession } from "../src/hooks/sessions/terminal-session";
import type { HybridTerminal } from "../src/hooks/sessions/hybrid-types";

test("PTY-only projection follows the stream and never creates a chat/archive fallback", () => {
  const stream: HybridTerminal = {
    info: undefined,
    connection: "connecting",
    restoring: false,
    drawn: undefined,
    reconnect: vi.fn(),
  };
  const session = terminalSession(stream);
  expect(session.view).toBe("terminal");
  expect(session.known).toBe(false);
  expect(session.recoverable).toBeUndefined();
  expect(session.missing).toBe(false);
  Object.assign(stream, { connection: "missing" });
  expect(session.missing).toBe(true);
  expect(session.view).toBe("terminal");
  expect(session.recoverable).toBeUndefined();
  expect(() => session.chat).toThrow("聊天插件未启用");
  expect(stream.reconnect).not.toHaveBeenCalled();
});
