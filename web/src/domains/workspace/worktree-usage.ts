import { api } from "../../platform/api";
import {
  EmptyRequestSchema,
  WorktreeUsageSchema,
  type Worktree,
} from "../../contracts/schemas";

/** Measure a worktree on demand without loading chat recovery services. */
export function measureWorktree(worktree: Worktree, signal?: AbortSignal) {
  return api(
    `/repositories/${worktree.repositoryId}/worktrees/${worktree.id}/usage`,
    WorktreeUsageSchema,
    "POST",
    {},
    signal,
    EmptyRequestSchema,
  );
}
