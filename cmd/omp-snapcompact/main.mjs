import {
  archiveSourceText,
  compact,
  createFileOps,
  frameBilling,
  frameDataBytesBudget,
  frameTokens,
  geometry,
  getPreservedArchive,
  historyBlocks,
  maxFramesForDataBudget,
  plannedFrameTokens,
  providerFrameBudget,
  resolveShapeAndScan,
  serializeConversation,
} from "@oh-my-pi/snapcompact";

const callKinds = new Set(["tool", "tool_arguments", "tool-call"]);
const resultKinds = new Set(["tool_result", "tool-result"]);
const providerAPIs = {
  anthropic: "anthropic-messages",
  google: "google-generative-ai",
  "google-vertex": "google-vertex",
  openai: "openai-responses",
  "openai-codex": "openai-codex-responses",
  openrouter: "openai-completions",
};

function argumentsOf(text) {
  try {
    const args = JSON.parse(text);
    if (args && typeof args === "object" && !Array.isArray(args)) return args;
  } catch {
    // Some readers only retain the visible tool call body.
  }
  return text ? { input: text } : {};
}

function messagesOf(records) {
  return records.map((record, index) => {
    const callId = record.callId || `webmux-orphan-${index}`;
    const timestamp = 0;
    if (callKinds.has(record.kind)) {
      return {
        role: "assistant",
        content: [
          {
            type: "toolCall",
            id: callId,
            name: record.toolName || "tool",
            arguments: argumentsOf(record.text),
          },
        ],
        timestamp,
      };
    }
    const content = [{ type: "text", text: record.text }];
    if (record.role === "tool" || resultKinds.has(record.kind)) {
      return {
        role: "toolResult",
        content,
        toolCallId: callId,
        toolName: record.toolName || "tool",
        isError: ["error", "failed"].includes(record.status),
        timestamp,
      };
    }
    return {
      role: ["assistant", "agent"].includes(record.role) ? "assistant" : "user",
      content,
      timestamp,
    };
  });
}

function targetOf(id) {
  const slash = id.indexOf("/");
  if (slash < 0) return { id };
  const provider = id.slice(0, slash);
  return { provider, id: id.slice(slash + 1), api: providerAPIs[provider] };
}

export async function archive(request) {
  const messages = messagesOf(request.records);
  const model = targetOf(request.model);
  const serialized = serializeConversation(messages, {
    includeThinking: false,
  });
  const { shape, isSafe } = resolveShapeAndScan(serialized, model);
  if (!isSafe) return { protocol: 1, unrenderable: true };
  const edgeTokens = Math.ceil((2 * geometry(shape).capacity * 1.15) / 4);
  const frameBytes = Math.min(
    request.maxFrameDataBytes,
    frameDataBytesBudget(model),
  );
  // Apply the upstream edge and summary reserve inside WebMux's history
  // envelope. The caller validates the final prompt budget.
  const frameTokensRoom = request.historyTokens - edgeTokens - 2000;
  const maxFrames = Math.min(
    request.maxFrames,
    providerFrameBudget(model),
    maxFramesForDataBudget(shape, frameBytes),
    Math.max(1, Math.floor(frameTokensRoom / plannedFrameTokens(shape))),
  );
  const result = await compact(
    {
      messagesToSummarize: messages,
      turnPrefixMessages: [],
      firstKeptEntryId: "webmux-frozen-history",
      tokensBefore: 0,
      fileOps: createFileOps(),
    },
    { model, shape, maxFrames, includeThinking: false },
  );
  const resultArchive = getPreservedArchive(result.preserveData);
  const blocks = historyBlocks(resultArchive);
  const billing = frameBilling(model);
  let imageTokens = 0;
  let imageBytes = 0;
  for (const block of blocks) {
    if (block.type !== "image") continue;
    const png = Buffer.from(block.data, "base64");
    const size = { width: png.readUInt32BE(16), height: png.readUInt32BE(20) };
    imageTokens += frameTokens(billing, size);
    imageBytes += block.data.length;
  }
  return {
    protocol: 1,
    summary: result.summary,
    blocks,
    history: archiveSourceText(resultArchive) ?? "",
    retainedChars: resultArchive.totalChars,
    omittedChars: resultArchive.truncatedChars,
    imageTokens,
    imageBudgetExceeded: imageBytes > frameBytes,
    imageTokensVerified: shape.frameTokensVerified === true,
    shape: `${shape.cellWidth}x${shape.cellHeight}/${shape.frameSize}px (${shape.font})`,
  };
}

if (import.meta.main) {
  try {
    const input = JSON.parse(await Bun.stdin.text());
    const result = await archive(input);
    await Bun.write(Bun.stdout, JSON.stringify(result));
  } catch {
    // Source records may contain private content. Keep child errors opaque.
    process.exitCode = 1;
  }
}
