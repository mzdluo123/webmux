import { m } from "$lib/paraglide/messages.js";
import { api } from "../../platform/api";
import { configurePerf } from "../../platform/perf";
import { homeCache, type HomeSetup } from "../../domains/workspace/home-cache";
import { ConfigSchema } from "../../contracts/schemas";

/** Server settings the shell's pages read: host, Git capability, defaults. */
export interface ServerSetup {
  readonly value: HomeSetup;
  /** A read finished, or a cached value was available. */
  readonly settled: boolean;
  readonly busy: boolean;
  /** Why the last read failed; "" after a successful one. */
  readonly error: string;
  load(signal?: AbortSignal): Promise<void>;
}

const EMPTY: HomeSetup = {
  cwd: "~",
  hostname: "",
  pendingUpdate: undefined,
  gitReady: false,
  registryReady: false,
  capabilityError: "",
  // Until the server says otherwise, assume listening ports are detected.
  portDetection: true,
  voice: false,
};

export function createServerSetup(): ServerSetup {
  const cached = homeCache.setup();
  let value = $state.raw<HomeSetup>(cached ?? EMPTY);
  let settled = $state(!!cached);
  let busy = $state(false);
  let loadFailed = $state(false);
  return {
    get value() {
      return value;
    },
    get settled() {
      return settled;
    },
    get busy() {
      return busy;
    },
    get error() {
      return loadFailed ? m.s1_server_setup_load_failed() : "";
    },
    async load(signal) {
      if (busy) return;
      busy = true;
      try {
        const config = await api(
          "/config",
          ConfigSchema,
          "GET",
          undefined,
          signal,
        );
        value = {
          cwd: config.defaultCwd,
          hostname: config.hostname,
          pendingUpdate: config.pendingUpdate,
          gitReady: config.workspace.gitAvailable,
          registryReady: config.workspace.registryAvailable,
          capabilityError: [
            config.workspace.gitError,
            config.workspace.registryError,
          ]
            .filter(Boolean)
            .join(" · "),
          portDetection: config.portDetection,
          voice: config.voice,
        };
        homeCache.rememberSetup(value);
        configurePerf(config.perf);
        loadFailed = false;
      } catch {
        loadFailed = true;
      } finally {
        busy = false;
        settled = true;
      }
    },
  };
}
