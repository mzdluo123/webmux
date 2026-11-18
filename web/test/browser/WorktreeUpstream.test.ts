import { afterEach, expect, test, vi } from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import { z } from "zod";
import { api } from "../../src/platform/api";
import {
  CreateSessionRequestSchema,
  CreateWorktreeRequestSchema,
} from "../../src/contracts/schemas";
import {
  UpstreamChoiceSchema,
  WorktreeUpstreamErrorSchema,
} from "../../src/contracts/worktree-upstream-schemas";
import { worktreeUpstreamConfirmation } from "../../src/platform/worktree-upstream";
import WorktreeUpstreamDialog from "../../src/components/shared/WorktreeUpstreamDialog.svelte";
import type { HandoffEditor } from "../../src/hooks/sessions/handoff.svelte";
import Harness from "./WorktreeUpstreamHarness.svelte";
import { session } from "./shell-fixture";

const challenge = {
  confirmationId: "initial",
  branch: "main",
  localHead: "a".repeat(40),
  upstream: "origin/main",
  upstreamHead: "b".repeat(40),
  warning: "Review the fetched history",
  canUseUpstream: true,
};
const conflict = (confirmationId = "initial", canUseUpstream = true) =>
  Response.json(
    {
      error: "Choose baseline",
      reason: "worktree_upstream_confirmation",
      worktreeUpstream: { ...challenge, confirmationId, canUseUpstream },
    },
    { status: 409 },
  );
const dialog = () =>
  page.getByRole("dialog", { name: "选择工作树基线", exact: true });
const worktree = {
  mode: "new",
  branch: "feature/task",
  base: "refs/heads/main",
  path: "/repo/task",
} as const;
const responseSchema = z.object({ ok: z.boolean() });

function start(
  body: unknown = worktree,
  sessionRequest = false,
  signal?: AbortSignal,
) {
  return api(
    sessionRequest ? "/sessions" : "/repositories/repo/worktrees",
    responseSchema,
    "POST",
    body,
    signal,
    sessionRequest ? CreateSessionRequestSchema : CreateWorktreeRequestSchema,
    sessionRequest,
  ).then(
    (value) => ({ value, error: null }),
    (error: unknown) => ({ value: null, error }),
  );
}

afterEach(async () => {
  worktreeUpstreamConfirmation.choose(null);
  await cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

test.each([
  ["使用上游最新", true],
  ["保留本地", false],
] as const)(
  "%s retries only the frozen create body",
  async (label, useUpstream) => {
    const requests: unknown[] = [];
    const controller = new AbortController();
    vi.stubGlobal(
      "fetch",
      vi.fn(async (_input, init: RequestInit) => {
        expect(init.signal).toBe(controller.signal);
        requests.push(JSON.parse(String(init.body)));
        return requests.length === 1 ? conflict() : Response.json({ ok: true });
      }),
    );
    render(WorktreeUpstreamDialog);
    const result = start(worktree, false, controller.signal);
    await expect.element(dialog()).toMatchTextContent(challenge.localHead);
    await expect.element(dialog()).toMatchTextContent(challenge.upstreamHead);
    await expect.element(page.getByText(challenge.warning)).toBeVisible();
    await page.getByRole("button", { name: label, exact: true }).click();
    expect((await result).value).toEqual({ ok: true });
    expect(requests).toEqual([
      worktree,
      {
        ...worktree,
        upstreamChoice: { confirmationId: "initial", useUpstream },
      },
    ]);
  },
);

test("cancel sends no second create request", async () => {
  const fetcher = vi.fn(async () => conflict());
  vi.stubGlobal("fetch", fetcher);
  render(WorktreeUpstreamDialog);
  const result = start();
  await expect.element(dialog()).toBeVisible();
  await page.getByRole("button", { name: "取消", exact: true }).click();
  expect((await result).error).toMatchObject({ name: "AbortError" });
  expect(fetcher).toHaveBeenCalledTimes(1);
});

test("stale confirmation asks again and honors canUseUpstream", async () => {
  const requests: unknown[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_input, init: RequestInit) => {
      requests.push(JSON.parse(String(init.body)));
      return requests.length === 1
        ? conflict()
        : requests.length === 2
          ? conflict("fresh", false)
          : Response.json({ ok: true });
    }),
  );
  render(WorktreeUpstreamDialog);
  const result = start();
  await page.getByRole("button", { name: "使用上游最新", exact: true }).click();
  await expect
    .element(page.getByRole("button", { name: "使用上游最新", exact: true }))
    .toBeDisabled();
  await page.getByRole("button", { name: "保留本地", exact: true }).click();
  expect((await result).value).toEqual({ ok: true });
  expect(requests[2]).toEqual({
    ...worktree,
    upstreamChoice: { confirmationId: "fresh", useUpstream: false },
  });
});

