import { z } from "zod";
import { api, store, stored } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import { connect } from "../../platform/ws";
import type { Connection } from "../../platform/ws-contracts";
import {
  reloadSettings,
  settings,
} from "../../domains/settings/settings.svelte";
import { homeCache } from "../../domains/workspace/home-cache";
import {
  RepositoriesEventSchema,
  RepositoriesSchema,
  type Repository,
} from "../../contracts/schemas";

/** The registered repositories and the workbench's chosen view of them. */
export interface Repositories {
  readonly list: readonly Repository[];
  /** A read finished (successfully or not) since the last refresh began. */
  readonly settled: boolean;
  /** A read has finished; stays true during background refreshes. */
  readonly initialSettled: boolean;
  readonly refreshing: boolean;
  /** The last read's failure; "" after a successful one. */
  readonly error: string;
  /**
   * The saved view: "all", "direct" or a repository ID, null before any
   * choice. It may name a repository that no longer exists; the home page
   * resolves that once a read settles.
   */
  readonly choice: string | null;
  /**
   * The worktree the view narrows to inside the chosen repository; null
   * for the whole repository. Saved like the choice.
   */
  readonly worktree: string | null;
  /** Chooses and remembers the view, optionally one worktree of it. */
  select(id: string, worktreeId?: string): void;
  /**
   * Reads the repositories. Until the first read finishes, a lifecycle
   * refresh joins it instead of starting another slow discovery; `force`
   * and every later refresh start a new read and discard older replies.
   */
  refresh(force?: boolean): Promise<void>;
  /**
   * Puts a repository another request just read (such as a fetch's reply)
   * in place of its listed snapshot; unknown repositories are ignored.
   */
  replace(repository: Repository): void;
  dispose(): void;
}

const ChoiceSchema = z.string().min(1).max(256);
const STREAM_COALESCE_MS = 80;

export function createRepositories(): Repositories {
  // The previous grouping renders at once instead of flashing ungrouped
  // sessions while Git runs.
  let list = $state.raw<Repository[]>(homeCache.repositories() ?? []);
  let settled = $state(false);
  let initialSettled = $state(false);
  let refreshing = $state(false);
  let error = $state("");
  let choice = $state(ChoiceSchema.safeParse(stored("homeRepository")).data);
  let worktree = $state(
    ChoiceSchema.safeParse(stored("homeWorktree")).data ?? null,
  );
  let startup: Promise<void> | undefined;
  let generation = 0;
  let disposed = false;
  let coalesceTimer: ReturnType<typeof setTimeout> | undefined;
  let streamState: Connection = "connecting";
  const abort = new AbortController();

  async function read() {
    const current = ++generation;
    refreshing = true;
    settled = false;
    error = "";
    try {
      const result = await api(
        "/repositories",
        RepositoriesSchema,
        "GET",
        undefined,
        abort.signal,
      );
      if (!disposed && current === generation) {
        list = result;
        homeCache.rememberRepositories(result);
        if (!settings.connected) void reloadSettings();
      }
    } catch (err) {
      if (!disposed && current === generation) error = errorMessage(err);
    } finally {
      if (!disposed && current === generation) {
        refreshing = false;
        settled = true;
        initialSettled = true;
      }
    }
  }

  function refresh(force = false) {
    if (!force && startup) return startup;
    const pending = read();
    if (generation === 1) {
      startup = pending;
      void pending.finally(() => {
        startup = undefined;
      });
    }
    return pending;
  }

  function scheduleStreamRefresh() {
    if (disposed || coalesceTimer !== undefined) return;
    coalesceTimer = setTimeout(() => {
      coalesceTimer = undefined;
      if (!disposed) void refresh(true);
    }, STREAM_COALESCE_MS);
  }

  const socket = connect(
    "/ws/repositories",
    (raw) => {
      if (disposed || typeof raw !== "string") return;
      try {
        const parsed = RepositoriesEventSchema.safeParse(JSON.parse(raw));
        if (parsed.success) scheduleStreamRefresh();
      } catch {
        // Ignore malformed text frames.
      }
    },
    (state) => {
      const reconnected =
        state === "connected" && streamState === "reconnecting";
      streamState = state;
      if (reconnected) scheduleStreamRefresh();
    },
    () => {
      // Explicit HTTP reads report repository errors; background stream drops retry silently.
    },
  );

  return {
    get list() {
      return list;
    },
    get settled() {
      return settled;
    },
    get initialSettled() {
      return initialSettled;
    },
    get refreshing() {
      return refreshing;
    },
    get error() {
      return error;
    },
    get choice() {
      return choice ?? null;
    },
    get worktree() {
      return worktree;
    },
    select(id, worktreeId) {
      choice = id;
      worktree = worktreeId ?? null;
      store("homeRepository", id);
      store("homeWorktree", worktreeId ?? "");
    },
    refresh,
    replace(repository) {
      if (!list.some((item) => item.id === repository.id)) return;
      list = list.map((item) =>
        item.id === repository.id ? repository : item,
      );
      homeCache.rememberRepositories(list);
      if (!settings.connected) void reloadSettings();
    },
    dispose() {
      disposed = true;
      ++generation;
      if (coalesceTimer !== undefined) {
        clearTimeout(coalesceTimer);
        coalesceTimer = undefined;
      }
      socket.close();
      abort.abort();
    },
  };
}
