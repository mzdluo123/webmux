import { expect, test } from "./fixtures";

test("renaming a launch config preserves plugin options when their editors fail to load", async ({
  page,
  request,
}) => {
  const attention = {
    mode: "urgent",
    programStatus: true,
    agentProtocol: false,
    notify: true,
    bell: false,
    minTurnSeconds: 7,
    doneDelaySeconds: 3,
  };
  const created = await request.post("/api/launch-configs", {
    data: {
      name: "Optional editor preservation",
      cmd: ["omp"],
      env: {},
      chat: true,
      attention,
    },
  });
  expect(created.status(), await created.text()).toBe(201);
  const config = await created.json();
  const chatEditor = Promise.withResolvers<void>();
  const attentionEditor = Promise.withResolvers<void>();
  await page.route(
    /\/assets\/Launch(?:Chat|Attention)Fields-[^/]+\.js$/,
    async (route) => {
      const isChat = route.request().url().includes("LaunchChatFields-");
      await route.abort("failed");
      (isChat ? chatEditor : attentionEditor).resolve();
    },
  );
  await page.goto("/settings#launch-configs");
  await Promise.all([chatEditor.promise, attentionEditor.promise]);
  await page.getByRole("button", { name: "关闭 chat 加载提示" }).click();
  await page
    .getByRole("button", { name: "关闭 notifications 加载提示" })
    .click();
  await page
    .locator("#launch-configs")
    .getByRole("button", {
      name: "编辑 Optional editor preservation",
      exact: true,
    })
    .click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("名称", { exact: true }).fill("Renamed safely");
  await dialog.getByRole("button", { name: "保存", exact: true }).click();
  await expect(dialog).toBeHidden();
  const stored = await request.get(`/api/launch-configs/${config.id}`);
  expect(stored.status()).toBe(200);
  expect(await stored.json()).toMatchObject({
    name: "Renamed safely",
    chat: true,
    attention,
  });
});
