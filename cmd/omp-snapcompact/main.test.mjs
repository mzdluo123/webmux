import { expect, test } from "bun:test";
import {
  compact,
  createFileOps,
  getPreservedArchive,
  historyBlocks,
} from "@oh-my-pi/snapcompact";
import { archive } from "./main.mjs";

const model = "openai-codex/gpt-6-astra";
const upstreamModel = {
  id: "gpt-6-astra",
  provider: "openai-codex",
  api: "openai-codex-responses",
};

function fixture(turns) {
  const records = [];
  const messages = [];
  for (let turn = 0; turn < turns; turn++) {
    const command = `inspect revision ${turn}`;
    const text = `Revision ${turn} 青竹\n    return result\n`;
    const callId = `call-${turn}`;
    const output = String(turn).repeat(4) + text.repeat(55);
    records.push(
      { role: "user", kind: "prose", text: command },
      { role: "assistant", kind: "code", text },
      {
        role: "assistant",
        kind: "tool_arguments",
        toolName: "read",
        callId,
        text: '{"limit":200}',
      },
      {
        role: "tool",
        kind: "tool_result",
        toolName: "read",
        callId,
        text: output,
      },
    );
    messages.push(
      { role: "user", content: command, timestamp: 0 },
      {
        role: "assistant",
        timestamp: 0,
        content: [
          { type: "text", text },
          {
            type: "toolCall",
            name: "read",
            id: callId,
            arguments: { limit: 200 },
          },
        ],
      },
      {
        role: "toolResult",
        content: [{ type: "text", text: output }],
        toolName: "read",
        toolCallId: callId,
        isError: false,
        timestamp: 0,
      },
    );
  }
  return { records, messages };
}

test.each([
  { turns: 1, maxFrames: 8, omitted: false },
  { turns: 48, maxFrames: 8, omitted: false },
  { turns: 92, maxFrames: 3, omitted: true },
])(
  "same OMP blocks and PNG bytes for %j",
  async ({ turns, maxFrames, omitted }) => {
    const { records, messages } = fixture(turns);
    const expected = await compact(
      {
        messagesToSummarize: messages,
        turnPrefixMessages: [],
        firstKeptEntryId: "fixture",
        tokensBefore: 0,
        fileOps: createFileOps(),
      },
      { model: upstreamModel, maxFrames, includeThinking: false },
    );
    const expectedArchive = getPreservedArchive(expected.preserveData);
    const actual = await archive({
      records,
      model,
      maxFrames,
      maxFrameDataBytes: 20 << 20,
      historyTokens: 64000,
    });
    expect(expectedArchive.truncatedChars > 0).toBe(omitted);
    expect(actual.omittedChars).toBe(expectedArchive.truncatedChars);
    expect(actual.summary).toBe(expected.summary);
    expect(actual.blocks).toEqual(historyBlocks(expectedArchive));
  },
);
