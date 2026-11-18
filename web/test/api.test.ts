import { afterEach, expect, test, vi } from "vite-plus/test";
import { z } from "zod";
import { api } from "../src/platform/api";
import { CreateSessionRequestSchema } from "../src/contracts/schemas";
import { resumeSession } from "../src/domains/sessions/recoverable";

vi.stubGlobal("localStorage", { getItem: () => null });
afterEach(() => {
  vi.unstubAllGlobals();
});

test("an invalid request body is rejected before any network request", async () => {
  const fetchMock = vi.fn<typeof fetch>();
  vi.stubGlobal("fetch", fetchMock);
  await expect(
    api(
      "/sessions",
      z.undefined(),
      "POST",
      { configId: "saved", cmd: ["sh"] },
      undefined,
      z.object({ configId: z.string() }).strict(),
    ),
  ).rejects.toThrow();
  expect(fetchMock).not.toHaveBeenCalled();
});

test.each([{ cmd: [] }, { env: {} }])(
  "an empty override does not conflict with configId: %j",
  (override) => {
    const body = { configId: "saved", ...override };
    expect(CreateSessionRequestSchema.parse(body)).toEqual(body);
  },
);

test("non-empty command or environment conflicts with configId", () => {
  for (const body of [
    { configId: "saved", cmd: ["sh"] },
    { configId: "saved", env: { A: "1" } },
  ])
    expect(CreateSessionRequestSchema.safeParse(body).success).toBe(false);
});

test.each(["session_missing", "session_ambiguous"])(
  "resume offers the candidates from %s",
  async (reason) => {
    vi.stubGlobal("localStorage", { getItem: () => null });
    const candidate = {
      agentSessionId: "session_1",
      title: "原对话",
      updatedAt: "2026-11-18T00:00:00Z",
    };
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        Response.json(
          { error: "请选择要恢复的会话", reason, candidates: [candidate] },
          { status: 409 },
        ),
      ),
    );
    await expect(resumeSession("hybrid22", "terminal")).resolves.toEqual({
      status: "candidates",
      message: "请选择要恢复的会话",
      candidates: [candidate],
    });
  },
);

test("the initial terminal size stays within the PTY's 1..1000 grid", () => {
  const cmd = ["sh"];
  expect(
    CreateSessionRequestSchema.safeParse({ cmd, cols: 200, rows: 50 }).success,
  ).toBe(true);
  for (const size of [
    { cols: 0, rows: 24 },
    { cols: 1001, rows: 24 },
    { cols: 80, rows: 1.5 },
  ])
    expect(CreateSessionRequestSchema.safeParse({ cmd, ...size }).success).toBe(
      false,
    );
});
