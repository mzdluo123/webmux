import { mkdtemp, readFile, rm } from "node:fs/promises";
import { join } from "node:path";
import {
  test as base,
  expect,
  type APIRequestContext,
  type Locator,
  type Page,
} from "@playwright/test";
import {
  chooseSessionDirectory,
  ensureSessionSetup,
  sessionLocation,
} from "./session-project";
import { startE2EServer, type E2EServer } from "./server.mjs";
import {
  CapabilitiesSchema,
  type FeatureID,
} from "../../src/contracts/feature-schemas";

type Session = { id: string; status: string; exitCode: number | null };

async function sessions(request: APIRequestContext): Promise<Session[]> {
  const response = await request.get("/api/sessions");
  expect(response.ok(), "List isolated backend sessions").toBeTruthy();
  return response.json();
}

async function cleanSessions(request: APIRequestContext) {
  for (const session of await sessions(request)) {
    // A hybrid session refuses to close (409) while it switches sides and
    // leaves the list once closed; cleanHybridRecords removes its record.
    // One archived just before the test ended may leave between the list
    // and its close (404).
    let closed = 0;
    await expect
      .poll(
        async () => {
          closed = (
            await request.delete(`/api/sessions/${session.id}`)
          ).status();
          return closed === 204 || closed === 404;
        },
        { message: `Close ${session.id}` },
      )
      .toBe(true);
    if (closed === 404) continue;
    // A session with a chat view is archived once closed; its record is
    // deleted with the other archived sessions.
    let gone = false;
    await expect
      .poll(async () => {
        const response = await request.get(`/api/sessions/${session.id}`);
        gone = response.status() === 404;
        if (gone) return "exited";
        const info = await response.json();
        gone = info.state !== "active";
        return info.status;
      })
      .toBe("exited");
    if (gone) continue;
    const removed = await request.delete(
      `/api/sessions/${session.id}?remove=1`,
    );
    expect(removed.status(), `Remove ${session.id}`).toBe(204);
  }
}

// Closed and interrupted hybrid sessions stay recoverable on the home page
// until their records are removed.
async function cleanHybridRecords(request: APIRequestContext) {
  const response = await request.get("/api/sessions?state=archived");
  expect(response.ok(), "List archived sessions").toBeTruthy();
  for (const record of await response.json()) {
    expect(
      (await request.delete(`/api/sessions/${record.id}?forget=1`)).status(),
      `Remove hybrid record ${record.id}`,
    ).toBe(204);
  }
}

async function cleanRepositories(request: APIRequestContext) {
  const response = await request.get("/api/repositories");
  expect(response.ok()).toBeTruthy();
  for (const repository of await response.json()) {
    expect(
      (await request.delete(`/api/repositories/${repository.id}`)).status(),
    ).toBe(204);
  }
}

async function cleanLaunchConfigs(request: APIRequestContext) {
  const response = await request.get("/api/launch-configs");
  expect(response.ok()).toBeTruthy();
  for (const config of await response.json()) {
    expect(
      (await request.delete(`/api/launch-configs/${config.id}`)).status(),
    ).toBe(204);
  }
}

async function cleanP2PSettings(request: APIRequestContext) {
  const response = await request.delete("/api/p2p/settings");
  expect([200, 501]).toContain(response.status());
}

async function cleanFeatures(
  request: APIRequestContext,
  enabled: ReadonlySet<FeatureID>,
) {
  if (enabled.has("chat")) await cleanHybridRecords(request);
  if (enabled.has("workspace")) await cleanRepositories(request);
  if (enabled.has("launch")) await cleanLaunchConfigs(request);
  if (enabled.has("p2p")) await cleanP2PSettings(request);
}

async function resetSettings(
  request: APIRequestContext,
  enabled: ReadonlySet<FeatureID>,
) {
  const response = await request.get("/api/settings");
  expect(response.ok()).toBeTruthy();
  const prefs = await response.json();
  const changes: Record<string, null> = {
    theme: null,
    sessionPins: null,
    forceSoftKeys: null,
  };
  if (enabled.has("code")) {
    changes.preview = null;
    changes.codeReader = null;
  }
  if (enabled.has("quick-commands")) changes.quickCommands = null;
  if (enabled.has("workspace")) changes.homeWorktrees = null;
  if (enabled.has("forward")) changes.ignoreHighPorts = null;
  expect(
    (
      await request.patch("/api/settings", {
        data: { baseRevision: prefs.revision, changes },
      })
    ).ok(),
  ).toBeTruthy();
}

