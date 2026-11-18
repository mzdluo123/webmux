import { api } from "../../platform/api";
import { RelayAuthSessionSchema } from "../../contracts/relay-schemas";
import {
  applyRelayAuth,
  relayAuthRevision,
  relayAuthStatus,
} from "../../platform/relay-auth";

// A disconnect gets one finite recovery window, never an automatic login loop.
const delays = [0, 500, 1000, 2000, 4000, 8000, 8000];
const probeTimeout = 4000;

export function createRelayAuthRecovery(options: {
  onChecking: (checking: boolean) => void;
  onRecovered: () => void;
}) {
  let generation = 0;
  let active = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let timeout: ReturnType<typeof setTimeout> | undefined;
  let request: AbortController | undefined;

  function cancel() {
    generation++;
    active = false;
    clearTimeout(timer);
    clearTimeout(timeout);
    request?.abort();
    request = undefined;
    options.onChecking(false);
  }

  async function probe(attempt: number, epoch: number) {
    if (epoch !== generation) return;
    if (relayAuthStatus().reason !== "reconnecting") {
      cancel();
      return;
    }
    const controller = new AbortController();
    request = controller;
    timeout = setTimeout(() => controller.abort(), probeTimeout);
    const revision = relayAuthRevision();
    try {
      const session = await api(
        "/relay/auth/session",
        RelayAuthSessionSchema,
        "GET",
        undefined,
        controller.signal,
      );
      if (epoch !== generation || controller.signal.aborted) return;
      // Recovery must not silently downgrade a trusted relay into Token mode.
      if (session.relay === true) applyRelayAuth(session, revision);
    } catch {
      // Network errors and host restarts spend the same bounded retry budget.
    } finally {
      if (epoch === generation) {
        clearTimeout(timeout);
        request = undefined;
        const status = relayAuthStatus();
        if (status.authenticated) {
          cancel();
          options.onRecovered();
        } else if (
          status.reason !== "reconnecting" ||
          attempt + 1 === delays.length
        ) {
          cancel();
        } else {
          timer = setTimeout(
            () => void probe(attempt + 1, epoch),
            delays[attempt + 1],
          );
        }
      }
    }
  }

  return {
    start() {
      if (active || relayAuthStatus().reason !== "reconnecting") return;
      active = true;
      options.onChecking(true);
      void probe(0, ++generation);
    },
    cancel,
  };
}
