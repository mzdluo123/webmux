import { test as base, expect } from "./fixtures";
import { startE2EServer } from "./server.mjs";
import type { FeatureID } from "../../src/contracts/feature-schemas";

const test = base.extend<{}, { features: FeatureID[] | undefined }>({
  features: [["messaging"], { scope: "worker", option: true }],
});

test("local channel creation, sending and reload preserve history", async ({
  page,
  request,
  server,
}) => {
  await page.goto("/settings#messaging");
  const panel = page.locator("#messaging");
  const name = `channel-${Date.now()}`;
  await panel
    .getByRole("textbox", { name: "频道名称", exact: true })
    .fill(name);
  await panel
    .getByRole("textbox", { name: "频道主题（可选）", exact: true })
    .fill("Local coordination");
  await panel.getByRole("button", { name: "创建频道", exact: true }).click();
  await expect(panel.getByRole("combobox")).toHaveValue(name);
  await panel
    .getByRole("textbox", { name: "消息", exact: true })
    .fill("A persistent local message");
  await panel.getByRole("button", { name: "发送消息", exact: true }).click();
  await expect(panel.getByText("消息已发送。", { exact: true })).toBeVisible();
  await expect(
    panel.getByRole("textbox", { name: "消息", exact: true }),
  ).toHaveValue("");
  await server.restart();
  await page.reload();
  await panel.getByRole("combobox").selectOption(name);
  await expect(
    panel.getByText("A persistent local message", { exact: true }),
  ).toBeVisible();
  const messages = await request.get(
    `/api/messaging/channels/${name}/messages`,
  );
  expect(await messages.json()).toEqual([
    expect.objectContaining({
      text: "A persistent local message",
      agentId: "owner",
      agentName: "WebMux",
    }),
  ]);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});

test("local messaging rejects cross-origin writes", async ({ request }) => {
  const response = await request.post("/api/messaging/channels", {
    headers: { Origin: "https://untrusted.example" },
    data: { name: "untrusted", topic: "" },
  });
  expect(response.status()).toBe(403);
});

test("the standalone backend requires the existing owner credential", async ({
  request,
}) => {
  const isolated = await startE2EServer({
    features: ["messaging"],
    token: "messaging-test-owner",
  });
  try {
    const path = `${isolated.baseURL}/api/messaging/channels`;
    expect((await request.get(path)).status()).toBe(401);
    expect(
      (
        await request.get(path, { headers: { Authorization: "Bearer wrong" } })
      ).status(),
    ).toBe(401);
    expect(
      (
        await request.post(path, {
          headers: { Authorization: "Bearer messaging-test-owner" },
          data: { name: "general", topic: "" },
        })
      ).status(),
    ).toBe(201);
    const messages = await request.get(path, {
      headers: { Authorization: "Bearer messaging-test-owner" },
    });
    expect(await messages.json()).toEqual([
      expect.objectContaining({ name: "general" }),
    ]);
  } finally {
    await isolated.close();
  }
});
