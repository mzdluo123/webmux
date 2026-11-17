import { api } from "../../platform/api";
import { HealthSchema } from "../../contracts/schemas";
import { RelayAuthSessionSchema } from "../../contracts/relay-schemas";
import {
  applyRelayAuth,
  resetRelayAuth,
  relayAuthStatus,
  relayAuthRevision,
} from "../../platform/relay-auth";

/** Restores owner authorization before probing the host; explicit Token login wins. */
export async function prepareHostAuthentication(
  preferToken: boolean,
  signal: AbortSignal,
  current: () => boolean,
) {
  const restoreOwner = !preferToken;
  if (!restoreOwner) resetRelayAuth();
  const authRevision = relayAuthRevision();
  if (restoreOwner) {
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
      if (session.relay && !session.authenticated) return false;
    } catch {
      // Local-only and older hosts still use the existing Token login.
      if (!current()) return false;
      if (relayAuthStatus().relay) return false;
      resetRelayAuth();
    }
  }
  await api("/health", HealthSchema, "GET", undefined, signal);
  return true;
}
