import { readFile } from "node:fs/promises";
import { join } from "node:path";
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
async function source(request: APIRequestContext, cwd: string, long = false) {
  const cfg = await config(request, "Handoff source");
  const id = await create(request, cwd, cfg);
  for (let turn = 0; turn < (long ? 3 : 1); turn++) {
    const response = await request.post(`/api/sessions/${id}/input`, {
      data: {
        clientRequestId: `original-source-turn-${turn}`,
        text:
          "original source constraint\n" +
          (long
            ? Array.from(
                { length: 160 },
                (_, index) =>
                  `record ${turn}.${index}: preserve exact code(x <= 7) in the history;\n`,
              ).join("")
            : ""),
        mode: "refuse",
      },
    });
    expect(response.ok(), await response.text()).toBeTruthy();
    await expect
      .poll(async () => {
        const response = await request.get(
          `/api/sessions/${id}/native-history?maxBytes=64000`,
        );
        if (!response.ok()) return false;
        const result = await response.json();
        return (
          result.records.filter((r: { role: string }) => r.role === "assistant")
            .length ===
          turn + 1
        );
      })
      .toBe(true);
  }
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
  const target = await (
    await request.get(`/api/sessions/${operation.targetSessionId}`)
  ).json();
  const stored = JSON.parse(
    await readFile(
      join(
        server.env.FAKEAGENT_STORE,
        "sessions",
        `${target.agent.sessionId}.json`,
      ),
      "utf8",
    ),
  );
  const prompt = stored.messages.find(
    (m: { role: string }) => m.role === "user",
  );
  const task = "\n当前用户任务：\n" + p.userText;
  expect(p.pages).toHaveLength(1);
  expect(p.text).not.toContain("original source constraint");
  expect(prompt.blocks).toEqual([
    { type: "text", text: p.text.slice(0, -task.length) },
    { type: "image", mimeType: "image/png", data: p.pages[0].data },
    { type: "text", text: task },
  ]);
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

test("image page budget preserves typing and blocks invalid or stale previews", async ({
  page,
  request,
  server,
}) => {
  const src = await source(request, server.workDir);
  await page.goto(`/handoff/${src.id}`);
  await page.getByRole("combobox", { name: "目标 Agent", exact: true }).click();
  await page
    .getByRole("option", { name: "Handoff source", exact: true })
    .click();
  await page
    .getByRole("textbox", { name: "下一步任务", exact: true })
    .fill("Continue the original source task");
  await page.getByText("更多设置", { exact: true }).click();
  const pages = page.getByRole("textbox", {
    name: "图片页数预算（1–8）",
    exact: true,
  });
  const generate = page.getByRole("button", { name: "预览交接", exact: true });
  const send = page.getByRole("button", {
    name: "创建聊天并发送",
    exact: true,
  });
  const budgetError = page.getByRole("alert").filter({
    hasText: "请输入 1 到 8 之间的整数。",
  });
  const previewPath = `/api/sessions/${src.id}/handoff-previews`;
  const submittedBudgets: number[] = [];
  page.on("request", (req) => {
    if (req.method() === "POST" && req.url().endsWith(previewPath)) {
      const body = req.postDataJSON() as { maxPages: number };
      submittedBudgets.push(body.maxPages);
    }
  });
  async function typeBudget(value: string) {
    await pages.press("End");
    const length = (await pages.inputValue()).length;
    for (let index = 0; index < length; index++) {
      await pages.press("Backspace");
    }
    await expect(pages).toHaveValue("");
    await expect(send).toHaveCount(0);
    if (value) await pages.pressSequentially(value);
    await expect(pages).toHaveValue(value);
  }
  async function generatePreview() {
    const response = page.waitForResponse(
      (res) =>
        res.request().method() === "POST" && res.url().endsWith(previewPath),
    );
    await generate.click();
    expect((await response).status()).toBe(201);
    await expect(send).toBeEnabled();
    await page.getByRole("button", { name: "返回修改" }).click();
    await page.getByText("更多设置", { exact: true }).click();
  }

  await expect(pages).toHaveAttribute("inputmode", "numeric");
  await expect(pages).toHaveValue("8");
  await typeBudget("1");
  await pages.blur();
  await expect(pages).toHaveValue("1");
  await generatePreview();
  expect(submittedBudgets).toEqual([1]);

  for (const invalid of ["", "0", "9", "2.8", "1e0", "abc", "-1"]) {
    await typeBudget(invalid);
    await expect(send).toHaveCount(0);
    await generate.click();
    await expect(budgetError).toBeVisible();
    await expect(pages).toHaveAttribute("aria-invalid", "true");
    await expect(pages).toHaveAccessibleDescription(
      /请输入 1 到 8 之间的整数。/,
    );
    await expect(pages).toHaveValue(invalid);
    expect(submittedBudgets).toEqual([1]);
  }

  await typeBudget("8");
  await pages.blur();
  await expect(pages).toHaveValue("8");
  await expect(pages).toHaveAttribute("aria-invalid", "false");
  await expect(budgetError).toHaveCount(0);
  await expect(send).toHaveCount(0);
  await generatePreview();
  expect(submittedBudgets).toEqual([1, 8]);

  await typeBudget("2");
  await expect(send).toHaveCount(0);
  await generatePreview();
  expect(submittedBudgets).toEqual([1, 8, 2]);

  await typeBudget("");
  await pages.blur();
  await expect(budgetError).toBeVisible();
  await expect(send).toHaveCount(0);
});

test("review shows the recipient, image and task before creating a chat", async ({
  page,
  request,
  server,
}) => {
  const src = await source(request, server.workDir, true);
  await config(request, "UI handoff target");
  await page.goto(`/handoff/${src.id}`);
  await page.getByRole("combobox", { name: "目标 Agent" }).click();
  await page.getByRole("option", { name: "UI handoff target" }).click();
  await page
    .getByRole("textbox", { name: "下一步任务" })
    .fill("Inspect the original constraint");
  await page.getByRole("button", { name: "预览交接" }).click();
  await expect(
    page.getByRole("heading", { name: "UI handoff target" }),
  ).toBeFocused();
  const zoom = page.getByRole("button", { name: "放大第 1 页" });
  await zoom.click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(zoom).toBeFocused();
  await page.getByText("查看详情", { exact: true }).click();
  await page.getByRole("button", { name: "补查冻结原文", exact: true }).click();
  await expect(page.locator("pre")).toContainText("original source constraint");
  await page.getByRole("button", { name: "创建聊天并发送" }).click();
  await expect(page.getByText("已发送到「UI handoff target」")).toBeVisible();
  await page.getByRole("button", { name: "打开目标聊天" }).click();
  await expect(page).toHaveURL(/\/s\//);
});

test("switch agent searches available chats and delivers to the chosen session", async ({
  page,
  request,
  server,
}) => {
  const src = await source(request, server.workDir);
  const target = await create(request, server.workDir, src.cfg);
  await page.goto(`/handoff/${src.id}?target=switch`);
  await expect(page.getByRole("heading", { name: "更换 Agent" })).toBeVisible();
  await page.getByRole("radio", { name: "已有聊天" }).click();
  await page.locator("#handoff-target").click();
  await page
    .getByPlaceholder("搜索聊天标题、Agent 或目录")
    .fill(server.workDir);
  const option = page.getByRole("option").first();
  await option.click();
  await page.getByRole("textbox", { name: "下一步任务" }).fill("Continue here");
  await page.getByRole("button", { name: "预览交接" }).click();
  await page.getByRole("button", { name: "发送到此聊天" }).click();
  await expect(page.getByText(/已发送到「/)).toBeVisible();
  await page.getByRole("button", { name: "打开目标聊天" }).click();
  await expect(page).toHaveURL(new RegExp("/s/" + target + "$"));
});

test("image-unsupported targets block delivery and text previews are rejected", async ({
  request,
  server,
}) => {
  const src = await source(request, server.workDir, true);
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
  expect(response.status(), await response.text()).toBe(400);
});
