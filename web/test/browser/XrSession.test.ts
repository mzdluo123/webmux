import {
  afterEach,
  beforeEach,
  expect,
  test,
  vi,
  type Mock,
} from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./XrSessionHarness.svelte";
import type {
  XrHeaderControl,
  XrSessionSource,
  XrWorkspaceOptions,
} from "../../src/domains/terminal/xr-workspace";

const runtime = vi.hoisted(() => ({ start: vi.fn(), dispose: vi.fn() }));
vi.mock("../../src/hooks/terminal/xr-workspace", () => ({
  startXrWorkspace: runtime.start,
}));

let request: Mock<(mode: XRSessionMode) => Promise<XRSession>>;
let supported: boolean;
let session: XRSession;
let header: XrHeaderControl;
let errors: string[];
let immersive: boolean;
const previousXr = Object.getOwnPropertyDescriptor(navigator, "xr");

function source(): XrSessionSource {
  return {
    id: "existing-session",
    title: "Shell",
    status: "connected",
    primaryKind: "terminal",
    terminal: undefined,
    graphicsUnsupported: false,
    modifiers: { ctrl: false, alt: false, shift: false },
    shiftLocked: false,
    toggleModifier() {},
    resetModifiers() {},
    lockModifier() {},
    panels: () => [],
    actions: () => [],
    input() {},
    focus() {},
    canInput: () => true,
    blur() {},
    latest() {},
    scroll() {},
    resize: () => false,
    setImmersive: (value) => {
      immersive = value;
    },
  };
}

beforeEach(() => {
  supported = true;
  errors = [];
  immersive = false;
  runtime.start.mockReset().mockResolvedValue({ dispose: runtime.dispose });
  runtime.dispose.mockClear();
  const target = new EventTarget();
  session = Object.assign(target, {
    end: vi.fn(async () => {
      target.dispatchEvent(new Event("end"));
    }),
  }) as unknown as XRSession;
  request = vi.fn().mockResolvedValue(session);
  Object.defineProperty(navigator, "xr", {
    configurable: true,
    value: {
      isSessionSupported: async () => supported,
      requestSession: request,
    },
  });
});

afterEach(async () => {
  await cleanup();
  if (previousXr) Object.defineProperty(navigator, "xr", previousXr);
  else Reflect.deleteProperty(navigator, "xr");
});

async function mount() {
  const view = await render(Harness, {
    source: source(),
    errors,
    expose: (value) => {
      header = value;
    },
  });
  await expect
    .element(page.getByRole("status", { name: "XR state" }))
    .toHaveTextContent("supported 2D idle");
  return view;
}

test("explicit 2D return activates its owning pane before ending VR", async () => {
  const opened = Promise.withResolvers<void>();
  await render(Harness, {
    source: source(),
    errors,
    expose: (value) => {
      header = value;
    },
    reveal: () => opened.promise,
  });
  await expect
    .element(page.getByRole("status", { name: "XR state" }))
    .toHaveTextContent("supported 2D idle");
  header.enter();
  await expect
    .element(page.getByRole("status", { name: "XR state" }))
    .toHaveTextContent("supported immersive idle");
  await page.getByRole("button", { name: "返回 2D 原会话" }).click();
  expect(session.end).not.toHaveBeenCalled();
  await expect
    .element(page.getByRole("region", { name: "Active session" }))
    .toHaveTextContent("other-session");
  opened.resolve();
  await expect
    .element(page.getByRole("status", { name: "XR state" }))
    .toHaveTextContent("supported 2D idle");
  await expect
    .element(page.getByRole("region", { name: "Active session" }))
    .toHaveTextContent("existing-session");
  expect(session.end).toHaveBeenCalledOnce();
});

test("keeps the existing source through enter, system end, and re-entry", async () => {
  await mount();
  await page.getByRole("button", { name: "进入 VR" }).click();
  await expect
    .element(page.getByRole("status"))
    .toHaveTextContent("supported immersive idle");
  expect(immersive).toBe(true);
  session.dispatchEvent(new Event("end"));
  await expect
    .element(page.getByRole("status"))
    .toHaveTextContent("supported 2D idle");
  expect(immersive).toBe(false);
  expect(runtime.dispose).toHaveBeenCalledOnce();
  await page.getByRole("button", { name: "进入 VR" }).click();
  await expect
    .element(page.getByRole("status"))
    .toHaveTextContent("supported immersive idle");
  expect(request).toHaveBeenCalledTimes(2);
});

