import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import { resetAttentionToasts } from "../../src/domains/notifications/attention-toast.svelte";
import PushSettings from "../../src/components/settings/PushSettings.svelte";
import type { PushSupport } from "../../src/platform/push";
import type { PushStatus } from "../../src/contracts/schemas";

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  localStorage.removeItem("webmux.toastNotifications");
  localStorage.removeItem("webmux.push.subscriptionId");
  resetAttentionToasts();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

const capable: PushSupport = {
  secure: true,
  serviceWorker: true,
  pushManager: true,
  notification: true,
  ios: false,
  standalone: false,
  permission: "default",
};
const available: PushStatus = {
  enabled: true,
  available: true,
  publicKey: "BP4z",
  subscriptions: 2,
};

const cases: {
  name: string;
  support?: Partial<PushSupport>;
  status?: Partial<PushStatus>;
  shows: string[];
  toggle: boolean;
}[] = [
  {
    name: "ready",
    shows: ["本设备未开启", "服务端共有 2 台设备开启"],
    toggle: true,
  },
  {
    name: "quick tunnel",
    status: { available: false, reason: "quick_tunnel", publicKey: undefined },
    shows: [
      "Quick Tunnel 无法持续接收推送",
      "旧订阅点开即失效",
      "Tailscale Serve/Funnel",
      "查看公网访问配置",
    ],
    toggle: false,
  },
  {
    name: "unsupported origin",
    status: { available: false, reason: "unsupported_origin" },
    shows: ["当前地址不支持推送", "HTTPS 入口", "查看公网访问配置"],
    toggle: false,
  },
  {
    name: "insecure context",
    support: { secure: false, serviceWorker: false, pushManager: false },
    shows: [
      "推送需要 HTTPS 安全上下文",
      "Tailscale Serve/Funnel",
      "查看公网访问配置",
    ],
    toggle: false,
  },
  {
    name: "iOS Safari tab",
    support: { ios: true, pushManager: false },
    shows: ["iOS 需要从主屏幕打开", "添加到主屏幕", "重新输入 Token"],
    toggle: false,
  },
  {
    name: "permission denied",
    support: { permission: "denied" },
    shows: ["通知权限已被拒绝", "刷新页面"],
    toggle: false,
  },
  {
    name: "disabled in config",
    status: { enabled: false, available: false, reason: "disabled" },
    shows: ["推送已在配置中停用", "[push] enabled = false"],
    toggle: false,
  },
];

for (const row of cases) {
  test(`push card: ${row.name}`, async () => {
    render(PushSettings, {
      support: { ...capable, ...row.support },
      loadStatus: async () => ({ ...available, ...row.status }),
    });
    for (const text of row.shows)
      await expect
        .element(page.getByText(text, { exact: false }).first())
        .toBeVisible();
    await expect
      .element(page.getByText("推送是尽力送达", { exact: true }))
      .toBeVisible();
    await expect
      .element(
        page.getByText("会话列表中的状态仍是唯一准确来源", { exact: false }),
      )
      .toBeVisible();
    await expect
      .element(page.getByRole("switch", { name: "页面内右上角弹窗提醒" }))
      .toBeChecked();
    expect(
      page.getByRole("switch", { name: "在本设备开启推送" }).elements(),
    ).toHaveLength(row.toggle ? 1 : 0);
  });
}

test("in-app toast notifications are enabled by default and can be toggled on this device", async () => {
  render(PushSettings, {
    support: { ...capable, secure: false, serviceWorker: false },
    loadStatus: async () => ({
      ...available,
      available: false,
      reason: "unsupported_origin",
    }),
  });
  const toastToggle = page.getByRole("switch", {
    name: "页面内右上角弹窗提醒",
  });
  await expect.element(toastToggle).toBeChecked();
  await toastToggle.click();
  await expect.element(toastToggle).not.toBeChecked();
  expect(localStorage.getItem("webmux.toastNotifications")).toBe("0");
  await toastToggle.click();
  await expect.element(toastToggle).toBeChecked();
  expect(localStorage.getItem("webmux.toastNotifications")).toBe("1");
});

