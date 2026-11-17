import { gitCommitPreview } from "../../platform/git-api";
import type { GitWorkbenchViewState } from "../../contracts/git-workbench-schemas";
import type { GitCommitPreviewRequest } from "../../contracts/schemas";
import { validateAuthorOverrideInput } from "../../domains/workspace/git-v2-adapter";
import { writeGitTargetMemory } from "../../domains/workspace/git-workbench-memory";
import { LatestRequest } from "../shared/remote-request.svelte";

export interface GitAmendDraftPrompt {
  headMessage: string;
  currentDraft: string;
}

export interface GitCommitOptionsDeps {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  onUpdateDraftMessage: (nextMessage: string) => void;
}

export function createGitCommitOptionsController(deps: GitCommitOptionsDeps) {
  let expanded = $state(false);
  let amend = $state(false);
  let authorOverride = $state("");
  let signoff = $state(false);
  let acknowledgePushedAmend = $state(false);
  let headMessage = $state("");
  let amendPushed = $state(false);
  let signoffTrailer = $state("");
  let loadingHead = $state(false);
  let draftPrompt = $state<GitAmendDraftPrompt | null>(null);

  const headReq = new LatestRequest();
  let disposed = false;

  const authorValidation = $derived.by(() =>
    validateAuthorOverrideInput(authorOverride),
  );

  function syncToView() {
    deps.patchView({
      commitAmend: amend,
      commitAmendPushed: amend && amendPushed,
      commitAuthorOverride: authorOverride,
      commitSignoff: signoff,
    });
  }

  function dispose() {
    disposed = true;
    headReq.dispose();
  }

  function resetTarget() {
    headReq.cancel();
    expanded = false;
    amend = false;
    authorOverride = "";
    signoff = false;
    acknowledgePushedAmend = false;
    headMessage = "";
    amendPushed = false;
    signoffTrailer = "";
    loadingHead = false;
    draftPrompt = null;
  }

  function resetAfterCommit() {
    amend = false;
    authorOverride = "";
    signoff = false;
    acknowledgePushedAmend = false;
    amendPushed = false;
    draftPrompt = null;
    syncToView();
  }

  function buildPreviewRequest(): GitCommitPreviewRequest | undefined {
    const cleanAuthor = authorOverride.trim();
    if (!amend && !cleanAuthor && !signoff) return undefined;
    return {
      ...(amend ? { amend: true } : {}),
      ...(cleanAuthor ? { authorOverride: cleanAuthor } : {}),
      ...(signoff ? { signoff: true } : {}),
    };
  }

  function ingestPreflightMetadata(meta: {
    headMessage?: string;
    amendPushed?: boolean;
    signoffTrailer?: string;
    committerName?: string;
    committerEmail?: string;
  }) {
    if (meta.headMessage !== undefined) {
      headMessage = meta.headMessage;
    }
    if (meta.amendPushed !== undefined) {
      amendPushed = meta.amendPushed;
    }
    if (meta.signoffTrailer) {
      signoffTrailer = meta.signoffTrailer;
    } else if (meta.committerName && meta.committerEmail) {
      signoffTrailer = `Signed-off-by: ${meta.committerName} <${meta.committerEmail}>`;
    }
    syncToView();
  }

  async function toggleAmend(next: boolean) {
    if (!next) {
      amend = false;
      acknowledgePushedAmend = false;
      draftPrompt = null;
      syncToView();
      return;
    }

    expanded = true;
    amend = true;
    acknowledgePushedAmend = false;
    syncToView();

    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !wtId || disposed) return;

    loadingHead = true;
    const req = headReq.begin();
    try {
      const preview = await gitCommitPreview(
        repoId,
        wtId,
        { amend: true, signoff },
        req.signal,
      );
      if (
        !req.current() ||
        disposed ||
        !amend ||
        repoId !== deps.getRepoId() ||
        wtId !== deps.getWorktreeId()
      ) {
        return;
      }
      const loadedHeadMsg = (preview.headMessage ?? "").trim();
      headMessage = loadedHeadMsg;
      amendPushed = Boolean(preview.amendPushed);
      if (preview.identity.signoffTrailer) {
        signoffTrailer = preview.identity.signoffTrailer;
      }
      syncToView();

      if (!loadedHeadMsg) return;
      const curDraft = deps.getView().commitMessage.trim();
      if (!curDraft) {
        deps.onUpdateDraftMessage(loadedHeadMsg);
        writeGitTargetMemory(repoId, wtId, { commitMessage: loadedHeadMsg });
        draftPrompt = null;
      } else if (curDraft !== loadedHeadMsg) {
        draftPrompt = {
          headMessage: loadedHeadMsg,
          currentDraft: curDraft,
        };
      }
    } catch {
      /* Error surfaces cleanly during explicit commit preview if worktree is unborn */
    } finally {
      if (req.current()) loadingHead = false;
    }
  }

  function resolveAmendDraft(choice: "replace" | "merge" | "keep") {
    const cur = draftPrompt;
    if (!cur) return;
    draftPrompt = null;
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (choice === "keep") return;
    const nextMsg =
      choice === "replace"
        ? cur.headMessage
        : `${cur.currentDraft}\n\n${cur.headMessage}`;
    deps.onUpdateDraftMessage(nextMsg);
    if (repoId && wtId) {
      writeGitTargetMemory(repoId, wtId, { commitMessage: nextMsg });
    }
  }

  function setAuthorOverride(value: string) {
    authorOverride = value;
    if (value.trim()) expanded = true;
    syncToView();
  }

  function setSignoff(next: boolean) {
    signoff = next;
    if (next) expanded = true;
    const ctx = deps.getView().context;
    if (next && !signoffTrailer && ctx.committerName && ctx.committerEmail) {
      signoffTrailer = `Signed-off-by: ${ctx.committerName} <${ctx.committerEmail}>`;
    }
    syncToView();
  }

  return {
    get expanded() {
      return expanded;
    },
    setExpanded(next: boolean) {
      expanded = next;
    },
    get amend() {
      return amend;
    },
    get authorOverride() {
      return authorOverride;
    },
    get authorValidation() {
      return authorValidation;
    },
    get signoff() {
      return signoff;
    },
    get acknowledgePushedAmend() {
      return acknowledgePushedAmend;
    },
    setAcknowledgePushedAmend(next: boolean) {
      acknowledgePushedAmend = next;
    },
    get headMessage() {
      return headMessage;
    },
    get amendPushed() {
      return amendPushed;
    },
    get signoffTrailer() {
      return signoffTrailer;
    },
    get loadingHead() {
      return loadingHead;
    },
    get draftPrompt() {
      return draftPrompt;
    },
    dispose,
    resetTarget,
    resetAfterCommit,
    buildPreviewRequest,
    ingestPreflightMetadata,
    toggleAmend,
    resolveAmendDraft,
    setAuthorOverride,
    setSignoff,
  };
}

export type GitCommitOptionsController = ReturnType<
  typeof createGitCommitOptionsController
>;