test("does not create a second session while a gesture request is pending", async () => {
  const pending = Promise.withResolvers<XRSession>();
  request.mockReturnValue(pending.promise);
  const view = await mount();
  header.enter();
  header.enter();
  expect(request).toHaveBeenCalledOnce();
  await view.unmount();
  pending.resolve(session);
  await expect.poll(() => vi.mocked(session.end).mock.calls.length).toBe(1);
  expect(runtime.start).not.toHaveBeenCalled();
  expect(immersive).toBe(false);
});

test("disposes a scene that finishes initialization after the XR session ended", async () => {
  const pending = Promise.withResolvers<{ dispose(): void }>();
  runtime.start.mockReturnValue(pending.promise);
  await mount();
  header.enter();
  await expect.poll(() => runtime.start.mock.calls.length).toBe(1);
  session.dispatchEvent(new Event("end"));
  pending.resolve({ dispose: runtime.dispose });
  await expect.poll(() => runtime.dispose.mock.calls.length).toBe(1);
  expect(immersive).toBe(false);
  await expect
    .element(page.getByRole("status"))
    .toHaveTextContent("supported 2D idle");
});

test("failed scene initialization restores 2D and ends only its owned XR session", async () => {
  runtime.start.mockRejectedValueOnce(new Error("WebGL unavailable"));
  await mount();
  header.enter();
  await expect.poll(() => errors.at(-1)).toContain("WebGL unavailable");
  expect(immersive).toBe(false);
  expect(session.end).toHaveBeenCalledOnce();
  await expect
    .element(page.getByRole("status"))
    .toHaveTextContent("supported 2D idle");
});

test("unsupported browsers retain 2D without requesting permission", async () => {
  supported = false;
  await render(Harness, {
    source: source(),
    errors,
    expose: (value) => {
      header = value;
    },
  });
  await expect
    .element(page.getByRole("status"))
    .toHaveTextContent("unsupported 2D idle");
  header.enter();
  expect(request).not.toHaveBeenCalled();
  expect(errors.at(-1)).toContain("不支持沉浸式 VR");
});

test("observation acquisition and readiness never activate a 2D pane", async () => {
  let ready = false;
  const observe = vi.fn(async () => {});
  const unobserve = vi.fn();
  const original = source();
  original.blur = vi.fn();
  await render(Harness, {
    source: original,
    errors,
    observe,
    unobserve,
    ready: () => ready,
    expose: (value) => {
      header = value;
    },
  });
  await expect
    .element(page.getByRole("status"))
    .toHaveTextContent("supported 2D idle");
  header.enter();
  await expect.poll(() => runtime.start.mock.calls.length).toBe(1);
  const options = runtime.start.mock.calls[0]![1] as XrWorkspaceOptions;
  expect(original.blur).toHaveBeenCalled();
  expect(options.ready()).toBe(false);
  ready = true;
  expect(options.ready()).toBe(true);
  await options.observe("vr-only");
  options.unobserve("vr-only");
  expect(observe).toHaveBeenCalledWith("vr-only");
  expect(unobserve).toHaveBeenCalledWith("vr-only");
  await expect
    .element(page.getByRole("region", { name: "Active session" }))
    .toHaveTextContent("other-session");
  expect(session.end).not.toHaveBeenCalled();
});

test("a failed explicit 2D reveal preserves VR and its current pane", async () => {
  await render(Harness, {
    source: source(),
    errors,
    reveal: async () => {
      throw new Error("2D pane capacity");
    },
    expose: (value) => {
      header = value;
    },
  });
  await expect
    .element(page.getByRole("status"))
    .toHaveTextContent("supported 2D idle");
  header.enter();
  await expect.poll(() => runtime.start.mock.calls.length).toBe(1);
  await page.getByRole("button", { name: "返回 2D 原会话" }).click();
  await expect.poll(() => errors.at(-1)).toContain("2D pane capacity");
  expect(session.end).not.toHaveBeenCalled();
  await expect
    .element(page.getByRole("status"))
    .toHaveTextContent("supported immersive idle");
  await expect
    .element(page.getByRole("region", { name: "Active session" }))
    .toHaveTextContent("other-session");
});
