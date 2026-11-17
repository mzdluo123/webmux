import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import HandoffHistoryReader from "../../src/components/chat/HandoffHistoryReader.svelte";

const sourceId = "abcdefgh";
const snapshotId = "process_frozen-scope";

function serveHistory(expireSecondPage = false) {
  const requests: URL[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input) => {
      const url = new URL(String(input), location.origin);
      requests.push(url);
      const first = url.searchParams.get("textOffset") === "0";
      if (!first && expireSecondPage)
        return Response.json({ error: "冻结范围已失效" }, { status: 410 });
      const text = first ? "中文🙂" : "  code()";
      return Response.json({
        expiresAt: "2026-11-16T00:05:00Z",
        agent: "omp",
        nativeId: "native-source",
        branch: "",
        format: "v1",
        boundary: "complete-prefix",
        contentHash: "frozen-content",
        truncated: false,
        more: first,
        nextPosition: first ? "0" : "1",
        nextTextOffset: first ? "3" : "0",
        recordCount: 1,
        files: [],
        snapshotId,
        parser: "v1",
        pending: false,
        gaps: [],
        records: [
          {
            id: "record-1",
            nativeId: "native-record-1",
            parentId: "",
            hash: "record-hash",
            kind: "prose",
            role: "user",
            status: "",
            text,
            callId: "",
            nativeCallId: "",
            toolName: "",
            position: "1",
            offset: "0",
            end: "18",
            byteLength: "18",
            nextTextOffset: first ? "3" : "11",
            truncated: first,
            attachments: [],
          },
        ],
      });
    }),
  );
  render(HandoffHistoryReader, { sourceId, snapshotId });
  return requests;
}

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  vi.unstubAllGlobals();
});

const read = () =>
  page.getByRole("button", { name: "补查冻结原文", exact: true });

test("reads Unicode pages from the same frozen scope", async () => {
  const requests = serveHistory();
  await read().click();
  await expect.element(page.getByText("中文🙂", { exact: true })).toBeVisible();
  await read().click();
  await expect
    .element(page.getByText("中文🙂  code()", { exact: true }))
    .toBeVisible();
  await expect.element(read()).toBeDisabled();
  expect(requests.map((url) => url.pathname)).toEqual([
    `/api/sessions/${sourceId}/native-history`,
    `/api/sessions/${sourceId}/native-history`,
  ]);
  expect(requests.map((url) => url.searchParams.get("snapshotId"))).toEqual([
    snapshotId,
    snapshotId,
  ]);
  expect(requests.map((url) => url.searchParams.get("textOffset"))).toEqual([
    "0",
    "3",
  ]);
});

test("keeps displayed text when the frozen scope expires", async () => {
  serveHistory(true);
  await read().click();
  await expect.element(page.getByText("中文🙂", { exact: true })).toBeVisible();
  await read().click();
  await expect
    .element(page.getByText("冻结范围已失效", { exact: true }))
    .toBeVisible();
  await expect.element(page.getByText("中文🙂", { exact: true })).toBeVisible();
});
