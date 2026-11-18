import {
  test,
  expect,
  expectConnected,
  expectTerminalLine,
  runCommand,
  openSessionForm,
  useCustomCommand,
  useDirectory,
} from "./fixtures";
import {
  CapabilitiesSchema,
  type FeatureID,
} from "../../src/contracts/feature-schemas";

const selections: { name: string; features: FeatureID[] | undefined }[] = [
  { name: "missing selection preserves legacy defaults", features: undefined },
  { name: "explicit empty selection stays core-only", features: [] },
  { name: "code-only", features: ["code"] },
  { name: "launch-only", features: ["launch"] },
  { name: "workspace-only", features: ["workspace"] },
  { name: "workspace with launch", features: ["workspace", "launch"] },
  { name: "workspace with files", features: ["workspace", "files"] },
  {
    name: "chat with explicit dependencies",
    features: ["chat", "files", "launch"],
  },
  { name: "mcp-only", features: ["mcp"] },
];

for (const selection of selections) {
  const profileTest = test.extend<{}, { features: FeatureID[] | undefined }>({
    features: [selection.features, { scope: "worker", option: true }],
  });
  profileTest.describe(selection.name, () => {
    profileTest(
      "the visible new-session flow creates a terminal in the selected folder",
      async ({ page, request, workDir }) => {
        const paths: string[] = [];
        page.on("request", (request) =>
          paths.push(new URL(request.url()).pathname),
        );
        await page.goto("/");
        await openSessionForm(page);
        await useCustomCommand(page, "/bin/bash --noprofile --norc -i");
        await useDirectory(page, workDir);
        await page
          .getByRole("button", { name: "创建并进入 →", exact: true })
          .click();
        await expect(page).toHaveURL(/\/s\/[a-z0-9]+$/);
        const sessions = await (await request.get("/api/sessions")).json();
        expect(sessions).toHaveLength(1);
        expect(sessions[0]).toMatchObject({ cwd: workDir, workspace: null });
        await expectConnected(page);
        await runCommand(page, "printf '%s\\n' PROJECT_PROFILE_OK");
        await expectTerminalLine(page, "PROJECT_PROFILE_OK");
        if (selection.features && !selection.features.includes("workspace"))
          expect(
            paths.some((path) =>
              /^\/api\/(projects|repositories)(?:\/|$)/.test(path),
            ),
          ).toBe(false);
        if (selection.features && !selection.features.includes("launch"))
          expect(
            paths.some((path) => path.startsWith("/api/launch-configs")),
          ).toBe(false);
      },
    );
    profileTest(
      "capability bootstrap and raw terminal remain usable with the exact configured selection",
      async ({ page, request, workDir }) => {
        const response = await request.get("/api/capabilities");
        expect(response.ok()).toBeTruthy();
        const capabilities = CapabilitiesSchema.parse(await response.json());
        const enabled = capabilities.features
          .filter((feature) => feature.enabled)
          .map((feature) => feature.id)
          .sort();
        if (selection.features === undefined) {
          expect(enabled).toEqual(
            capabilities.features.map((feature) => feature.id).sort(),
          );
          expect(enabled.length).toBeGreaterThan(0);
        } else {
          expect(enabled).toEqual([...selection.features].sort());
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
        await runCommand(page, "printf '%s\\n' FEATURE_SELECTION_PTY_OK");
        await expectTerminalLine(page, "FEATURE_SELECTION_PTY_OK");
      },
    );
  });
}
