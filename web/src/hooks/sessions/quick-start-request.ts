import { api, APIError } from "../../platform/api";
import { newRequestId } from "../../platform/ids";
import {
  CreateSessionFailureSchema,
  CreateSessionRequestSchema,
  CreateSessionResultSchema,
  type ChatConfig,
  type CreateSessionFailure,
  type CreateSessionRequest,
} from "../../contracts/schemas";

/** The values the page shows for each option, as the request sends them. */
export function configValues(options: readonly ChatConfig[]) {
  return Object.fromEntries(
    options.flatMap((option) =>
      option.type === "boolean"
        ? option.enabled === undefined
          ? []
          : [[option.id, String(option.enabled)]]
        : option.value === undefined
          ? []
          : [[option.id, option.value]],
    ),
  );
}

export type QuickStartOutcome =
  | { ok: true; sessionId: string }
  | { ok: false; failure: CreateSessionFailure };

/**
 * Sends one quick start: a session created in a new worktree with its first
 * message. A dropped connection is retried once with the same request ID,
 * which the server answers with the first run's result.
 */
export async function sendQuickStart(
  request: Omit<CreateSessionRequest, "clientRequestId">,
  clientRequestId = newRequestId(),
): Promise<QuickStartOutcome> {
  const body = { ...request, clientRequestId };
  try {
    const result = await api(
      "/sessions",
      CreateSessionResultSchema,
      "POST",
      body,
      undefined,
      CreateSessionRequestSchema,
      true,
    );
    return { ok: true, sessionId: result.session.id };
  } catch (err) {
    const failure =
      err instanceof APIError
        ? CreateSessionFailureSchema.safeParse(err.body).data
        : undefined;
    return {
      ok: false,
      failure: failure ?? {
        error: err instanceof Error ? err.message : String(err),
        step: "precheck",
      },
    };
  }
}
