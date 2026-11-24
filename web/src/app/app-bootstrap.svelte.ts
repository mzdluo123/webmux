import {
  APIError,
  api,
  consumeTokenFragment,
  navigate,
  store,
  stored,
} from "../platform/api";
import {
  clearCachedStepUp,
  fetchAuthStatus,
  loginWithPasskey,
  loginWithRecoveryCode,
} from "../platform/auth-api";
import { errorMessage } from "../platform/error";
import type { AuthStatus } from "../contracts/auth-schemas";
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
import { runOnboardingEnterTransition } from "../hooks/shared/onboarding-enter-transition";
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

export type BootGate =
  | "probing"
  | "auth"
  | "offline"
  | "onboarding"
  | "pair"
  | "storage_error";

function isAuthFlowPath(pathname: string): boolean {
  return pathname === "/onboarding" || pathname === "/pair";
}

function hasReturningSession(tokenFromFragment: boolean): boolean {
  if (tokenFromFragment || isAuthFlowPath(location.pathname)) return false;
  return stored(BOOT_CONNECTED_KEY) === "1" || stored("token") !== "";
}

export interface AppBootstrapCallbacks {
  route: () => Route;
  preloadSessionPage: () => void;
  onPathNormalized: () => void;
  onClearUiForAuth: (relay: boolean) => void;
  onRebuilding: (value: boolean) => void;
}

