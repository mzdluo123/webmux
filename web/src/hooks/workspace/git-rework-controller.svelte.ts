import { SvelteMap, SvelteSet } from "svelte/reactivity";
import type {
  GitWorkbenchViewState,
  GitWorkingChange,
} from "../../contracts/git-workbench-schemas";
import type { GitReworkItem, SessionInfo } from "../../contracts/schemas";
import {
  createReworkDraftItem,
  formatCommitDraftAssistPrompt,
  formatReworkFeedbackMarkdown,
  readGitReworkItems,
  writeGitReworkItems,
} from "../../domains/workspace/git-rework-model";
import { gitReviews } from "../../platform/git-collab-api";
import { LatestRequest } from "../shared/remote-request.svelte";

export type GitSessionHandoffMode = "rework" | "commit-assist";

export interface GitReworkBindings {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
}

export function createGitReworkController(bindings: GitReworkBindings) {
  let items = $state<GitReworkItem[]>([]);
  let sheetOpen = $state(false);
  let mode = $state<GitSessionHandoffMode>("rework");
  let selectedSessionId = $state("");
  let checkingFreshness = $state(false);
  let stalePaths = $state<string[]>([]);
  let staleConfirmed = $state(false);
  let copiedFeedback = $state(false);

  const freshnessReq = new LatestRequest();

  function dispose() {
    freshnessReq.dispose();
  }

  function loadTarget(
    repoId = bindings.getRepoId(),
    wtId = bindings.getWorktreeId(),
  ) {
    freshnessReq.cancel();
    items = repoId ? readGitReworkItems(repoId, wtId) : [];
    sheetOpen = false;
    mode = "rework";
    selectedSessionId = "";
    checkingFreshness = false;
    stalePaths = [];
    staleConfirmed = false;
    copiedFeedback = false;
  }

  function persistItems(next: GitReworkItem[]) {
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    items = repoId ? writeGitReworkItems(repoId, wtId, next) : next;
  }

  function syncDefaultSessionSelection(activeSessions: readonly SessionInfo[]) {
    if (activeSessions.length === 1) {
      selectedSessionId = activeSessions[0]!.id;
      return;
    }
    if (activeSessions.length === 0) {
      selectedSessionId = "";
      return;
    }
    // >= 2 active sessions: keep explicit user choice if still present, else empty!
    if (!activeSessions.some((s) => s.id === selectedSessionId)) {
      selectedSessionId = "";
    }
  }

  function addItem(
    input: Parameters<typeof createReworkDraftItem>[0],
    options?: { openSheet?: boolean },
  ): GitReworkItem {
    const created = createReworkDraftItem(input);
    persistItems([...items, created]);
    stalePaths = [];
    staleConfirmed = false;
    copiedFeedback = false;
    if (options?.openSheet) {
      mode = "rework";
      sheetOpen = true;
    } else {
      bindings.patchView({
        statusBanner: `已加入审阅意见草稿箱：${created.path} (${created.versionLabel} L${created.startLine}${created.endLine !== created.startLine ? `-L${created.endLine}` : ""})`,
      });
    }
    return created;
  }

  function updateItemNote(itemId: string, userNote: string) {
    persistItems(
      items.map((it) => (it.id === itemId ? { ...it, userNote } : it)),
    );
    copiedFeedback = false;
  }

  function removeItem(itemId: string) {
    persistItems(items.filter((it) => it.id !== itemId));
    stalePaths = [];
    staleConfirmed = false;
    copiedFeedback = false;
  }

  function clearAll() {
    persistItems([]);
    stalePaths = [];
    staleConfirmed = false;
    copiedFeedback = false;
  }

  function openReworkSheet(activeSessions: readonly SessionInfo[]) {
    mode = "rework";
    stalePaths = [];
    staleConfirmed = false;
    copiedFeedback = false;
    syncDefaultSessionSelection(activeSessions);
    sheetOpen = true;
  }

  function openCommitAssistSheet(activeSessions: readonly SessionInfo[]) {
    mode = "commit-assist";
    stalePaths = [];
    staleConfirmed = true;
    copiedFeedback = false;
    syncDefaultSessionSelection(activeSessions);
    sheetOpen = true;
  }

  function closeSheet() {
    freshnessReq.cancel();
    checkingFreshness = false;
    sheetOpen = false;
  }

  async function verifyWorktreeFreshness(): Promise<boolean> {
    if (mode !== "rework") return true;
    const worktreeItems = items.filter(
      (it) => it.sourceKind === "worktree" && it.capturedWorktreeIdentity,
    );
    if (worktreeItems.length === 0 || staleConfirmed) return true;

    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    if (!repoId || !wtId) return true;

    const req = freshnessReq.begin();
    checkingFreshness = true;
    try {
      const [uncom, task] = await Promise.all([
        gitReviews(repoId, wtId, "uncommitted", req.signal).catch(() => null),
        gitReviews(repoId, wtId, "task", req.signal).catch(() => null),
      ]);
      if (!req.current()) return false;

      const liveIdentityByPath = new SvelteMap<string, string>();
      if (task) {
        for (const f of task.files) {
          if (f.targetIdentity)
            liveIdentityByPath.set(f.path, f.targetIdentity);
        }
      }
      if (uncom) {
        for (const f of uncom.files) {
          if (f.area !== "staged" && f.targetIdentity) {
            liveIdentityByPath.set(f.path, f.targetIdentity);
          }
        }
      }

      const changed = new SvelteSet<string>();
      for (const item of worktreeItems) {
        const liveId = liveIdentityByPath.get(item.path);
        if (!liveId || liveId !== item.capturedWorktreeIdentity) {
          changed.add(item.path);
        }
      }

      if (changed.size > 0) {
        stalePaths = [...changed];
        staleConfirmed = false;
        return false;
      }
      stalePaths = [];
      staleConfirmed = true;
      return true;
    } finally {
      if (req.current()) checkingFreshness = false;
    }
  }

  function buildPayloadText(
    stagedChanges: readonly GitWorkingChange[],
  ): string {
    const view = bindings.getView();
    if (mode === "commit-assist") {
      return formatCommitDraftAssistPrompt(view.context, stagedChanges);
    }
    return formatReworkFeedbackMarkdown(
      view.context,
      items,
      new SvelteSet(stalePaths),
    );
  }

  async function copyMarkdown(stagedChanges: readonly GitWorkingChange[]) {
    const text = buildPayloadText(stagedChanges);
    if (!text.trim()) return;
    if (typeof navigator !== "undefined" && navigator.clipboard) {
      await navigator.clipboard.writeText(text);
    }
    copiedFeedback = true;
    bindings.patchView({
      statusBanner:
        mode === "commit-assist"
          ? "已复制暂存摘要起草提示词。"
          : "已复制结构化审阅意见 Markdown。",
    });
  }

  return {
    get items() {
      return items;
    },
    get sheetOpen() {
      return sheetOpen;
    },
    get mode() {
      return mode;
    },
    get selectedSessionId() {
      return selectedSessionId;
    },
    set selectedSessionId(id: string) {
      selectedSessionId = id;
    },
    get checkingFreshness() {
      return checkingFreshness;
    },
    get stalePaths() {
      return stalePaths;
    },
    get staleConfirmed() {
      return staleConfirmed;
    },
    get copiedFeedback() {
      return copiedFeedback;
    },
    dispose,
    loadTarget,
    syncDefaultSessionSelection,
    addItem,
    updateItemNote,
    removeItem,
    clearAll,
    openReworkSheet,
    openCommitAssistSheet,
    closeSheet,
    verifyWorktreeFreshness,
    confirmSendDespiteStale() {
      staleConfirmed = true;
    },
    buildPayloadText,
    copyMarkdown,
  };
}
