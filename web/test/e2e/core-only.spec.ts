import {
  coreTest as test,
  expect,
  expectConnected,
  expectTerminalLine,
  nameSession,
  quote,
  runCommand,
} from "./fixtures";
import { CapabilitiesSchema } from "../../src/contracts/plugin-schemas";

// These are production lazy-entry asset names, not source-content assertions.
// Observe actual browser requests, including preload fetches and WebSockets.
const pluginChunk =
  /\/(?:terminal-(?:chat|files|code|forward|immersive|quick-commands|voice|workspace|git)|chat|code|workspace|launch|forward|notifications|p2p|cloudflare|tailscale|LaunchPicker|LocationPicker|WorktreeCreateFields|DirectoryPicker|ConnectorInstallHint|P2PSettings|QuickCommandsSettings|VoiceSettings|LaunchConfigsSettings|TunnelSettings|TailscaleSettings|PushSettings|GitWorkbench|CodePopout|Handoff|QuickStart)-[^/]+\.js$/;

test("core-only ignores stale plugin preferences and creates, types into, and replays a real PTY", async ({
  page,
  request,
  workDir,
}) => {
  const capabilities = CapabilitiesSchema.parse(
    await (await request.get("/api/capabilities")).json(),
  );
  expect(
    capabilities.plugins.every(
      (plugin) => !plugin.enabled && plugin.status === "disabled",
    ),
  ).toBe(true);
  expect(capabilities.capabilities).toContainEqual({
    id: "terminal.pty",
    available: true,
  });

  const disabledRequests: string[] = [];
  const terminalSockets: string[] = [];
  const inspect = (address: string) => {
    const url = new URL(address);
    const path = url.pathname;
    const coreAPI =
      /^\/api\/(?:capabilities|health|config|settings)$/.test(path) ||
      /^\/api\/sessions(?:\/[a-z0-9]+(?:\/(?:buffer|input))?)?$/.test(path);
    if (
      (path.startsWith("/api/") &&
        (!coreAPI || url.searchParams.get("state") === "archived")) ||
      (path.startsWith("/ws/") &&
        !/^\/ws\/(?:settings|sessions|s\/[a-z0-9]+)$/.test(path)) ||
      pluginChunk.test(path)
    )
      disabledRequests.push(path);
  };
  page.on("request", (resource) => inspect(resource.url()));
  page.on("websocket", (socket) => {
    inspect(socket.url());
    if (new URL(socket.url()).pathname.startsWith("/ws/s/"))
      terminalSockets.push(socket.url());
  });
  await page.addInitScript(() => {
    localStorage.setItem("webmux.shellMode", "chat");
    localStorage.setItem("webmux.lastLaunch", "config:deleted-plugin-launch");
    localStorage.setItem("webmux.hybridStartSide", "chat");
    localStorage.setItem("webmux.codePanelShown", "1");
  });
  await page.goto("/");
  await page
    .getByRole("main")
    .getByRole("button", { name: "新建会话", exact: true })
    .first()
    .click();
  await expect(
    page.getByRole("button", { name: "位置", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "运行", exact: true }),
  ).toHaveCount(0);
  const command = page.getByLabel("自定义启动命令", { exact: true });
  // A saved launch ID must never be reinterpreted as a shell command.
  await expect(command).toHaveValue("");
  await page.getByLabel("工作目录", { exact: true }).fill(workDir);
  await command.fill("/bin/bash --noprofile --norc -i");
  await nameSession(page, "Core isolated PTY");
  await page.getByRole("button", { name: "创建并进入 →", exact: true }).click();
  await expect(page).toHaveURL(/\/s\/[a-z0-9]+$/);
  const sessionURL = page.url();
  const id = new URL(sessionURL).pathname.split("/").at(-1)!;
  await expectConnected(page);
  await expectTerminalLine(page, "webmux-e2e$");
  const session = await (await request.get(`/api/sessions/${id}`)).json();
  expect(session).toMatchObject({ cwd: workDir });
  await expect(
    page.getByRole("button", { name: "代码", exact: true }),
  ).toHaveCount(0);

  await runCommand(
    page,
    `test "$PWD" = ${quote(workDir)} && printf '%s\\n' CORE_CWD_OK; printf '%s\\n' CORE_REPLAY_READY > ${quote(`${workDir}/core-output`)}; cat ${quote(`${workDir}/core-output`)}`,
  );
  await expectTerminalLine(page, "CORE_CWD_OK");
  await expectTerminalLine(page, "CORE_REPLAY_READY");
  // Reload destroys the browser terminal; the same server PTY buffer must replay.
  await page.reload();
  await expectConnected(page);
  await expectTerminalLine(page, "CORE_REPLAY_READY");
  await runCommand(page, "printf '%s\\n' CORE_AFTER_REATTACH");
  await expectTerminalLine(page, "CORE_AFTER_REATTACH");
  expect(terminalSockets.length).toBeGreaterThanOrEqual(2);

  // Settings is a separate core consumer and must not initialize disabled
  // plugin providers merely because old browser preferences still exist.
  await page.goto("/settings");
  await expect(
    page.getByRole("heading", { name: "设置", exact: true }),
  ).toBeVisible();
  await page.goto(sessionURL);
  await expectConnected(page);
  await expectTerminalLine(page, "CORE_AFTER_REATTACH");
  expect(
    disabledRequests,
    "No disabled REST, WebSocket, or implementation chunk requests",
  ).toEqual([]);
});