test("session confirmation preserves dedupe, staged attachments and first prompt", async () => {
  const body = {
    clientRequestId: "same-request",
    configId: "agent",
    view: "chat",
    prompt: "one first prompt",
    attachments: [
      { path: "/staged/image.png", name: "image.png", mimeType: "image/png" },
    ],
    newWorktree: { repositoryId: "repo", ...worktree },
  };
  // Keep attachment wire validation aligned with the real schema; the body
  // being retried is exactly what the caller already uploaded and submitted.
  const parsed = CreateSessionRequestSchema.parse(body);
  const requests: unknown[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_input, init: RequestInit) => {
      requests.push(JSON.parse(String(init.body)));
      if (requests.length === 2)
        throw new TypeError("connection interrupted after confirmation");
      return requests.length === 1 ? conflict() : Response.json({ ok: true });
    }),
  );
  render(WorktreeUpstreamDialog);
  const result = start(parsed, true);
  await page.getByRole("button", { name: "使用上游最新", exact: true }).click();
  expect((await result).value).toEqual({ ok: true });
  const confirmed = {
    ...parsed,
    newWorktree: {
      ...parsed.newWorktree,
      upstreamChoice: { confirmationId: "initial", useUpstream: true },
    },
  };
  expect(requests).toEqual([parsed, confirmed, confirmed]);
});

test("aborting while confirmation is open dismisses it without creating", async () => {
  const controller = new AbortController();
  const fetcher = vi.fn(async () => conflict());
  vi.stubGlobal("fetch", fetcher);
  render(WorktreeUpstreamDialog);
  const result = start(worktree, false, controller.signal);
  await expect.element(dialog()).toBeVisible();
  controller.abort();
  expect((await result).error).toMatchObject({ name: "AbortError" });
  await expect.element(dialog()).not.toBeInTheDocument();
  expect(fetcher).toHaveBeenCalledTimes(1);
});

test("challenge and choice reject malformed payloads", () => {
  expect(
    UpstreamChoiceSchema.safeParse({ confirmationId: "", useUpstream: true })
      .success,
  ).toBe(false);
  expect(
    UpstreamChoiceSchema.safeParse({ confirmationId: "id", useUpstream: "yes" })
      .success,
  ).toBe(false);
  expect(
    WorktreeUpstreamErrorSchema.safeParse({
      error: "bad",
      reason: "worktree_upstream_confirmation",
      worktreeUpstream: { ...challenge, canUseUpstream: "yes" },
    }).success,
  ).toBe(false);
});

const operation = (state: string, worktreeUpstream?: typeof challenge) => ({
  id: "delivery",
  previewId: "preview",
  instanceId: "instance",
  state,
  targetSessionId: "",
  error: "",
  worktreeId: "",
  worktreePath: "",
  startedAt: "2026-11-01T00:00:00Z",
  finishedAt: null,
  ...(worktreeUpstream ? { worktreeUpstream } : {}),
});

test.each([true, false, null] as const)(
  "handoff poll resumes the frozen operation or cancels: %s",
  async (choice) => {
    const sent: Record<string, unknown>[] = [];
    let previewReads = 0;
    let cancels = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input, init: RequestInit) => {
        const path = new URL(String(input), location.href).pathname;
        if (path === "/api/sessions/source") return Response.json(session());
        if (path === "/api/sessions/source/handoff-previews") {
          previewReads++;
          return Response.json({
            id: "preview",
            format: "text",
            pages: [],
            imageBudget: {
              maxPages: 8,
              pages: 0,
              bytes: 0,
              maxBytes: 1,
              maxRequestBytes: 1,
              projectedRunes: 0,
              retainedRunes: 0,
              estimatedTokens: 0,
              tokenEstimateKnown: false,
              tokenBasis: "",
              shape: "",
            },
            blockedReason: "",
            capability: "supported",
            historyText: "frozen history",
            instanceId: "instance",
            operationId: "delivery",
            sourceSessionId: "source",
            snapshotId: "snapshot",
            text: "frozen prompt",
            userText: "",
            request: JSON.parse(String(init.body)),
            createdAt: "2026-11-01T00:00:00Z",
            expiresAt: "2026-11-02T00:00:00Z",
            selectedCount: 1,
            omittedCount: 0,
            gapCount: 0,
            budget: {
              window: 10,
              nativeUsed: 0,
              current: 0,
              reserve: 0,
              history: 1,
              used: 1,
              source: "native",
              overBudget: false,
            },
          });
        }
        if (path === "/api/handoff-previews/preview/send") {
          sent.push(JSON.parse(String(init.body)));
          return Response.json(
            operation(sent.length === 1 ? "preparing" : "sending"),
          );
        }
        if (path === "/api/handoff-operations/delivery")
          return Response.json(operation("awaiting_upstream", challenge));
        if (path === "/api/handoff-operations/delivery/cancel") {
          cancels++;
          return Response.json(operation("failed"));
        }
        throw new Error(`Unexpected path ${path}`);
      }),
    );
    let editor!: HandoffEditor;
    render(Harness, {
      onready: (value) => {
        editor = value;
      },
    });
    await vi.waitFor(() => expect(editor?.loading).toBe(false));
    await editor.generate();
    await editor.send();
    const refreshing = editor.refresh();
    await expect.element(dialog()).toBeVisible();
    await page
      .getByRole("button", {
        name: choice === null ? "取消" : choice ? "使用上游最新" : "保留本地",
        exact: true,
      })
      .click();
    await refreshing;
    expect(previewReads).toBe(1);
    expect(sent[0]).toEqual({ operationId: "delivery" });
    if (choice === null) {
      expect(sent).toHaveLength(1);
      expect(cancels).toBe(1);
    } else {
      expect(sent[1]).toEqual({
        operationId: "delivery",
        upstreamChoice: { confirmationId: "initial", useUpstream: choice },
      });
      expect(cancels).toBe(0);
      expect(editor.operation?.state).toBe("sending");
    }
  },
);
