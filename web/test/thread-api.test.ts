import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import { threadAPI } from "../src/platform/thread-api";
import { APIError } from "../src/platform/api";
const page = {
  expiresAt: "",
  agent: "omp",
  nativeId: "native",
  branch: "",
  format: "v1",
  boundary: "fixed",
  contentHash: "hash",
  truncated: false,
  more: false,
  nextPosition: "1",
  nextTextOffset: "0",
  recordCount: 1,
  files: [],
  snapshotId: "instance_scope",
  parser: "v1",
  pending: false,
  gaps: [],
  records: [],
};
describe("frozen native history client", () => {
  afterEach(() => vi.restoreAllMocks());
  it("carries the frozen scope on every continuation", async () => {
    const requests: URL[] = [];
    const signal = new AbortController().signal;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      requests.push(new URL(String(input), "http://localhost"));
      expect(init?.signal).toBe(signal);
      return Response.json(page);
    });
    for (const offset of ["0", "31", "9007199254740993"]) {
      expect(
        (
          await threadAPI.history(
            "source",
            "instance_scope",
            "0",
            offset,
            signal,
          )
        ).snapshotId,
      ).toBe("instance_scope");
      const request = requests.at(-1)!;
      expect(request.pathname).toBe("/api/sessions/source/native-history");
      expect(request.searchParams.get("snapshotId")).toBe("instance_scope");
      expect(request.searchParams.get("textOffset")).toBe(offset);
    }
  });
  it("surfaces an expired handle with one request", async () => {
    const fetch = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(Response.json({ error: "expired" }, { status: 410 }));
    await expect(
      threadAPI.history("source", "expired-scope"),
    ).rejects.toBeInstanceOf(APIError);
    expect(fetch).toHaveBeenCalledTimes(1);
  });
});
