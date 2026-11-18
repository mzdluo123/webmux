import { m } from "$lib/paraglide/messages.js";
import { writable } from "svelte/store";
import {
  UpstreamChoiceSchema,
  WorktreeUpstreamSchema,
  WorktreeUpstreamErrorSchema,
  type UpstreamChoice,
  type WorktreeUpstream,
} from "../contracts/worktree-upstream-schemas";

type Pending = {
  challenge: WorktreeUpstream;
  finish: (useUpstream: boolean | null) => void;
};
// One application-wide queue: concurrent creation requests retain their own
// challenge, resolver and cancellation signal, including while queued.
let pending: Pending[] = [];
const current = writable<WorktreeUpstream | undefined>();
export const worktreeUpstreamConfirmation = {
  subscribe: current.subscribe,
  choose(useUpstream: boolean | null) {
    const current = pending[0];
    if (useUpstream && !current?.challenge.canUseUpstream) return;
    current?.finish(useUpstream);
  },
};

export function confirmWorktreeUpstream(
  input: WorktreeUpstream,
  signal?: AbortSignal,
): Promise<UpstreamChoice | null> {
  const challenge = WorktreeUpstreamSchema.parse(input);
  const { promise, resolve, reject } =
    Promise.withResolvers<UpstreamChoice | null>();
  if (signal?.aborted) {
    reject(signal.reason ?? new DOMException(m.s1_cancel(), "AbortError"));
    return promise;
  }
  const remove = () => {
    pending = pending.filter((item) => item !== entry);
    current.set(pending[0]?.challenge);
    signal?.removeEventListener("abort", abort);
  };
  const entry: Pending = {
    challenge,
    finish(useUpstream) {
      remove();
      resolve(
        useUpstream === null
          ? null
          : UpstreamChoiceSchema.parse({
              confirmationId: challenge.confirmationId,
              useUpstream,
            }),
      );
    },
  };
  const abort = () => {
    remove();
    reject(signal?.reason ?? new DOMException(m.s1_cancel(), "AbortError"));
  };
  signal?.addEventListener("abort", abort, { once: true });
  pending = [...pending, entry];
  current.set(pending[0]?.challenge);
  return promise;
}

/** Only replace the worktree choice; all uploaded files and prompts stay put. */
export async function resolveWorktreeUpstream(
  errorBody: unknown,
  payload: unknown,
  signal?: AbortSignal,
): Promise<unknown> {
  const challenge = WorktreeUpstreamErrorSchema.safeParse(errorBody);
  if (!challenge.success) return undefined;
  const choice = await confirmWorktreeUpstream(
    challenge.data.worktreeUpstream,
    signal,
  );
  if (!choice) throw new DOMException(m.s1_cancel(), "AbortError");
  const original = payload as Record<string, unknown>;
  return original.newWorktree
    ? {
        ...original,
        newWorktree: {
          ...(original.newWorktree as Record<string, unknown>),
          upstreamChoice: choice,
        },
      }
    : { ...original, upstreamChoice: choice };
}
