import {
  APIError,
  api,
  consumeTokenFragment,
  store,
  stored,
} from "../platform/api";
import { errorMessage } from "../platform/error";
import {
  CapabilitiesSchema,
  type Capabilities,
} from "../contracts/feature-schemas";
import {
  startSettings,
  stopSettings,
} from "../domains/settings/settings.svelte";
import { clearHomeCache } from "../domains/workspace/home-cache";
import { clearSessionListSnapshot } from "../hooks/sessions/session-list.svelte";
import { prepareHostAuthentication } from "../hooks/shared/host-authentication";
import { useRelayAuthRecovery } from "../hooks/shared/relay-auth-recovery.svelte";
import type { Route } from "../hooks/shared/route";
import {
  resetFeatureCapabilities,
  setFeatureCapabilities,
} from "../platform/feature-host.svelte";
import { clearRelayAuth, resetRelayAuth } from "../platform/relay-auth";
import { relayAuth, relayBlocked } from "../platform/relay-auth.svelte";
import { onActiveHostChange } from "../platform/storage";
import { watchFeatureHostRefresh } from "../features/host-refresh";
import {
  disposeFeatureContributions,
  loadFeatureContributions,
} from "../features/registry.svelte";

const BOOT_CONNECTED_KEY = "connected";

export type BootGate = "probing" | "auth" | "offline";

function hasReturningSession(tokenFromFragment: boolean): boolean {
  if (tokenFromFragment) return false;
  return stored(BOOT_CONNECTED_KEY) === "1" || stored("token") !== "";
}

export interface AppBootstrapCallbacks {
  route: () => Route;
  preloadSessionPage: () => void;
  onPathNormalized: () => void;
  onClearUiForAuth: (relay: boolean) => void;
  onRebuilding: (value: boolean) => void;
}

