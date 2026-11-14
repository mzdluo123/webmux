import {
  test,
  expect,
  expectConnected,
  expectTerminalLine,
  runCommand,
} from "./fixtures";
import {
  CapabilitiesSchema,
  type PluginID,
} from "../../src/contracts/plugin-schemas";

const selections: { name: string; plugins: PluginID[] | undefined }[] = [
  { name: "missing selection preserves legacy defaults", plugins: undefined },
  { name: "explicit empty selection stays core-only", plugins: [] },
  { name: "code-only", plugins: ["code"] },
  { name: "launch-only", plugins: ["launch"] },
  { name: "workspace-only", plugins: ["workspace"] },
  {
    name: "chat with explicit dependencies",
    plugins: ["chat", "files", "launch"],
  },
  { name: "mcp-only", plugins: ["mcp"] },
];

for (const selection of selections) {
  const profileTest = test.extend<{}, { plugins: PluginID[] | undefined }>({
    plugins: [selection.plugins, { scope: "worker", option: true }],
  });
  profileTest.describe(selection.name, () => {
    profileTest(
      "capability bootstrap and raw terminal remain usable with the exact configured selection",
      async ({ page, request, workDir }) => {
        const response = await request.get("/api/capabilities");
        expect(response.ok()).toBeTruthy();
        const capabilities = CapabilitiesSchema.parse(await response.json());
        const enabled = capabilities.plugins
          .filter((plugin) => plugin.enabled)
          .map((plugin) => plugin.id)
          .sort();
        if (selection.plugins === undefined) {
          expect(enabled).toEqual(
            capabilities.plugins.map((plugin) => plugin.id).sort(),
          );
          expect(enabled.length).toBeGreaterThan(0);
        } else {
          expect(enabled).toEqual([...selection.plugins].sort());
        }
        expect(capabilities.capabilities).toContainEqual({
          id: "terminal.pty",
          available: true,
        });

        // Use the same real core create and attach contract regardless of which
        // optional controllers are installed. No launch/chat metadata is supplied.
        const created = await request.post("/api/sessions", {
          data: {
            cwd: workDir,
            cmd: ["/bin/bash", "--noprofile", "--norc", "-i"],
            name: `Selection: ${selection.name}`,
          },
        });
        expect(created.status(), await created.text()).toBe(201);
        const { session } = await created.json();
        expect(session.cwd).toBe(workDir);
        await page.goto(`/s/${session.id}`);
        await expectConnected(page);
        await expectTerminalLine(page, "webmux-e2e$");
        await runCommand(page, "printf '%s\\n' PLUGIN_SELECTION_PTY_OK");
        await expectTerminalLine(page, "PLUGIN_SELECTION_PTY_OK");
      },
    );
  });
}
