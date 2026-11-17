import type { APIRequestContext } from "@playwright/test";
import type {
  HandoffPreview,
  HandoffOperation,
} from "../../src/contracts/handoff-schemas";
import { test, expect } from "./fixtures";
async function config(request: APIRequestContext, name: string, images = true) {
  const response = await request.post("/api/launch-configs", {
    data: {
      name,
      cmd: ["omp"],
      chat: true,
      env: {
        FAKEAGENT_VERSION: "18.13.1",
        FAKEAGENT_PROMPT_IMAGE: images ? "1" : "0",
      },
    },
  });
  expect(response.status(), await response.text()).toBe(201);
  return (await response.json()).id as string;
}
async function create(
  request: APIRequestContext,
  cwd: string,
  configId: string,
) {
  const response = await request.post("/api/sessions", {
    data: { cwd, configId, view: "chat" },
  });
  expect(response.status(), await response.text()).toBe(201);
  return (await response.json()).session.id as string;
}
async function source(request: APIRequestContext, cwd: string) {
  const cfg = await config(request, "Handoff source");
  const id = await create(request, cwd, cfg);
  expect(
    (
      await request.post(`/api/sessions/${id}/input`, {
        data: {
          clientRequestId: "original-source-turn",
          text: "original source constraint",
          mode: "refuse",
        },
      })
    ).ok(),
  ).toBeTruthy();
  await expect
    .poll(async () => {
      const response = await request.get(`/api/sessions/${id}/native-history`);
      if (!response.ok()) return false;
      const result = await response.json();
      return result.records.some(
        (r: { role: string }) => r.role === "assistant",
      );
    })
    .toBe(true);
  return { id, cfg };
}
async function preview(
  request: APIRequestContext,
  sourceId: string,
  configId: string,
  cwd: string,
  targetSessionId = "",
) {
  const response = await request.post(
    `/api/sessions/${sourceId}/handoff-previews`,
    {
      data: {
        configId,
        targetSessionId,
        userText: "当前任务\n\tkeep  spaces",
        agentConfig: {},
        location: {
          cwd,
          repositoryId: "",
          worktreeId: "",
          newWorktreeBase: "",
          newWorktreeBranch: "",
        },
      },
    },
  );
  expect(response.status(), await response.text()).toBe(201);
  return (await response.json()) as HandoffPreview;
}
async function complete(request: APIRequestContext, id: string) {
  let operation: HandoffOperation | undefined;
  await expect
    .poll(async () => {
      const response = await request.get(`/api/handoff-operations/${id}`);
      expect(response.ok()).toBeTruthy();
      operation = (await response.json()) as HandoffOperation;
      return operation.state;
    })
    .toBe("succeeded");
  return operation!;
}

test("concurrent confirmation creates one target and sends one native prompt", async ({
  request,
  server,
}) => {
  const src = await source(request, server.workDir);
  const targetConfig = await config(request, "Handoff target");
  const before = await (await request.get("/api/sessions")).json();
  const p = await preview(request, src.id, targetConfig, server.workDir);
  expect((await (await request.get("/api/sessions")).json()).length).toBe(
    before.length,
  );
  const responses = await Promise.all(
    Array.from({ length: 8 }, () =>
      request.post(`/api/handoff-previews/${p.id}/send`, {
        data: { operationId: p.operationId },
      }),
    ),
  );
  for (const response of responses)
    expect(response.status(), await response.text()).toBe(202);
  const operation = await complete(request, p.operationId);
  expect(operation.targetSessionId).toBeTruthy();
  expect((await (await request.get("/api/sessions")).json()).length).toBe(
    before.length + 1,
  );
  await expect
    .poll(async () => {
      const response = await request.get(
        `/api/sessions/${operation.targetSessionId}/native-history`,
      );
      if (!response.ok()) return 0;
      const page = await response.json();
      return page.records.filter(
        (r: { role: string; text: string }) =>
          r.role === "user" && r.text.endsWith("当前任务\n\tkeep  spaces"),
      ).length;
    })
    .toBe(1);
  const original = await request.get(
    `/api/sessions/${src.id}/native-history?snapshotId=${p.snapshotId}`,
  );
  expect(original.ok(), await original.text()).toBeTruthy();
  expect(
    (await original.json()).records.some((r: { text: string }) =>
      r.text.includes("original source constraint"),
    ),
  ).toBe(true);
});