export const test = base.extend<
  { workDir: string; _isolation: void },
  { server: E2EServer; features: FeatureID[] | undefined }
>({
  // Missing preserves legacy defaults; [] is explicitly core-only. Do not
  // resolve dependencies here: the real backend validates the exact selection.
  features: [undefined, { scope: "worker", option: true }],
  server: [
    async ({ features }, use) => {
      const server = await startE2EServer({ features });
      try {
        await use(server);
      } finally {
        await server.close();
      }
    },
    { scope: "worker", timeout: 240_000 },
  ],
  baseURL: async ({ server }, use) => {
    await use(server.baseURL);
  },
  workDir: async ({ server }, use) => {
    const directory = await mkdtemp(join(server.workDir, "case-"));
    try {
      await use(directory);
    } finally {
      await rm(directory, { recursive: true, force: true });
    }
  },
  _isolation: [
    async ({ request, page, server }, use, testInfo) => {
      const capabilityResponse = await request.get("/api/capabilities");
      expect(capabilityResponse.ok()).toBeTruthy();
      const capabilities = CapabilitiesSchema.parse(
        await capabilityResponse.json(),
      );
      const enabled = new Set(
        capabilities.features
          .filter((feature) => feature.enabled)
          .map((feature) => feature.id),
      );
      await cleanSessions(request);
      await cleanFeatures(request, enabled);
      await resetSettings(request, enabled);
      const browserLogs: string[] = [];
      page.on("console", (message) =>
        browserLogs.push(`[${message.type()}] ${message.text()}`),
      );
      page.on("pageerror", (error) =>
        browserLogs.push(error.stack ?? error.message),
      );
      page.on("requestfailed", (request) =>
        browserLogs.push(
          `${request.method()} ${request.url()}: ${request.failure()?.errorText}`,
        ),
      );
      try {
        await use();
      } finally {
        try {
          if (testInfo.status !== testInfo.expectedStatus) {
            await testInfo.attach("server.log", {
              body: server.logs(),
              contentType: "text/plain",
            });
            await testInfo.attach("browser.log", {
              body: browserLogs.join("\n"),
              contentType: "text/plain",
            });
            const snapshot = await sessions(request);
            await testInfo.attach("sessions.json", {
              body: JSON.stringify(snapshot, null, 2),
              contentType: "application/json",
            });
            for (const session of snapshot) {
              const buffer = await request.get(
                `/api/sessions/${session.id}/buffer`,
              );
              await testInfo.attach(`terminal-${session.id}.txt`, {
                body: await buffer.body(),
                contentType: "text/plain",
              });
            }
            if (enabled.has("chat"))
              await testInfo.attach("archived-sessions.json", {
                body: await (
                  await request.get("/api/sessions?state=archived")
                ).body(),
                contentType: "application/json",
              });
            const agentEvents = await readFile(
              join(server.env.FAKEAGENT_STORE!, "events.log"),
              "utf8",
            ).catch(() => "");
            if (agentEvents)
              await testInfo.attach("fakeagent-events.log", {
                body: agentEvents,
                contentType: "text/plain",
              });
          }
        } finally {
          await cleanSessions(request);
          await cleanFeatures(request, enabled);
          await resetSettings(request, enabled);
        }
      }
    },
    { auto: true },
  ],
});

export const coreTest = test.extend<{}, { features: FeatureID[] | undefined }>({
  features: [[], { scope: "worker", option: true }],
});

export { expect };

// New-session dialog pickers. Option names start with the item title and
// continue with its command or path.
async function pick(
  page: Page,
  field: "位置" | "工作位置" | "运行",
  name: string,
) {
  await page.getByRole("button", { name: field, exact: true }).click();
  const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  await page
    .getByRole("option", { name: new RegExp(`^${escaped}`) })
    .first()
    .click();
}

/**
 * Leaves a session page for the session list: the header's back button on
 * wide screens; on phones the sidebar drawer's "全部仓库".
 */
