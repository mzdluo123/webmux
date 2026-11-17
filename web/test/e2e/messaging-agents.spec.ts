import { test as base, expect } from "./fixtures";
import type { FeatureID } from "../../src/contracts/feature-schemas";

const test = base.extend<{}, { features: FeatureID[] | undefined }>({
  features: [
    ["messaging", "chat", "launch", "files"],
    { scope: "worker", option: true },
  ],
});

test("invited agent wakes and replies through its real message plugin", async ({
  page,
  request,
  workDir,
}) => {
  const config = await request.post("/api/launch-configs", {
    data: {
      name: "Channel worker",
      cmd: ["omp"],
      env: { FAKEAGENT_MESSAGES_MCP: "1", FAKEAGENT_TURN_DELAY: "100ms" },
      chat: true,
    },
  });
  expect(config.status(), await config.text()).toBe(201);
  const created = await request.post("/api/sessions", {
    data: {
      cwd: workDir,
      configId: (await config.json()).id,
      view: "chat",
      name: "Channel worker",
    },
  });
  expect(created.status(), await created.text()).toBe(201);
  const id = (await created.json()).session.id;
  await expect
    .poll(async () => {
      const agents = await (await request.get("/api/messaging/agents")).json();
      return agents.some(
        (agent: { sessionId: string }) => agent.sessionId === id,
      );
    })
    .toBe(true);
  const channel = `agents-${Date.now()}`;
  expect(
    (
      await request.post("/api/messaging/channels", {
        data: { name: channel, topic: "" },
      })
    ).status(),
  ).toBe(201);
  await page.goto("/messages");
  await page
    .getByRole("button", { name: `打开频道 ${channel}`, exact: true })
    .click();
  await page.getByRole("button", { name: "频道 Agent", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog
    .getByRole("button", { name: "邀请 Channel worker", exact: true })
    .click();
  await expect(
    dialog.getByRole("button", { name: "移除 omp", exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  // Start a normal turn first: channel notifications must queue behind it.
  const busy = await request.post(`/api/sessions/${id}/input`, {
    data: { text: "An existing task", clientRequestId: crypto.randomUUID() },
  });
  expect(busy.status(), await busy.text()).toBe(200);
  let last = "";
  for (let i = 0; i < 3; i++) {
    const sent = await request.post(
      `/api/messaging/channels/${channel}/messages`,
      {
        data: {
          text: `Review request ${i}`,
          clientRequestId: crypto.randomUUID(),
        },
      },
    );
    expect(sent.status(), await sent.text()).toBe(201);
    last = (await sent.json()).id;
  }
  await expect
    .poll(async () => {
      const members = await (
        await request.get(`/api/messaging/channels/${channel}/members`)
      ).json();
      return members[0]?.cursor;
    })
    .toBe(last);
  const messages = await (
    await request.get(`/api/messaging/channels/${channel}/messages`)
  ).json();
  const replies = messages.filter(
    (message: { agentId: string }) => message.agentId === `session:${id}`,
  );
  expect(replies).toHaveLength(1);
  expect(replies[0].text).toBe("Handled: Review request 0");
  await expect(
    page
      .getByRole("region", { name: `频道 ${channel} 的对话`, exact: true })
      .getByText("Handled: Review request 0", { exact: true }),
  ).toBeVisible();
  // A new message after the acknowledged batch wakes the now-idle agent.
  await request.post(`/api/messaging/channels/${channel}/messages`, {
    data: { text: "One more task", clientRequestId: crypto.randomUUID() },
  });
  await expect(
    page
      .getByRole("region", { name: `频道 ${channel} 的对话`, exact: true })
      .getByText("Handled: One more task", { exact: true }),
  ).toBeVisible();
});
