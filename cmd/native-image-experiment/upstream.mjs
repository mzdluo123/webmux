// Run beside a temporary installation of @oh-my-pi/snapcompact@18.22.2.
import {
  compact,
  createFileOps,
  frameBilling,
  frameTokens,
  getPreservedArchive,
  historyBlocks,
  normalize,
  resolveShapeForText,
  serializeConversation,
} from "@oh-my-pi/snapcompact";
import { createHash } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";

const [manifestPath, outputDir, frameLimit] = process.argv.slice(2);
const maxFrames = Number(frameLimit);
if (
  !manifestPath ||
  !outputDir ||
  !Number.isInteger(maxFrames) ||
  maxFrames < 1 ||
  maxFrames > 8
) {
  throw new Error(
    "usage: bun upstream.mjs MANIFEST OUTPUT_DIRECTORY MAX_FRAMES",
  );
}
const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
if (!manifest.synthetic_only || manifest.model !== "openai-codex/gpt-6-astra") {
  throw new Error("expected the synthetic gpt-6-astra experiment manifest");
}
const model = {
  api: "openai-codex-responses",
  id: "gpt-6-astra",
  provider: "openai-codex",
};
const messages = manifest.input.History.map((record) => {
  const content = [{ type: "text", text: record.Text }];
  if (record.Role === "tool") {
    return {
      role: "toolResult",
      content,
      toolCallId: record.CallID,
      toolName: record.ToolName,
      isError: false,
      timestamp: 0,
    };
  }
  return { role: record.Role, content, timestamp: 0 };
});
const serialized = serializeConversation(messages, { includeThinking: false });
const shape = resolveShapeForText(serialized, model);
const source = normalize(serialized, { shape });
const sha256 = (value) => createHash("sha256").update(value).digest("hex");
const result = await compact(
  {
    messagesToSummarize: messages,
    turnPrefixMessages: [],
    firstKeptEntryId: "synthetic-tail",
    tokensBefore: 0,
    fileOps: createFileOps(),
  },
  { model, maxFrames, includeThinking: false },
);
const archive = getPreservedArchive(result.preserveData);
const texts = [result.summary];
const pages = [];
await mkdir(outputDir, { recursive: true, mode: 0o700 });
for (const block of historyBlocks(archive)) {
  if (block.type === "text") {
    texts.push(block.text);
    continue;
  }
  const png = Buffer.from(block.data, "base64");
  const size = { width: png.readUInt32BE(16), height: png.readUInt32BE(20) };
  const file = `archive-${pages.length}.png`;
  await writeFile(join(outputDir, file), png, { mode: 0o600 });
  pages.push({
    file,
    ...size,
    bytes: png.length,
    sha256: sha256(png),
    detail: block.detail,
    estimatedTokens: frameTokens(frameBilling(model), size),
  });
}
const report = {
  scenario: manifest.scenario,
  maxFrames,
  shape,
  sourceSha256: sha256(source),
  sourceChars: source.length,
  retainedSha256: sha256(archive.text ?? ""),
  textHeadSha256: sha256(archive.textHead ?? ""),
  textTailSha256: sha256(archive.textTail ?? ""),
  retainedChars: archive.totalChars,
  omittedChars: archive.truncatedChars,
  textHeadChars: archive.textHead?.length ?? 0,
  textTailChars: archive.textTail?.length ?? 0,
  pages,
};
await writeFile(join(outputDir, "archive.txt"), texts.join("\n"), {
  mode: 0o600,
});
await writeFile(
  join(outputDir, "report.json"),
  JSON.stringify(report, null, 2),
  {
    mode: 0o600,
  },
);
console.log(JSON.stringify(report));