export async function backToList(page: Page) {
  const back = page.getByRole("button", { name: "返回会话列表", exact: true });
  const open = page.getByRole("button", { name: "打开侧栏" }).first();
  await expect(back.or(open).first()).toBeVisible();
  if (await back.isVisible()) return back.click();
  await open.click();
  await page
    .getByRole("tree", { name: "仓库与会话" })
    .locator('[data-tree-key="all"]')
    .click();
}

/**
 * Makes global sidebar entries reachable, waiting for mobile drawer entry
 * before consumers interact with its contents.
 */
export async function revealSidebar(page: Page): Promise<boolean> {
  const open = page
    .getByRole("button", { name: /^(打开|展开)侧栏$/ })
    .filter({ visible: true })
    .first();
  const navigation = page.getByRole("navigation", {
    name: "导航",
    exact: true,
  });
  // Capability bootstrap and lazy pages may not have mounted the mobile
  // trigger yet. An immediate isVisible() would mistake that for desktop.
  await expect
    .poll(
      async () => (await open.isVisible()) || (await navigation.isVisible()),
    )
    .toBe(true);
  let opened = false;
  // A closing drawer is still visible during its exit animation. Prefer
  // the trigger's open state rather than mistaking that for an open drawer.
  if (
    (await open.isVisible()) &&
    (await open.getAttribute("aria-expanded")) !== "true"
  ) {
    await open.click();
    opened = true;
  }
  await expect(navigation).toBeVisible();
  const drawer = page.getByRole("dialog", { name: "导航", exact: true });
  if (await drawer.isVisible())
    await expect
      .poll(() =>
        drawer.evaluate((node) =>
          node
            .getAnimations()
            .some(
              (animation) =>
                animation.pending || animation.playState === "running",
            ),
        ),
      )
      .toBe(false);
  return opened;
}

export async function chooseLaunch(page: Page, name: string) {
  await ensureSessionSetup(page);
  await pick(page, "运行", name);
}

export async function chooseLocation(page: Page, name: string) {
  await expect(sessionLocation(page)).toBeVisible();
  await pick(
    page,
    (await page
      .getByRole("button", { name: "工作位置", exact: true })
      .isVisible())
      ? "工作位置"
      : "位置",
    name,
  );
}

export async function useCustomCommand(page: Page, command: string) {
  await ensureSessionSetup(page);
  const launch = page.getByRole("button", { name: "运行", exact: true });
  const custom = page.getByLabel("自定义启动命令", { exact: true });
  await expect(launch.or(custom).first()).toBeVisible();
  if (await launch.isVisible()) await chooseLaunch(page, "自定义命令…");
  await page.getByLabel("自定义启动命令", { exact: true }).fill(command);
}

export async function useDirectory(page: Page, cwd: string) {
  const input = page.getByLabel("工作目录", { exact: true });
  const legacy = page.getByRole("button", { name: "位置", exact: true });
  await expect(
    input
      .or(legacy)
      .or(page.getByRole("combobox", { name: "搜索项目", exact: true }))
      .or(page.getByRole("button", { name: "更换项目", exact: true }))
      .first(),
  ).toBeVisible();
  if (await input.isVisible()) await input.fill(cwd);
  else if (await legacy.isVisible()) {
    await chooseLocation(page, "输入或浏览目录…");
    await input.fill(cwd);
  } else await chooseSessionDirectory(page, cwd);
}

export async function nameSession(page: Page, name: string) {
  await ensureSessionSetup(page);
  await page.getByRole("button", { name: "高级：会话名称" }).click();
  await page.getByLabel("会话名称", { exact: false }).fill(name);
}

export async function createShell(
  page: Page,
  name = "E2E shell",
  cwd?: string,
): Promise<string> {
  await page.goto("/");
  await page
    .getByRole("main")
    .getByRole("button", { name: "＋ 新建会话", exact: true })
    .first()
    .click();
  await useCustomCommand(page, "/bin/bash --noprofile --norc -i");
  if (cwd) await useDirectory(page, cwd);
  await nameSession(page, name);
  await page.getByRole("button", { name: "创建并进入 →", exact: true }).click();
  await expect(page).toHaveURL(/\/s\/[a-z0-9]+$/);
  await expectConnected(page);
  await expect(page.locator(".restore-overlay")).toBeHidden();
  await expect(page.locator(".xterm-helper-textarea")).toBeAttached();
  // A live PTY connection can precede Bash's terminal-mode initialization.
  // Wait for the fixture shell before sending command bytes into it.
  await expectTerminalLine(page, "webmux-e2e$");
  return new URL(page.url()).pathname.split("/").at(-1)!;
}

