import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import { threadAPI } from "../src/platform/thread-api";
import { APIError } from "../src/platform/api";
import { ConversationActivitiesSchema } from "../src/contracts/conversation-schemas";

const source = "a".repeat(32);
const handoff = "b".repeat(32);
const id = "c".repeat(64);
const snapshot = "d".repeat(64);
const text = '甲🙂\n  {"command": "printf \'raw\'"}\n';
const item = {
  exportable: true,
  exclusionReason: "",
  activityId: id,
  conversationId: source,
  providerThreadId: "",
  runId: "",
  position: "2",
  runOrdinal: "0",
  kind: "tool_arguments",
  role: "agent",
  status: "",
  text,
  source: "native",
  providerItemId: "native-item",
  byteLength: String(new TextEncoder().encode(text).length),
  nextTextOffset: "31",
  truncated: true,
  createdAt: "0001-01-01T00:00:00Z",
  attachments: [],
  snapshotIds: [snapshot],
  sourceHash: "e".repeat(64),
  parentId: "parent",
  nativeId: "native-item",
  callId: "scoped-call",
  nativeCallId: "call",
  toolName: "shell",
};

describe("frozen history consumers", () => {
  afterEach(() => vi.restoreAllMocks());

  it("sends the trusted handoff on every byte continuation and preserves native text", async () => {
    const requests: URL[] = [];
    const signal = new AbortController().signal;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      requests.push(new URL(String(input), "http://localhost"));
      expect(init?.signal).toBe(signal);
      return Response.json(item);
    });
    for (const offset of ["0", "31", "9223372036854775807"]) {
      const result = await threadAPI.activity(
        source,
        handoff,
        id,
        offset,
        signal,
      );
      expect(result.text).toBe(text);
      expect(result.providerThreadId).toBe("");
      expect(result.sourceHash).toBe(item.sourceHash);
      expect(result.nativeCallId).toBe("call");
      const request = requests.at(-1)!;
      expect(request.pathname).toBe(
        `/api/conversations/${source}/activities/${id}`,
      );
      expect(request.searchParams.get("handoffId")).toBe(handoff);
      expect(request.searchParams.get("textOffset")).toBe(offset);
      expect(request.searchParams.get("maxBytes")).toBe("4000");
    }
  });

  it("does not fall back to live history when a frozen scope is rejected", async () => {
    const fetch = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        Response.json(
          { error: "native history identity mismatch" },
          { status: 409 },
        ),
      );
    await expect(
      threadAPI.activity(source, handoff, id),
    ).rejects.toBeInstanceOf(APIError);
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("retains snapshot IDs for subsequent fixed-range requests", () => {
    const page = ConversationActivitiesSchema.parse({
      items: [item],
      snapshotIds: [snapshot],
      nextPosition: "2",
      gaps: [],
      reconcileRequired: false,
      historyOrigin: "complete",
    });
    expect(page.snapshotIds).toEqual([snapshot]);
    expect(page.items[0]?.kind).toBe("tool_arguments");
    expect(page.items[0]?.parentId).toBe("parent");
  });
});
