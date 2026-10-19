import { StoredDraftSchema, type StoredDraft } from "./schemas";

const prefix = "webmux.draft.";
const settingKey = "webmux.draftPersistence";
export const draftRetention = 7 * 24 * 60 * 60 * 1000;
// Large drafts stay in memory only rather than exhausting the origin quota.
export const draftStorageLimit = 200_000;

export interface DraftStorage {
  readonly enabled: boolean;
  setEnabled(enabled: boolean): void;
  load(sessionId: string): StoredDraft | null;
  // False when the draft could not be kept on this device.
  save(draft: StoredDraft): boolean;
  remove(sessionId: string): void;
  // Drops drafts of sessions the server no longer has.
  keep(sessionIds: Iterable<string>): void;
  clear(): void;
  count(): number;
}

// Drafts live in this browser only, keyed by session and bound to the run
// (generation) they were written for. Every access tolerates unavailable or
// corrupt storage: persistence is a convenience, never a requirement.
export function createDraftStorage(
  storage: Storage | null,
  now: () => number = Date.now,
): DraftStorage {
  const attempt = <T>(fallback: T, action: (storage: Storage) => T): T => {
    if (!storage) return fallback;
    try {
      return action(storage);
    } catch {
      return fallback;
    }
  };
  const keys = () =>
    attempt<string[]>([], (storage) =>
      Array.from({ length: storage.length }, (_, index) =>
        storage.key(index),
      ).filter((key): key is string => !!key?.startsWith(prefix)),
    );
  const read = (key: string) =>
    attempt<StoredDraft | null>(null, (storage) => {
      const parsed = StoredDraftSchema.safeParse(
        JSON.parse(storage.getItem(key) ?? "null"),
      );
      return parsed.success &&
        prefix + parsed.data.sessionId === key &&
        now() - parsed.data.savedAt < draftRetention
        ? parsed.data
        : null;
    });
  const drop = (key: string) =>
    attempt(undefined, (storage) => storage.removeItem(key));
  const result: DraftStorage = {
    get enabled() {
      return attempt(false, (storage) => storage.getItem(settingKey) !== "off");
    },
    setEnabled(enabled) {
      attempt(undefined, (storage) => {
        if (enabled) storage.removeItem(settingKey);
        else storage.setItem(settingKey, "off");
      });
      if (!enabled) result.clear();
    },
    load(sessionId) {
      return result.enabled ? read(prefix + sessionId) : null;
    },
    save(draft) {
      const key = prefix + draft.sessionId;
      if (!result.enabled) return false;
      if (!draft.text && !draft.submission) {
        drop(key);
        return true;
      }
      if (draft.text.length > draftStorageLimit) {
        drop(key);
        return false;
      }
      return attempt(false, (storage) => {
        storage.setItem(key, JSON.stringify(draft));
        return true;
      });
    },
    remove(sessionId) {
      drop(prefix + sessionId);
    },
    keep(sessionIds) {
      const present = new Set(Array.from(sessionIds, (id) => prefix + id));
      for (const key of keys()) if (!present.has(key)) drop(key);
    },
    clear() {
      for (const key of keys()) drop(key);
    },
    count() {
      return keys().filter((key) => read(key)).length;
    },
  };
  // Expired or corrupt entries are pruned whenever storage is opened.
  for (const key of keys()) if (!read(key)) drop(key);
  return result;
}

let shared: DraftStorage | undefined;
export function browserDraftStorage() {
  return (shared ??= createDraftStorage(
    (() => {
      try {
        return localStorage;
      } catch {
        return null;
      }
    })(),
  ));
}