// Routes terminal sockets so a test can drop the live one, as a page returning
// from the background finds its mobile socket dead, and hold the reattachment
// until `release`. Install it before the page opens the terminal.
export async function holdTerminalReattach(page: Page) {
  const gate = Promise.withResolvers<void>();
  let held = false;
  let live: (() => Promise<void>) | undefined;
  await page.routeWebSocket("**/ws/s/*", async (socket) => {
    if (held) await gate.promise;
    socket.connectToServer();
    live = () => socket.close();
  });
  return {
    async drop() {
      expect(live, "A routed terminal socket").toBeDefined();
      held = true;
      await live!();
    },
    release: () => gate.resolve(),
  };
}

// Session links navigate in the same tab; the page that showed the list
// becomes the terminal page.
export async function openSessionLink(page: Page, name: string): Promise<Page> {
  await page.getByRole("link", { name, exact: true }).click();
  await expect(page).toHaveURL(/\/s\/[a-z2-7]{8}$/);
  return page;
}

// A healthy connection shows no text; the status dot names it instead.
export async function expectConnected(page: Page) {
  const header = page.locator(".terminal-header");
  await expect(header.getByRole("img", { name: /· 已连接$/ })).toBeVisible();
  await expect(header.getByRole("status")).toHaveCount(0);
}

// Below 640px the header moves upload and fullscreen into the terminal menu
// (scroll-to-latest is the floating button); this opens the menu there and
// returns the entry.
export async function headerAction(page: Page, name: string) {
  if ((page.viewportSize()?.width ?? 1280) >= 640)
    return page
      .locator(".terminal-header")
      .getByRole("button", { name, exact: true });
  await page.getByRole("button", { name: "终端菜单", exact: true }).click();
  return page.getByRole("menuitem", { name, exact: true });
}

export async function runCommand(page: Page, command: string) {
  const input = page.locator(".xterm-helper-textarea");
  await expect(page.locator(".restore-overlay")).toBeHidden();
  // Target the textarea after replay rather than relying on focus held across
  // the wait. Both locator actions focus it when dispatching real key events.
  await input.pressSequentially(command);
  await input.press("Enter");
}

export async function expectTerminalLine(page: Page, line: string) {
  // The E2E Chromium projects use xterm's built-in DOM renderer. Match a full
  // rendered line, not the echoed printf command or a mocked terminal buffer.
  await expect
    .poll(async () =>
      page
        .locator(".xterm-rows > div")
        .evaluateAll((rows) =>
          rows.map((row) =>
            (row.textContent ?? "").replaceAll("\u00a0", " ").trimEnd(),
          ),
        ),
    )
    .toContain(line);
}

export const quote = (value: string) =>
  "'" + value.replaceAll("'", "'\\''") + "'";

/** Turns the shell into a raw byte recorder so tests can assert exact PTY input. */
export async function recordInput(
  page: Page,
  directory: string,
  bracketed = true,
) {
  const path = join(directory, "received-input");
  await runCommand(
    page,
    `stty raw -echo; : > ${quote(path)}; printf '\\033[?2004${bracketed ? "h" : "l"}RECORDER_READY\\r\\n'; cat >> ${quote(path)}`,
  );
  await expectTerminalLine(page, "RECORDER_READY");
  await expect.poll(() => readFile(path, "utf8")).toBe("");
  return path;
}

// Split actions live in the session's side-panel picker on every viewport.
export async function openSplitPanel(scope: Locator | Page) {
  const picker = await openPanelPicker(scope);
  await picker.getByRole("button", { name: "分屏", exact: true }).click();
}

async function openPanelPicker(scope: Locator | Page) {
  const page = "viewportSize" in scope ? scope : scope.page();
  await scope.getByRole("button", { name: "打开面板", exact: true }).click();
  return page.getByRole("group", { name: "面板选择", exact: true });
}

export async function toggleCode(scope: Locator | Page) {
  if (await scope.locator(".code-pane").isVisible()) {
    await openPanelPicker(scope);
    await scope
      .getByRole("button", { name: "关闭侧栏面板", exact: true })
      .click();
  } else
    await (
      await openPanelPicker(scope)
    )
      .getByRole("button", { name: "代码", exact: true })
      .click();
}
