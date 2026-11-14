import { afterEach, expect, test, vi } from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import NewSessionDialog from "../../src/components/sessions/NewSessionDialog.svelte";
import RepositorySettingsDialog from "../../src/components/workspace/RepositorySettingsDialog.svelte";
import QuickStartInitInstructionBar from "../../src/components/sessions/QuickStartInitInstructionBar.svelte";
import { PluginIDSchema } from "../../src/contracts/plugin-schemas";
import { setPluginCapabilities } from "../../src/platform/plugin-host.svelte";
import {
  getPluginContributions,
  setPluginContributions,
} from "../../src/plugins/context";
import { settings } from "../../src/domains/settings/settings.svelte";
import { createTerminalDrafts } from "../../src/domains/terminal/terminal-draft.svelte";
import {
  commands,
  installChatFixture,
  respond,
  socketAt,
} from "./chat-fixture";
import { chat, launchConfig, repository } from "./shell-fixture";
import ShellHarness from "./ShellHarness.svelte";

installChatFixture();
const originalCommands = settings.values.quickCommands;
const originalPath = location.pathname;
afterEach(() => {
  settings.values.quickCommands = originalCommands;
  localStorage.clear();
  history.replaceState(null, "", originalPath);
});

function capabilities(unavailable: string[], quickCommands = true) {
  setPluginCapabilities({
    version: 1,
    plugins: PluginIDSchema.options.map((id) => ({
      id,
      enabled: id !== "quick-commands" || quickCommands,
      status: id === "quick-commands" && !quickCommands ? "disabled" : "ready",
    })),
    capabilities: [
      "terminal.pty",
      "workspace.manage",
      "launch.configs",
      "chat.acp",
      "files.upload",
      ...(quickCommands ? ["quick-commands.send"] : []),
    ].map((id) => ({ id, available: !unavailable.includes(id) })),
  });
}

for (const failure of ["runtime", "contribution"] as const) {
  test(`${failure} unavailability hides executable file/chat controls and preserves a usable terminal form`, async () => {
    if (failure === "runtime") capabilities(["files.upload", "chat.acp"]);
    else {
      const contributions = getPluginContributions();
      setPluginContributions({
        ...contributions,
        creation: {
          ...contributions.creation,
          DirectoryPicker: undefined,
          LaunchStartSideFields: undefined,
        },
      });
    }
    localStorage.setItem("webmux.lastLaunch", "custom");
    localStorage.setItem("webmux.customCommand", "bash");
    localStorage.setItem("webmux.hybridStartSide", "chat");
    respond(/^POST \/api\/sessions$/, () =>
      Response.json(
        { error: "directory: no such file or directory" },
        { status: 400 },
      ),
    );
    render(NewSessionDialog, {
      launchConfigs: { status: "ready", configs: [launchConfig()] },
      defaultCwd: "/missing",
      onchange: vi.fn(),
      onworktreecreated: vi.fn(),
      onclose: vi.fn(),
    });
    await expect.element(page.getByText(/文件功能暂不可用/)).toBeVisible();
    await expect
      .element(page.getByText(/聊天功能暂不可用；请选择/))
      .toBeVisible();
    expect(page.getByRole("button", { name: "浏览目录" }).elements()).toEqual(
      [],
    );
    expect(page.getByRole("radio", { name: "从聊天开始" }).elements()).toEqual(
      [],
    );
    await page.getByRole("button", { name: "运行", exact: true }).click();
    await expect
      .element(page.getByRole("option", { name: /^Claude Code/ }))
      .toHaveAttribute("aria-disabled", "true");
    await page.getByRole("option", { name: /自定义命令/ }).click();
    await page
      .getByRole("button", { name: "创建并进入 →", exact: true })
      .click();
    await expect
      .element(page.getByRole("alert"))
      .toHaveTextContent("directory: no such file or directory");
    expect(
      page.getByRole("button", { name: "创建目录并启动" }).elements(),
    ).toEqual([]);
    expect(commands()).toEqual([
      expect.objectContaining({
        path: "/api/sessions",
        body: { cwd: "/missing", cmd: ["bash"], name: "" },
      }),
    ]);
  });
}

test("disabled quick commands keep saved references editable without exposing commands or expanding previews", async () => {
  capabilities([], false);
  const saved = [
    {
      id: "saved",
      name: "私有指令",
      command: "PRIVATE_COMMAND_BODY",
      enter: true,
    },
  ];
  settings.values.quickCommands = saved;
  const template = "说明 {{私有指令}} {{user}}";
  respond(/^PATCH \/api\/repositories\/repo1$/, () =>
    Response.json(repository()),
  );
  render(RepositorySettingsDialog, {
    repository: repository({ initInstruction: template }),
    onclose: vi.fn(),
    onchange: vi.fn(),
  });
  const field = page.getByRole("textbox", { name: "初始化指令" });
  await expect.element(field).toHaveValue(template);
  expect(
    page.getByRole("button", { name: "插入 私有指令" }).elements(),
  ).toEqual([]);
  await field.fill(template + " 保留新输入");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect
    .poll(() => commands().at(-1)?.body)
    .toEqual({ initInstruction: template + " 保留新输入" });
  expect(settings.values.quickCommands).toEqual(saved);
  await cleanup();
  render(QuickStartInitInstructionBar, {
    repository: repository({ initInstruction: template }),
    sending: false,
    useInstruction: true,
    onchange: vi.fn(),
  });
  await page.getByRole("button", { name: "预览", exact: true }).click();
  await expect
    .element(page.getByText("说明 {{私有指令}} 〔你的消息〕"))
    .toBeVisible();
  expect(page.getByText("PRIVATE_COMMAND_BODY").elements()).toEqual([]);
  await expect
    .element(page.getByRole("status"))
    .toHaveTextContent("找不到快捷指令：{{私有指令}}，发送时会被服务端拒绝。");
});

test("selected but unavailable workspace falls back to creating chat in an ordinary directory", async () => {
  capabilities(["workspace.manage"]);
  history.replaceState(null, "", "/new");
  respond(/^GET \/api\/launch-configs$/, () => Response.json([launchConfig()]));
  respond(/^GET \/api\/sessions$/, () => Response.json([]));
  respond(/^POST \/api\/sessions$/, () =>
    Response.json(
      { session: chat({ id: "directaa" }), configWarnings: [] },
      { status: 201 },
    ),
  );
  render(ShellHarness, { drafts: createTerminalDrafts() });
  const socket = await socketAt("/ws/sessions");
  socket.frame({ type: "snapshot", sessions: [], forwards: [] } as never);
  await expect
    .element(page.getByRole("dialog", { name: "新建会话" }))
    .toBeVisible();
  await page
    .getByRole("textbox", { name: "工作目录", exact: true })
    .fill("/plain-directory");
  await page.getByRole("button", { name: "运行", exact: true }).click();
  await page.getByRole("option", { name: /^Claude Code/ }).click();
  await page.getByRole("radio", { name: "从聊天开始" }).click();
  await page.getByRole("button", { name: "创建并进入 →", exact: true }).click();
  await expect.poll(() => location.pathname).toBe("/s/directaa");
  expect(
    commands().find((command) => command.path === "/api/sessions")?.body,
  ).toEqual({
    cwd: "/plain-directory",
    configId: "cfgclaud",
    name: "",
    view: "chat",
  });
});