test("any browser can send the test to every device once one is subscribed", async () => {
  // A desktop that cannot or has not turned push on still drives the phones.
  const button = page.getByRole("button", { name: /发送测试通知/ });
  render(PushSettings, {
    support: capable,
    loadStatus: async () => ({
      ...available,
      available: false,
      reason: "unsupported_origin",
    }),
  });
  await expect
    .element(page.getByText("向全部 2 台设备发送测试通知"))
    .toBeVisible();
  cleanup();

  render(PushSettings, {
    support: capable,
    loadStatus: async () => ({ ...available, subscriptions: 0 }),
  });
  await expect.element(page.getByText("本设备未开启")).toBeVisible();
  expect(button.elements()).toHaveLength(0);
});

test("a pending permission prompt explains where to allow notifications", async () => {
  // Chrome's quiet permission UI leaves the prompt pending until the user
  // clicks the address-bar bell; the card must not sit silently disabled.
  const prompt = Promise.withResolvers<NotificationPermission>();
  vi.stubGlobal("Notification", {
    permission: "default",
    requestPermission: () => prompt.promise,
  });
  render(PushSettings, {
    support: capable,
    loadStatus: async () => available,
  });
  const toggle = page.getByRole("switch", { name: "在本设备开启推送" });
  await expect.element(toggle).toBeVisible();
  const testAction = page.getByRole("button", { name: /发送测试通知/ });
  const initial = testAction.element().getBoundingClientRect().top;
  toggle.element().focus();
  await toggle.click();
  const hint = page.getByText("等待浏览器通知授权");
  await expect.element(hint).toBeVisible();
  await expect
    .element(page.getByText("通知已屏蔽", { exact: false }))
    .toBeVisible();
  await expect.element(toggle).toBeDisabled();

  expect(testAction.element().getBoundingClientRect().top).toBe(initial);
  prompt.resolve("denied");
  await expect
    .element(page.getByText("未获得通知权限", { exact: false }))
    .toBeVisible();
  await expect.element(hint).not.toBeInTheDocument();
  await expect.element(toggle).not.toBeChecked();
  await expect.element(toggle).toBeEnabled();
  await expect.element(toggle).toHaveFocus();
  expect(testAction.element().getBoundingClientRect().top).toBe(initial);
});

test("keeps push switch above test button, stable across 1->0 subscriptions, and restores keyboard focus on toggle-off", async () => {
  localStorage.setItem("webmux.push.subscriptionId", "sub-1");
  let activeSub = true;
  const deleteGate = Promise.withResolvers<Response>();
  vi.stubGlobal("Notification", {
    permission: "default",
    requestPermission: async () => "granted" as const,
  });
  vi.spyOn(navigator.serviceWorker, "getRegistration").mockImplementation(
    async () =>
      (activeSub
        ? {
            pushManager: {
              getSubscription: async () => ({
                unsubscribe: async () => {
                  activeSub = false;
                  return true;
                },
                options: { applicationServerKey: null },
              }),
            },
          }
        : undefined) as ServiceWorkerRegistration | undefined,
  );
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async () => deleteGate.promise),
  );

  const view = await render(PushSettings, {
    support: capable,
    loadStatus: async () => ({
      ...available,
      subscriptions: activeSub ? 1 : 0,
    }),
  });
  const toggle = page.getByRole("switch", { name: "在本设备开启推送" });
  const testAction = page.getByRole("button", { name: /发送测试通知/ });
  await expect.element(toggle).toBeChecked();
  await expect.element(testAction).toBeVisible();
  expect(view.container.querySelector('[data-slot="scroll-area"]')).toBeNull();

  const toggleRect = toggle.element().getBoundingClientRect();
  const testRect = testAction.element().getBoundingClientRect();
  expect(toggleRect.bottom).toBeLessThanOrEqual(testRect.top + 0.5);

  toggle.element().focus();
  await expect.element(toggle).toHaveFocus();
  await userEvent.keyboard(" ");
  await expect.element(toggle).toBeDisabled();

  deleteGate.resolve(new Response(null, { status: 204 }));
  await expect.element(toggle).not.toBeChecked();
  await expect.element(toggle).toBeEnabled();
  await expect.element(toggle).toHaveFocus();
  expect(testAction.elements()).toHaveLength(0);
  expect(
    Math.abs(toggle.element().getBoundingClientRect().top - toggleRect.top),
  ).toBeLessThanOrEqual(0.5);
});
