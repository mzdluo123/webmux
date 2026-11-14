import {
  test,
  expect,
  expectConnected,
  expectTerminalLine,
  runCommand,
} from "./fixtures";

test.use({ plugins: ["code"] });

test("a failed code chunk leaves the PTY usable and a page reload restores the plugin without restarting the session", async ({
  page,
  request,
  workDir,
}) => {
  const created = await request.post("/api/sessions", {
    data: {
      cwd: workDir,
      cmd: ["/bin/bash", "--noprofile", "--norc", "-i"],
    },
  });
  expect(created.status(), await created.text()).toBe(201);
  const { session } = await created.json();
  const chunk = /\/assets\/terminal-code-[^/]+\.js/;
  await page.route(chunk, (route) => route.abort());
  await page.goto(`/s/${session.id}`);
  await expectConnected(page);
  const reload = page.getByRole("button", { name: "重新加载页面" });
  await expect(reload).toBeVisible();
  await runCommand(page, "printf '%s\\n' CHUNK_FAILURE_CORE_OK");
  await expectTerminalLine(page, "CHUNK_FAILURE_CORE_OK");

  await page.unroute(chunk);
  await reload.click();
  await expectConnected(page);
  await expect(reload).not.toBeVisible();
  await expect(
    page.getByRole("button", { name: "代码", exact: true }),
  ).toBeVisible();
  await expectTerminalLine(page, "CHUNK_FAILURE_CORE_OK");
  await expect(page).toHaveURL(new RegExp(`/s/${session.id}$`));
});
