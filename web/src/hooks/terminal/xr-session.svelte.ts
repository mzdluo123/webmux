import { getContext, onDestroy, onMount, setContext } from "svelte";
import { SvelteMap } from "svelte/reactivity";
import { xrPresentation } from "../shared/xr-presentation.svelte";
import type {
  XrAvailableSession,
  XrHeaderControl,
  XrSessionSource,
  XrWorkspaceRuntime,
} from "../../domains/terminal/xr-workspace";
import { errorMessage } from "../../platform/error";

const contextKey = Symbol("xr-workspace");

interface Workspace {
  register(source: XrSessionSource): () => void;
  header(id: string): XrHeaderControl;
  returnTo2D(id: string): Promise<void>;
}

/** One page owns XR; its mounted panes retain their existing sessions. */
export function createXrSessionWorkspace(options: {
  available(this: void): readonly XrAvailableSession[];
  open(this: void, id: string): void | Promise<void>;
  error(this: void, message: string): void;
}): Workspace {
  const sources = new SvelteMap<string, XrSessionSource>();
  let sourceList: XrSessionSource[] = [];
  let supported = $state(false);
  let active = $state(false);
  let busy = $state(false);
  let owner: XRSession | undefined;
  let runtime: XrWorkspaceRuntime | undefined;
  let disposed = false;
  let serial = 0;

  function unavailable() {
    if (!window.isSecureContext)
      return "沉浸式 VR 需要 HTTPS 安全上下文。请使用 Tailscale Serve 或受信任的 HTTPS 入口。";
    if (!navigator.xr)
      return "当前浏览器不提供 WebXR；仍可使用完整的 2D 工作台。";
    return "当前设备或浏览器不支持沉浸式 VR。";
  }

  function restore() {
    runtime?.dispose();
    runtime = undefined;
    owner?.removeEventListener("end", ended);
    owner = undefined;
    active = false;
    xrPresentation.active = false;
    busy = false;
    for (const source of sources.values()) source.setImmersive(false);
  }

  function ended() {
    serial++;
    restore();
  }

  function exit() {
    const session = owner;
    if (!session) return;
    busy = true;
    return session.end().catch((error: unknown) => {
      options.error(`退出 VR 失败：${errorMessage(error)}`);
      restore();
    });
  }

  function enter(initialId: string) {
    if (disposed || busy || owner) return;
    if (!window.isSecureContext || !navigator.xr || !supported) {
      options.error(unavailable());
      return;
    }
    const attempt = ++serial;
    busy = true;
    // Request before importing/awaiting: transient user activation belongs to
    // this click, not to the eventual completion of the lazy scene chunk.
    const request = navigator.xr.requestSession("immersive-vr");
    void request
      .then(async (session) => {
        if (disposed || attempt !== serial) {
          await session.end();
          return;
        }
        owner = session;
        owner.addEventListener("end", ended);
        active = true;
        xrPresentation.active = true;
        for (const source of sources.values()) source.setImmersive(true);
        // XR-only platform boundary: static loading would send the Three.js
        // runtime to ordinary browsers that cannot enter immersive sessions.
        const module = await import("./xr-workspace");
        if (disposed || owner !== session || attempt !== serial) return;
        const next = await module.startXrWorkspace(session, {
          initialId,
          sources: () => sourceList,
          available: options.available,
          open: options.open,
          error: options.error,
          exit,
        });
        if (disposed || owner !== session || attempt !== serial) next.dispose();
        else runtime = next;
        if (attempt === serial) busy = false;
      })
      .catch((error: unknown) => {
        if (disposed || attempt !== serial) return;
        const session = owner;
        restore();
        if (session) void session.end().catch(() => undefined);
        options.error(`无法进入 VR：${errorMessage(error)}。2D 会话保持运行。`);
      });
  }

  onMount(() => {
    if (window.isSecureContext && navigator.xr) {
      void navigator.xr.isSessionSupported("immersive-vr").then(
        (value) => {
          if (!disposed) supported = value;
        },
        () => {
          if (!disposed) supported = false;
        },
      );
    }
  });
  onDestroy(() => {
    disposed = true;
    serial++;
    const session = owner;
    restore();
    if (session) void session.end().catch(() => undefined);
    sources.clear();
  });

  const workspace: Workspace = {
    register(source) {
      sources.set(source.id, source);
      sourceList = [...sources.values()];
      if (active) source.setImmersive(true);
      return () => {
        if (sources.get(source.id) !== source) return;
        source.setImmersive(false);
        sources.delete(source.id);
        sourceList = [...sources.values()];
      };
    },
    async returnTo2D(id) {
      try {
        await options.open(id);
        await exit();
      } catch (error) {
        options.error(`无法返回会话：${errorMessage(error)}`);
      }
    },
    header(id) {
      return {
        get supported() {
          return supported;
        },
        get active() {
          return active;
        },
        get busy() {
          return busy;
        },
        enter: () => enter(id),
        exit,
      };
    },
  };
  setContext(contextKey, workspace);
  return workspace;
}

export function findXrSessionWorkspace(): Workspace | undefined {
  return getContext<Workspace | undefined>(contextKey);
}
