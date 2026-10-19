import { describe, expect, test } from "vitest";
import {
  createDraftStorage,
  draftRetention,
  draftStorageLimit,
} from "../src/draftStorage";
import type { StoredDraft } from "../src/schemas";

class MemoryStorage implements Storage {
  data = new Map<string, string>();
  get length() {
    return this.data.size;
  }
  clear() {
    this.data.clear();
  }
  getItem(key: string) {
    return this.data.get(key) ?? null;
  }
  key(index: number) {
    return [...this.data.keys()][index] ?? null;
  }
  removeItem(key: string) {
    this.data.delete(key);
  }
  setItem(key: string, value: string) {
    this.data.set(key, value);
  }
}

const draft = (sessionId: string, savedAt: number, text = "draft") =>
  ({
    v: 1,
    sessionId,
    generation: "0123456789abcdef",
    text,
    savedAt,
    submission: null,
  }) satisfies StoredDraft;

describe("browser draft storage", () => {
  test("round-trips drafts and unconfirmed submissions per session", () => {
    const memory = new MemoryStorage();
    const storage = createDraftStorage(memory, () => 1000);
    const pending = {
      ...draft("aaaaaaaa", 1000, "中文🙂\nsecond"),
      submission: { id: "s-00000001", state: "received" as const, at: 900 },
    };
    expect(storage.save(pending)).toBe(true);
    expect(storage.save(draft("bbbbbbbb", 1000))).toBe(true);
    expect(createDraftStorage(memory, () => 2000).load("aaaaaaaa")).toEqual(
      pending,
    );
    expect(storage.count()).toBe(2);
    // An empty draft without a submission leaves nothing behind.
    storage.save(draft("bbbbbbbb", 1000, ""));
    expect(storage.load("bbbbbbbb")).toBeNull();
    expect(memory.getItem("webmux.draft.bbbbbbbb")).toBeNull();
  });

  test("expired, corrupt and misfiled entries are pruned on open", () => {
    const memory = new MemoryStorage();
    memory.setItem(
      "webmux.draft.aaaaaaaa",
      JSON.stringify(draft("aaaaaaaa", 0)),
    );
    memory.setItem("webmux.draft.bbbbbbbb", "{not json");
    memory.setItem(
      "webmux.draft.cccccccc",
      JSON.stringify(draft("dddddddd", draftRetention)),
    );
    memory.setItem(
      "webmux.draft.eeeeeeee",
      JSON.stringify(draft("eeeeeeee", draftRetention)),
    );
    memory.setItem("webmux.token", "secret");
    const storage = createDraftStorage(memory, () => draftRetention + 1);
    expect([...memory.data.keys()].sort()).toEqual([
      "webmux.draft.eeeeeeee",
      "webmux.token",
    ]);
    expect(storage.load("eeeeeeee")?.text).toBe("draft");
  });

  test("keep() drops drafts of sessions the server no longer has", () => {
    const storage = createDraftStorage(new MemoryStorage(), () => 1);
    storage.save(draft("aaaaaaaa", 1));
    storage.save(draft("bbbbbbbb", 1));
    storage.keep(["bbbbbbbb", "cccccccc"]);
    expect(storage.load("aaaaaaaa")).toBeNull();
    expect(storage.load("bbbbbbbb")).not.toBeNull();
  });

  test("turning persistence off deletes saved drafts and stops saving", () => {
    const memory = new MemoryStorage();
    const storage = createDraftStorage(memory, () => 1);
    storage.save(draft("aaaaaaaa", 1));
    storage.setEnabled(false);
    expect(storage.enabled).toBe(false);
    expect(storage.count()).toBe(0);
    expect(storage.save(draft("aaaaaaaa", 1))).toBe(false);
    expect(createDraftStorage(memory).enabled).toBe(false);
    storage.setEnabled(true);
    expect(storage.save(draft("aaaaaaaa", 1))).toBe(true);
  });

  test("oversized drafts and unavailable storage degrade to memory only", () => {
    const storage = createDraftStorage(new MemoryStorage(), () => 1);
    storage.save(draft("aaaaaaaa", 1));
    expect(
      storage.save(draft("aaaaaaaa", 1, "x".repeat(draftStorageLimit + 1))),
    ).toBe(false);
    expect(storage.load("aaaaaaaa")).toBeNull();
    const full = new MemoryStorage();
    full.setItem = () => {
      throw new DOMException("quota", "QuotaExceededError");
    };
    expect(createDraftStorage(full).save(draft("aaaaaaaa", 1))).toBe(false);
    const missing = createDraftStorage(null);
    expect(missing.enabled).toBe(false);
    expect(missing.save(draft("aaaaaaaa", 1))).toBe(false);
    expect(missing.load("aaaaaaaa")).toBeNull();
  });
});
