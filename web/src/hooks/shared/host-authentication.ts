import { api } from "../../platform/api";
import { HealthSchema } from "../../contracts/schemas";
import { RelayAuthSessionSchema } from "../../contracts/relay-schemas";
import {
  applyRelayAuth,
  clearRelayAuth,
  relayAuthRevision,
} from "../../platform/relay-auth";

/** Restores owner authorization before probing the host; explicit Token login wins. */
export async function prepareHostAuthentication(
  preferToken: boolean,
  signal: AbortSignal,
  current: () => boolean,
) {
  if (preferToken) clearRelayAuth();
  const authRevision = relayAuthRevision();
  if (!preferToken) {
    try {
      const session = await api(
        "/relay/auth/session",
        RelayAuthSessionSchema,
        "GET",
        undefined,
        signal,
      );
      if (!current()) return false;
      applyRelayAuth(session, authRevision);
    } catch {
      // Local-only and older hosts still use the existing Token login.
      if (!current()) return false;
      clearRelayAuth();
    }
  }
  await api("/health", HealthSchema, "GET", undefined, signal);
  return true;
}
