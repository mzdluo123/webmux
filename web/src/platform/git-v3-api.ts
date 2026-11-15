import { api } from "./api";
import { worktreeGitBase } from "./git-api";
import {
  GitDiscardPreviewRequestSchema,
  GitDiscardPreviewSchema,
  GitDiscardRequestSchema,
  GitDiscardResultSchema,
  GitSmartCheckoutPreviewRequestSchema,
  GitSmartCheckoutPreviewSchema,
  GitSmartCheckoutRequestSchema,
  GitSmartCheckoutResultSchema,
  type GitDiscardPreviewRequest,
  type GitDiscardRequest,
  type GitSmartCheckoutPreviewRequest,
  type GitSmartCheckoutRequest,
} from "../contracts/schemas";

export function gitDiscardPreview(
  repositoryId: string,
  worktreeId: string,
  request: GitDiscardPreviewRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeGitBase(repositoryId, worktreeId)}/discard-preview`,
    GitDiscardPreviewSchema,
    "POST",
    request,
    signal,
    GitDiscardPreviewRequestSchema,
  );
}

export function gitDiscard(
  repositoryId: string,
  worktreeId: string,
  request: GitDiscardRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeGitBase(repositoryId, worktreeId)}/discard`,
    GitDiscardResultSchema,
    "POST",
    request,
    signal,
    GitDiscardRequestSchema,
  );
}

export function gitSmartCheckoutPreview(
  repositoryId: string,
  worktreeId: string,
  request: GitSmartCheckoutPreviewRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeGitBase(repositoryId, worktreeId)}/smart-checkout-preview`,
    GitSmartCheckoutPreviewSchema,
    "POST",
    request,
    signal,
    GitSmartCheckoutPreviewRequestSchema,
  );
}

export function gitSmartCheckout(
  repositoryId: string,
  worktreeId: string,
  request: GitSmartCheckoutRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeGitBase(repositoryId, worktreeId)}/smart-checkout`,
    GitSmartCheckoutResultSchema,
    "POST",
    request,
    signal,
    GitSmartCheckoutRequestSchema,
  );
}
