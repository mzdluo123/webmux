import { describe, expect, test } from "vitest";
import { GitOperationSchema } from "../src/contracts/git-api-schemas";
import type { GitOperation } from "../src/contracts/git-workbench-schemas";
import {
  findRestorableAttentionOperation,
  generateGitClientRequestId,
  gitOperationKindLabel,
  gitOperationStatusLabel,
  isRecoverableNetworkFailure,
  isTerminalGitOperationStatus,
  mapGitOperationToCommitOutcome,
} from "../src/domains/workspace/git-operations-model";
import { APIError } from "../src/platform/api";

function makeOperation(overrides: Partial<GitOperation> = {}): GitOperation {
  return GitOperationSchema.parse({
    id: "op-commit-001",
    repositoryId: "repo-1",
    worktreeId: "wt-main",
    clientRequestId: "commit-abc123",
    kind: "commit",
    status: "succeeded",
    preview: {
      worktreePath: "/work/webmux",
      headState: "branch",
      branch: "main",
      headOid: "1111111111111111111111111111111111111111",
      expectedTreeOid: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    },
    subject: "feat(git): persistent operation ledger",
    actualOid: "2222222222222222222222222222222222222222",
    actualTreeOid: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    parents: ["1111111111111111111111111111111111111111"],
    fetchResults: [],
    reviewReasons: [],
    summary: "已提交 2222222：feat(git): persistent operation ledger",
    sanitizedOutput: "[main 2222222] feat(git): persistent operation ledger",
    startedAt: "2026-04-14T10:00:00Z",
    finishedAt: "2026-04-14T10:00:01Z",
    updatedAt: "2026-04-14T10:00:01Z",
    ...overrides,
  });
}

describe("git-operations-model", () => {
  test("generates prefixed clientRequestId and classifies recoverable network errors", () => {
    const id1 = generateGitClientRequestId("commit");
    const id2 = generateGitClientRequestId("commit");
    expect(id1).toMatch(/^commit-[a-z0-9]+$/);
    expect(id1).not.toBe(id2);

    expect(isRecoverableNetworkFailure(new TypeError("Failed to fetch"))).toBe(
      true,
    );
    expect(isRecoverableNetworkFailure(new APIError(502, "Bad Gateway"))).toBe(
      true,
    );
    expect(
      isRecoverableNetworkFailure(new APIError(408, "Request Timeout")),
    ).toBe(true);
    expect(
      isRecoverableNetworkFailure(new APIError(409, "conflict", "stale")),
    ).toBe(false);
    expect(
      isRecoverableNetworkFailure(new DOMException("Aborted", "AbortError")),
    ).toBe(false);
  });

  test("maps succeeded, needs_review, and uncertain operations to commit outcomes", () => {
    expect(isTerminalGitOperationStatus("pending")).toBe(false);
    expect(isTerminalGitOperationStatus("running")).toBe(false);
    expect(isTerminalGitOperationStatus("succeeded")).toBe(true);
    expect(isTerminalGitOperationStatus("uncertain")).toBe(true);

    expect(gitOperationKindLabel("fast_forward")).toBe("快进更新");
    expect(gitOperationStatusLabel("uncertain")).toBe("结果待核实");

    const matched = mapGitOperationToCommitOutcome(makeOperation());
    expect(matched?.status).toBe("matched");
    expect(matched?.operationId).toBe("op-commit-001");
    expect(matched?.commitOid).toBe("2222222222222222222222222222222222222222");

    const pushSucceeded = mapGitOperationToCommitOutcome(
      makeOperation({ kind: "push", status: "succeeded" }),
    );
    expect(pushSucceeded).toBeNull();

    const uncertainPush = mapGitOperationToCommitOutcome(
      makeOperation({
        id: "op-push-999",
        kind: "push",
        status: "uncertain",
        summary: "服务重启时推送中断，远端引用尚未更新，请先 Fetch 核实。",
        reviewReasons: ["服务在推送执行期间重启，未自动重试远端写操作。"],
      }),
    );
    expect(uncertainPush?.status).toBe("uncertain");
    expect(uncertainPush?.operationId).toBe("op-push-999");

    const ops = [
      makeOperation({ id: "op-ok", status: "succeeded" }),
      makeOperation({
        id: "op-review",
        status: "needs_review",
        reviewReasons: ["pre-commit hook 改写了提交树"],
      }),
      makeOperation({ id: "op-uncertain", status: "uncertain" }),
    ];
    expect(findRestorableAttentionOperation(ops, new Set())?.id).toBe(
      "op-review",
    );
    expect(
      findRestorableAttentionOperation(ops, new Set(["op-review"]))?.id,
    ).toBe("op-uncertain");
  });
});