/** Coordinates initial authentication, capability discovery, relay recovery and host switching. */
export function createAppBootstrap(callbacks: AppBootstrapCallbacks) {
  let ready = $state(false);
  let rebuildingFeatures = $state(false);
  let checking = $state(true);
  let gate = $state<BootGate>("probing");
  let optimisticBoot = $state(
    hasReturningSession(location.hash.startsWith("#token=")),
  );
  let token = $state(stored("token"));
  let error = $state("");
  let authFailed = $state(false);
  let discoveryError = $state("");
  let discoveryFailureCause = $state("");

  let submittedToken = false;
  let bootstrapEpoch = 0;
  let bootstrapAbort: AbortController | undefined;

  const reconnect = useRelayAuthRecovery(
    () => ready,
    () => void check(),
  );

  async function check(preferToken = false) {
    checking = true;
    submittedToken = preferToken;
    const optimistic = !preferToken && optimisticBoot;
    if (optimistic && callbacks.route().name === "terminal") {
      callbacks.preloadSessionPage();
    }
    bootstrapAbort?.abort();
    const controller = new AbortController();
    bootstrapAbort = controller;
    const epoch = ++bootstrapEpoch;
    try {
      disposeFeatureContributions();
      resetFeatureCapabilities();
      let capabilitiesPromise: Promise<Capabilities> | undefined;
      const authenticated = await prepareHostAuthentication(
        preferToken,
        controller.signal,
        () => epoch === bootstrapEpoch,
        optimistic
          ? () => {
              capabilitiesPromise = api(
                "/capabilities",
                CapabilitiesSchema,
                "GET",
                undefined,
                controller.signal,
              );
              // Prevent an unhandled rejection if /health fails before capabilities is awaited.
              void capabilitiesPromise.catch(() => undefined);
            }
          : undefined,
      );
      if (!authenticated) {
        if (relayAuth.reason === "reconnecting") reconnect.start();
        return;
      }
      discoveryError = "";
      discoveryFailureCause = "";
      try {
        const capabilities = await (capabilitiesPromise ??
          api(
            "/capabilities",
            CapabilitiesSchema,
            "GET",
            undefined,
            controller.signal,
          ));
        if (epoch !== bootstrapEpoch) return;
        setFeatureCapabilities(capabilities);
        callbacks.onPathNormalized();
        const currentRoute = callbacks.route();
        await loadFeatureContributions({
          forwardAuth: currentRoute.name === "forward-auth",
          codePopout: currentRoute.name === "code",
        });
      } catch (discoveryFailure) {
        if (epoch !== bootstrapEpoch) return;
        resetFeatureCapabilities();
        disposeFeatureContributions();
        discoveryFailureCause = errorMessage(discoveryFailure);
        callbacks.onPathNormalized();
      }
      if (epoch !== bootstrapEpoch) return;
      store(BOOT_CONNECTED_KEY, "1");
      startSettings();
      ready = true;
      gate = "probing";
      optimisticBoot = true;
      authFailed = false;
      submittedToken = false;
      error = "";
    } catch (err) {
      if (epoch === bootstrapEpoch) {
        optimisticBoot = false;
        if (err instanceof APIError && err.status === 401) {
          store(BOOT_CONNECTED_KEY, null);
          gate = "auth";
          authFailed = submittedToken || token.length > 0;
          error = authFailed ? "" : errorMessage(err);
        } else {
          gate = "offline";
          authFailed = false;
          error = errorMessage(err);
        }
      }
    } finally {
      if (epoch === bootstrapEpoch) checking = false;
    }
  }

  function start(): () => void {
    const featureRefresh = watchFeatureHostRefresh({
      ready: () => ready && !relayBlocked(),
      epoch: () => bootstrapEpoch,
      context: () => {
        const currentRoute = callbacks.route();
        return {
          forwardAuth: currentRoute.name === "forward-auth",
          codePopout: currentRoute.name === "code",
        };
      },
      onRebuilding: (value) => {
        rebuildingFeatures = value;
        callbacks.onRebuilding(value);
      },
      onError: (message) => (discoveryError = message),
    });

    const resetAuth = (clearSavedConnection: boolean) => {
      const relay = relayAuth.relay;
      if (relay && relayAuth.authenticated) clearRelayAuth("reconnecting");
      else if (!relay) {
        reconnect.cancel();
        clearRelayAuth();
      }
      const wasReady = ready;
      bootstrapEpoch++;
      bootstrapAbort?.abort();
      featureRefresh.cancel();
      stopSettings();
      checking = false;
      error = "";
      callbacks.onClearUiForAuth(relay);
      if (relay) {
        if (relayAuth.reason === "reconnecting") reconnect.start();
        else reconnect.cancel();
        return;
      }
      disposeFeatureContributions();
      rebuildingFeatures = false;
      resetFeatureCapabilities();
      clearHomeCache();
      clearSessionListSnapshot();
      ready = false;
      if (clearSavedConnection) {
        store(BOOT_CONNECTED_KEY, null);
        optimisticBoot = false;
        gate = "auth";
        authFailed = wasReady || submittedToken || token.length > 0;
      }
    };

    const auth = () => resetAuth(true);
    window.addEventListener("webmux:auth", auth);
    window.addEventListener("webmux:relay-check", auth);
    const tokenFromFragment = consumeTokenFragment();
    if (tokenFromFragment) {
      token = stored("token");
      optimisticBoot = false;
    }
    const disconnectHost = onActiveHostChange(() => {
      resetRelayAuth();
      resetAuth(false);
      token = stored("token");
      authFailed = false;
      submittedToken = false;
      gate = "probing";
      optimisticBoot = hasReturningSession(false);
      void check();
    });
    void check(tokenFromFragment);
    return () => {
      reconnect.cancel();
      disconnectHost();
      featureRefresh.dispose();
      bootstrapAbort?.abort();
      bootstrapEpoch++;
      disposeFeatureContributions();
      resetFeatureCapabilities();
      stopSettings();
      window.removeEventListener("webmux:auth", auth);
      window.removeEventListener("webmux:relay-check", auth);
    };
  }

  return {
    get ready() {
      return ready;
    },
    get rebuildingFeatures() {
      return rebuildingFeatures;
    },
    get checking() {
      return checking;
    },
    get gate() {
      return gate;
    },
    get optimisticBoot() {
      return optimisticBoot;
    },
    get token() {
      return token;
    },
    set token(value: string) {
      token = value;
    },
    get error() {
      return error;
    },
    get authFailed() {
      return authFailed;
    },
    get discoveryError() {
      return discoveryError;
    },
    get discoveryFailureCause() {
      return discoveryFailureCause;
    },
    get reconnectChecking() {
      return reconnect.checking;
    },
    start,
    connectWithToken() {
      store("token", token);
      error = "";
      authFailed = false;
      void check(true);
    },
    retryConnection() {
      error = "";
      authFailed = false;
      void check();
    },
    switchToTokenGate() {
      gate = "auth";
      authFailed = false;
      error = "";
    },
    retryRelayReconnect() {
      reconnect.start();
    },
  };
}
