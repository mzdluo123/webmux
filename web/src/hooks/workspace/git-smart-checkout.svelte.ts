import { APIError } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import type {
  GitSmartCheckoutPreview,
  GitSmartCheckoutPreviewRequest,
  GitSmartCheckoutRequest,
  GitSmartCheckoutResult,
  SwitchPreview,
  SwitchTarget,
  Worktree,
} from "../../contracts/schemas";
import { generateGitClientRequestId } from "../../domains/workspace/git-operations-model";
import { LatestRequest } from "../shared/remote-request.svelte";

const HARD_SWITCH_BLOCKERS = new Set([
  "occupied",
  "exists",
  "same",
  "operation",
  "locked",
  "unavailable",
  "unsupported",
]);

export function shouldOfferSmartCheckout(
  preview: SwitchPreview | null,
  hasSmartSource: boolean,
): boolean {
  if (!hasSmartSource || !preview) return false;
  const hasChangesBlocker = preview.blockers.some((b) => b.code === "changes");
  if (!hasChangesBlocker) return false;
  return !preview.blockers.some((b) => HARD_SWITCH_BLOCKERS.has(b.code));
}

function defaultIncludeUntracked(worktree: Worktree): boolean {
  return Boolean(
    worktree.hasUntracked || (worktree.changes?.untracked ?? 0) > 0,
  );
}

interface SmartCheckoutSource {
  smartPreview?: (
    tree: Worktree,
    request: GitSmartCheckoutPreviewRequest,
    signal: AbortSignal,
  ) => Promise<GitSmartCheckoutPreview>;
  smartCheckout?: (
    tree: Worktree,
    request: GitSmartCheckoutRequest,
  ) => Promise<GitSmartCheckoutResult>;
}

export interface SmartCheckoutDeps {
  getWorktree: () => Worktree;
  getTarget: () => SwitchTarget | null;
  getPlainPreview: () => SwitchPreview | null;
  getBusy: () => boolean;
  setBusy: (busy: boolean) => void;
  getSource: () => SmartCheckoutSource;
  onDone: (tree: Worktree) => void;
  onChange: () => void;
  onReloadPlainPreview: (target: SwitchTarget) => void;
}

export function createSmartCheckoutController(deps: SmartCheckoutDeps) {
  let includeUntracked = $state(defaultIncludeUntracked(deps.getWorktree()));
  let preview = $state<GitSmartCheckoutPreview | null>(null);
  let previewing = $state(false);
  let previewError = $state("");
  let conflictMessage = $state("");
  let execError = $state("");
  let acknowledgeSessions = $state(false);
  let disclosureOpen = $state(false);
  let lastResult = $state<GitSmartCheckoutResult | null>(null);

  const reqs = new LatestRequest();

  const hasSmartSource = () => {
    const src = deps.getSource();
    return Boolean(src.smartPreview && src.smartCheckout);
  };

  const visible = $derived(
    shouldOfferSmartCheckout(deps.getPlainPreview(), hasSmartSource()),
  );

  const hasActiveSessions = $derived((preview?.sessions.length ?? 0) > 0);

  const canExecute = $derived(
    Boolean(
      preview &&
      !previewing &&
      !deps.getBusy() &&
      preview.confirmationId !== "" &&
      preview.blockers.length === 0 &&
      (!hasActiveSessions || (disclosureOpen && acknowledgeSessions)),
    ),
  );

  async function loadSmartPreview(
    tree: Worktree,
    target: SwitchTarget,
    untracked: boolean,
  ) {
    const smartPreview = deps.getSource().smartPreview;
    if (!smartPreview) return;
    const req = reqs.begin();
    previewing = true;
    previewError = "";
    try {
      const res = await smartPreview(
        tree,
        { target, includeUntracked: untracked },
        req.signal,
      );
      if (!req.current()) return;
      preview = res;
      acknowledgeSessions = false;
      if (res.sessions.length === 0) disclosureOpen = true;
    } catch (err) {
      if (req.reportable()) {
        preview = null;
        previewError = errorMessage(err);
      }
    } finally {
      if (req.current()) previewing = false;
    }
  }

  function resetAll() {
    reqs.cancel();
    preview = null;
    previewing = false;
    previewError = "";
    conflictMessage = "";
    execError = "";
    acknowledgeSessions = false;
    disclosureOpen = false;
    lastResult = null;
  }

  function syncPlainPreview(plainPreview: SwitchPreview | null) {
    const target = deps.getTarget();
    if (!target || !shouldOfferSmartCheckout(plainPreview, hasSmartSource())) {
      reqs.cancel();
      preview = null;
      previewing = false;
      previewError = "";
      return;
    }
    disclosureOpen = false;
    void loadSmartPreview(deps.getWorktree(), target, includeUntracked);
  }

  function setIncludeUntracked(next: boolean) {
    if (includeUntracked === next) return;
    includeUntracked = next;
    const target = deps.getTarget();
    if (target && visible) {
      void loadSmartPreview(deps.getWorktree(), target, next);
    }
  }

  async function execute() {
    const curPreview = preview;
    const target = deps.getTarget();
    const smartCheckout = deps.getSource().smartCheckout;
    if (!canExecute || !curPreview || !target || !smartCheckout) return;

    reqs.cancel();
    deps.setBusy(true);
    execError = "";
    conflictMessage = "";
    lastResult = null;
    try {
      const res = await smartCheckout(deps.getWorktree(), {
        confirmationId: curPreview.confirmationId,
        clientRequestId: generateGitClientRequestId("smart_checkout"),
        acknowledgeSessions,
      });
      deps.onChange();
      if (res.outcome === "completed") {
        deps.onDone(res.worktree);
        return;
      }
      lastResult = res;
      preview = null;
      acknowledgeSessions = false;
      if (res.outcome === "rolled_back") {
        deps.onReloadPlainPreview(target);
      }
    } catch (err) {
      if (err instanceof APIError && err.status === 409) {
        conflictMessage = errorMessage(err);
      } else {
        execError = errorMessage(err);
      }
      deps.onChange();
      preview = null;
      acknowledgeSessions = false;
      deps.onReloadPlainPreview(target);
    } finally {
      deps.setBusy(false);
    }
  }

  function dispose() {
    reqs.dispose();
  }

  return {
    get visible() {
      return visible;
    },
    get includeUntracked() {
      return includeUntracked;
    },
    get preview() {
      return preview;
    },
    get previewing() {
      return previewing;
    },
    get previewError() {
      return previewError;
    },
    get conflictMessage() {
      return conflictMessage;
    },
    get execError() {
      return execError;
    },
    get acknowledgeSessions() {
      return acknowledgeSessions;
    },
    set acknowledgeSessions(next: boolean) {
      acknowledgeSessions = next;
    },
    get disclosureOpen() {
      return disclosureOpen;
    },
    set disclosureOpen(next: boolean) {
      disclosureOpen = next;
      if (!next) acknowledgeSessions = false;
    },
    get hasActiveSessions() {
      return hasActiveSessions;
    },
    get canExecute() {
      return canExecute;
    },
    get lastResult() {
      return lastResult;
    },
    resetAll,
    syncPlainPreview,
    setIncludeUntracked,
    execute,
    dispose,
  };
}