test("preview and operation handles expire across server restart", async ({
  request,
  server,
}) => {
  const src = await source(request, server.workDir);
  const p = await preview(request, src.id, src.cfg, server.workDir);
  await server.restart();
  const response = await request.post(`/api/handoff-previews/${p.id}/send`, {
    data: { operationId: p.operationId },
  });
  expect(response.status(), await response.text()).toBe(410);
  expect(
    (
      await request.post("/api/conversations/native", {
        data: { action: "import" },
      })
    ).status(),
  ).toBe(410);
});

test("handoff form previews frozen images and clears process handles on refresh", async ({
  page,
  request,
  server,
}) => {
  const src = await source(request, server.workDir);
  await config(request, "UI handoff target");
  await page.goto(`/handoff/${src.id}`);
  const agent = page.getByRole("combobox", { name: "目标 Agent", exact: true });
  await agent.click();
  await page
    .getByRole("option", { name: "UI handoff target", exact: true })
    .click();
  await page
    .getByRole("textbox", { name: "用户指令", exact: true })
    .fill("new task\n  preserve indentation");
  const pages = page.getByRole("spinbutton", { name: "图片页数预算（1–8）" });
  for (const [input, normalized] of [
    ["", "8"],
    ["0", "1"],
    ["9", "8"],
    ["2.8", "2"],
    ["", "8"],
  ]) {
    await pages.fill(input);
    await expect(pages).toHaveValue(normalized);
  }
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await expect(
    page.getByRole("textbox", { name: "当前任务（原样文本）", exact: true }),
  ).toHaveValue(/new task/);
  const image = page.getByRole("img", { name: "历史图片第 1 页", exact: true });
  await expect(image).toBeVisible();
  expect(
    await image.evaluate((node: HTMLImageElement) => node.naturalWidth),
  ).toBeGreaterThan(0);
  await page.getByRole("button", { name: "放大第 1 页", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("button", { name: "补查冻结原文", exact: true }).click();
  await expect(page.locator("pre")).toContainText("original source constraint");
  await page.getByRole("radio", { name: "文本交接", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "发送交接", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await expect(image).toHaveCount(0);
  await page.getByText("查看选入历史文本", { exact: true }).click();
  await expect(
    page.getByRole("textbox", { name: "查看选入历史文本", exact: true }),
  ).toHaveValue(/original source constraint/);
  await page.getByRole("radio", { name: "分页图片", exact: true }).click();
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await expect(image).toBeVisible();
  await page.route("**/api/handoff-previews/*", (route) =>
    route.request().method() === "GET" ? route.abort() : route.continue(),
  );
  const sent = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().includes("/handoff-previews/") &&
      response.url().endsWith("/send"),
  );
  await page.getByRole("button", { name: "发送交接", exact: true }).click();
  const response = await sent;
  expect(response.status(), await response.text()).toBe(202);
  const operation = (await response.json()) as HandoffOperation;
  expect((await complete(request, operation.id)).state).toBe("succeeded");
  await page.reload();
  await expect(
    page.getByRole("textbox", { name: "用户指令", exact: true }),
  ).toHaveValue("");
  await expect(
    page.getByRole("textbox", { name: "当前任务（原样文本）", exact: true }),
  ).toHaveCount(0);
});

test("image-unsupported targets require an explicitly reviewed text preview", async ({
  request,
  server,
}) => {
  const src = await source(request, server.workDir);
  const cfg = await config(request, "Text-only target", false);
  const target = await create(request, server.workDir, cfg);
  const image = await preview(request, src.id, cfg, server.workDir, target);
  expect(image.capability).toBe("unsupported");
  expect(image.blockedReason).not.toBe("");
  const refused = await request.post(`/api/handoff-previews/${image.id}/send`, {
    data: { operationId: image.operationId },
  });
  expect(refused.status()).toBe(400);
  const response = await request.post(
    `/api/sessions/${src.id}/handoff-previews`,
    {
      data: { ...image.request, format: "text" },
    },
  );
  expect(response.status(), await response.text()).toBe(201);
  const text = (await response.json()) as HandoffPreview;
  expect(text.pages).toEqual([]);
  expect(text.text).toContain("original source constraint");
  expect(text.userText).toBe(image.userText);
  const sent = await request.post(`/api/handoff-previews/${text.id}/send`, {
    data: { operationId: text.operationId },
  });
  expect(sent.status(), await sent.text()).toBe(202);
  expect((await complete(request, text.operationId)).targetSessionId).toBe(
    target,
  );
});
