import { afterEach, expect, test } from "vite-plus/test";
import {
  createChatSocket,
  type ChatSocket,
} from "../../src/hooks/chat/chat-socket.svelte";
import { handoffLiveSockets } from "../../src/platform/ws";
import {
  chatState,
  FakeSocket,
  installChatFixture,
  sockets,
} from "./chat-fixture";

installChatFixture(["chat"]);
let chat: ChatSocket | undefined;
afterEach(() => {
  chat?.dispose();
  chat = undefined;
});

const at = "2026-11-15T00:00:00Z";
function append(seq: number, text: string) {
  return {
    type: "events" as const,
    generation: "chat-run",
    items: [
      {
        seq,
        at,
        kind: "message.append" as const,
        message: { messageId: "reply", role: "agent" as const, text },
      },
    ],
  };
}

test("chat handoff resumes its own cursor and ignores overlapping events after an authoritative snapshot", () => {
  chat = createChatSocket("s1");
  chat.open();
  const baseline = sockets[0]!;
  baseline.open();
  baseline.frame({
    type: "snapshot",
    generation: "chat-run",
    from: 0,
    state: chatState(),
  });
  baseline.frame(append(0, "A"));
  handoffLiveSockets();
  const candidate = sockets[1]!;
  const query = new URL(candidate.url).searchParams;
  expect(query.get("generation")).toBe("chat-run");
  expect(query.get("seq")).toBe("0");
  expect(query.has("offset")).toBe(false);
  baseline.frame(append(1, "B"));
  candidate.open();
  expect(baseline.readyState).toBe(FakeSocket.OPEN);
  candidate.frame({
    type: "snapshot",
    generation: "chat-run",
    from: 1,
    state: chatState({ forceSwitchNote: "authoritative candidate state" }),
  });
  expect(baseline.readyState).toBe(FakeSocket.CLOSED);
  candidate.frame({
    ...append(1, "B"),
    items: [...append(1, "B").items, ...append(2, "C").items],
  });
  expect(chat.model.entries).toMatchObject([{ type: "message", text: "ABC" }]);
  expect(chat.state?.forceSwitchNote).toBe("authoritative candidate state");
  expect(chat.connection).toBe("connected");
});