/** Coordinates initial authentication, Passkey onboarding, capability discovery, relay recovery and host switching. */
export function createAppBootstrap(callbacks: AppBootstrapCallbacks) {
  let ready = $state(false);
  let rebuildingFeatures = $state(false);
  let checking = $state(true);
  let gate = $state<BootGate>("probing");
  let authStatus = $state<AuthStatus | null>(null);
  let optimisticBoot = $state(
    hasReturningSession(location.hash.startsWith("#token=")),
  );
  let token = $state(stored("token"));
  let error = $state("");
  let authFailed = $state(false);
  let discoveryError = $state("");
  let discoveryFailureCause = $state("");

  let submittedToken = false;
  let pendingOnboardingTransition = false;
  let bootstrapEpoch = 0;
  let bootstrapAbort: AbortController | undefined;

  const reconnect = useRelayAuthRecovery(
    () => ready,
    () => void check(),
  );

  async function probeAuthStatus(
    signal: AbortSignal,
  ): Promise<AuthStatus | null> {
    try {
      const st = await fetchAuthStatus(signal);
      authStatus = st;
      return st;
    } catch (err) {
      if (err instanceof APIError && err.status === 404) {
        authStatus = null;
        return null;
      }
      throw err;
    }
  }

  async function check(preferToken = false) {
    checking = true;
    submittedToken = preferToken;
    const currentRoute = callbacks.route();
    const optimistic = !preferToken && optimisticBoot;
    if (optimistic && currentRoute.name === "terminal") {
      callbacks.preloadSessionPage();
    }
    bootstrapAbort?.abort();
    const controller = new AbortController();
    bootstrapAbort = controller;
    const epoch = ++bootstrapEpoch;
    try {
      disposeFeatureContributions();
      resetFeatureCapabilities();

      // Always inspect auth status first when not on optimistic fast path, or
      // when entering /onboarding or /pair directly.
      if (!optimistic || isAuthFlowPath(location.pathname)) {
        const st = await probeAuthStatus(controller.signal);
        if (epoch !== bootstrapEpoch) return;
        if (st?.state === "needs_onboarding") {
          optimisticBoot = false;
          store(BOOT_CONNECTED_KEY, null);
          gate = "onboarding";
          error = "";
          authFailed = false;
          return;
        }
        if (st?.state === "storage_error") {
          optimisticBoot = false;
          store(BOOT_CONNECTED_KEY, null);
          gate = "storage_error";
          error = st.error ?? "";
          authFailed = false;
          return;
        }
        if (currentRoute.name === "pair" && st?.state === "ready") {
          optimisticBoot = false;
          gate = "pair";
          error = "";
          authFailed = false;
          return;
        }
      }

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
        const latestRoute = callbacks.route();
        if (latestRoute.name === "onboarding" || latestRoute.name === "pair") {
          navigate("/");
        }
        await loadFeatureContributions({
          forwardAuth: latestRoute.name === "forward-auth",
          codePopout: latestRoute.name === "code",
        });
      } catch (discoveryFailure) {
        if (epoch !== bootstrapEpoch) return;
        resetFeatureCapabilities();
        disposeFeatureContributions();
        discoveryFailureCause = errorMessage(discoveryFailure);
        callbacks.onPathNormalized();
      }
      if (epoch !== bootstrapEpoch) return;
      const animateOnboardingEnter = pendingOnboardingTransition;
      pendingOnboardingTransition = false;
      const commitReady = () => {
        ready = true;
        gate = "probing";
        optimisticBoot = true;
        authFailed = false;
        submittedToken = false;
        error = "";
      };
      store(BOOT_CONNECTED_KEY, "1");
      startSettings();
      if (animateOnboardingEnter) {
        await runOnboardingEnterTransition(commitReady);
      } else {
        commitReady();
      }
    } catch (err) {
      if (epoch === bootstrapEpoch) {
        pendingOnboardingTransition = false;
        optimisticBoot = false;
        if (err instanceof APIError && err.status === 401) {
          store(BOOT_CONNECTED_KEY, null);
          try {
            const st = await probeAuthStatus(controller.signal);
            if (epoch !== bootstrapEpoch) return;
            if (st?.state === "needs_onboarding") {
              gate = "onboarding";
              error = "";
              authFailed = false;
              return;
            }
            if (st?.state === "storage_error") {
              gate = "storage_error";
              error = st.error ?? "";
              authFailed = false;
              return;
            }
          } catch {
            // Keep 401 gate below.
          }
          gate = callbacks.route().name === "pair" ? "pair" : "auth";
          authFailed =
            authStatus?.state !== "ready" &&
            (submittedToken || token.length > 0);
          error = authFailed ? "" : "";
        } else if (err instanceof APIError && err.status === 503) {
          try {
            const st = await probeAuthStatus(controller.signal);
            if (epoch !== bootstrapEpoch) return;
            if (st?.state === "storage_error") {
              gate = "storage_error";
              error = st.error ?? "";
              return;
            }
          } catch {
            // Fall through to offline gate.
          }
          gate = "offline";
          authFailed = false;
          error = errorMessage(err);
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
      pendingOnboardingTransition = false;
      bootstrapEpoch++;
      bootstrapAbort?.abort();
      featureRefresh.cancel();
      stopSettings();
      clearCachedStepUp();
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
        void fetchAuthStatus()
          .then((st) => {
            authStatus = st;
            if (st.state === "needs_onboarding") {
              gate = "onboarding";
              authFailed = false;
              return;
            }
            if (st.state === "storage_error") {
              gate = "storage_error";
              error = st.error ?? "";
              authFailed = false;
              return;
            }
            gate = "auth";
            authFailed =
              st.state !== "ready" &&
              (wasReady || submittedToken || token.length > 0);
          })
          .catch(() => {
            gate = "auth";
            authFailed = wasReady || submittedToken || token.length > 0;
          });
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
      authStatus = null;
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
    get authStatus() {
      return authStatus;
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
    async signInWithPasskey() {
      checking = true;
      error = "";
      authFailed = false;
      try {
        await loginWithPasskey();
        token = stored("token");
        await check(true);
      } catch (err) {
        error = errorMessage(err);
        checking = false;
      }
    },
    async signInWithRecoveryCode(code: string) {
      checking = true;
      error = "";
      authFailed = false;
      try {
        await loginWithRecoveryCode(code);
        token = stored("token");
        await check(true);
      } catch (err) {
        error = errorMessage(err);
        checking = false;
      }
    },
    async completeOnboarding() {
      pendingOnboardingTransition = gate === "onboarding";
      token = stored("token");
      navigate("/");
      await check(true);
    },
    openPairPage() {
      error = "";
      authFailed = false;
      gate = "pair";
      if (location.pathname !== "/pair") navigate("/pair");
    },
    backToLogin() {
      error = "";
      authFailed = false;
      gate = "auth";
      if (location.pathname === "/pair") navigate("/");
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
