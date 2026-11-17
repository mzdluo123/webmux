import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Messaging from "../../src/pages/Messaging.svelte";
import {
  enableCreationFeatures,
  resetCreationFeatures,
} from "./feature-fixture";

const message = (id: string, text: string) => ({
  id,
  text,
  agentId: "owner",
  agentName: "WebMux",
  createdAt: "2026-11-17T08:00:00Z",
});
const channels = [
  { id: "1", name: "general", topic: "Team coordination" },
  { id: "2", name: "builds", topic: "" },
];
const box = () => page.getByRole("textbox", { name: "消息", exact: true });

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  resetCreationFeatures();
  vi.unstubAllGlobals();
  localStorage.clear();
  await page.viewport(1280, 900);
});

test("reading is the default and channel creation opens only on request", async () => {
  await enableCreationFeatures(["messaging"]);
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input) =>
      Response.json(
        String(input).endsWith("/channels")
          ? channels
          : [message("1", "Current discussion")],
      ),
    ),
  );
  render(Messaging);
  await expect
    .element(page.getByText("Current discussion", { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByRole("textbox", { name: "频道名称", exact: true }))
    .not.toBeInTheDocument();
  expect(
    getComputedStyle(box().element().closest('[data-slot="input-group"]')!)
      .opacity,
  ).toBe("1");
  await page.getByRole("button", { name: "创建频道", exact: true }).click();
  await expect
    .element(page.getByRole("textbox", { name: "频道名称", exact: true }))
    .toBeVisible();
});

test("channel navigation retains drafts and failed sends reuse the same request", async () => {
  await enableCreationFeatures(["messaging"]);
  let reject = true;
  const ids: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const url = String(input);
      if (init?.method === "POST") {
        const body = JSON.parse(String(init.body));
        ids.push(body.clientRequestId);
        if (reject)
          return Response.json({ error: "消息存储不可用" }, { status: 503 });
        return Response.json(message("101", body.text), { status: 201 });
      }
      if (url.endsWith("/channels")) return Response.json(channels);
      return Response.json(
        url.includes("/general/") ? [message("99", "Previous discussion")] : [],
      );
    }),
  );
  render(Messaging);
  await expect
    .element(page.getByText("Previous discussion", { exact: true }))
    .toBeVisible();
  await box().fill("hello team");
  await page
    .getByRole("button", { name: "打开频道 builds", exact: true })
    .click();
  await expect
    .element(page.getByText("开始讨论", { exact: true }))
    .toBeVisible();
  await box().fill("build draft");
  await page
    .getByRole("button", { name: "打开频道 general", exact: true })
    .click();
  await expect.element(box()).toHaveValue("hello team");
  await page.getByRole("button", { name: "发送消息", exact: true }).click();
  await expect.element(page.getByText(/未能确认发送结果/)).toBeVisible();
  await expect.element(box()).toHaveValue("hello team");
  reject = false;
  await page.getByRole("button", { name: "重试发送", exact: true }).click();
  await expect
    .element(page.getByText("消息已发送。", { exact: true }))
    .toBeVisible();
  await expect.element(box()).toHaveValue("");
  expect(ids).toHaveLength(2);
  expect(new Set(ids).size).toBe(1);
});

test("creates the first channel through a dialog and enters the conversation", async () => {
  await enableCreationFeatures(["messaging"]);
  let created = false;
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      if (init?.method === "POST") {
        created = true;
        return Response.json(channels[0], { status: 201 });
      }
      return Response.json(
        String(input).endsWith("/channels")
          ? created
            ? [channels[0]]
            : []
          : [],
      );
    }),
  );
  render(Messaging);
  await page
    .getByRole("button", { name: "创建第一个频道", exact: true })
    .click();
  await page
    .getByRole("textbox", { name: "频道名称", exact: true })
    .fill("general");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "创建频道", exact: true })
    .click();
  await expect.element(box()).toBeVisible();
  await expect.element(page.getByRole("dialog")).not.toBeInTheDocument();
});

test("search filters channels and phone navigation returns from a conversation", async () => {
  await page.viewport(390, 844);
  await enableCreationFeatures(["messaging"]);
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input) =>
      Response.json(String(input).endsWith("/channels") ? channels : []),
    ),
  );
  render(Messaging);
  await page
    .getByRole("textbox", { name: "查找频道", exact: true })
    .fill("build");
  await expect
    .element(
      page.getByRole("button", { name: "打开频道 general", exact: true }),
    )
    .not.toBeInTheDocument();
  await page
    .getByRole("button", { name: "打开频道 builds", exact: true })
    .click();
  await expect.element(box()).toBeVisible();
  await page.getByRole("button", { name: "返回频道列表", exact: true }).click();
  await expect
    .element(page.getByRole("textbox", { name: "查找频道", exact: true }))
    .toBeVisible();
});

test("loads earlier messages with an exclusive stable cursor", async () => {
  await enableCreationFeatures(["messaging"]);
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input) => {
      const url = String(input);
      if (url.endsWith("/channels")) return Response.json([channels[0]]);
      if (url.includes("before=50"))
        return Response.json([message("49", "Earlier history")]);
      return Response.json(
        Array.from({ length: 50 }, (_, i) =>
          message(String(i + 50), `message ${i + 50}`),
        ),
      );
    }),
  );
  render(Messaging);
  await page.getByRole("button", { name: "加载更早消息", exact: true }).click();
  await expect
    .element(page.getByText("Earlier history", { exact: true }))
    .toBeInTheDocument();
});

test("Enter sends, Shift+Enter and input composition preserve editing", async () => {
  await enableCreationFeatures(["messaging"]);
  let posts = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      if (init?.method === "POST") {
        posts++;
        return Response.json(message("5", "hello"), { status: 201 });
      }
      return Response.json(
        String(input).endsWith("/channels") ? [channels[0]] : [],
      );
    }),
  );
  render(Messaging);
  await box().fill("hello");
  const textarea = box().element();
  textarea.dispatchEvent(
    new KeyboardEvent("keydown", {
      key: "Enter",
      isComposing: true,
      bubbles: true,
      cancelable: true,
    }),
  );
  textarea.dispatchEvent(
    new KeyboardEvent("keydown", {
      key: "Enter",
      shiftKey: true,
      bubbles: true,
      cancelable: true,
    }),
  );
  expect(posts).toBe(0);
  textarea.dispatchEvent(
    new KeyboardEvent("keydown", {
      key: "Enter",
      bubbles: true,
      cancelable: true,
    }),
  );
  await expect
    .element(page.getByText("消息已发送。", { exact: true }))
    .toBeVisible();
  expect(posts).toBe(1);
});

test("channel agents are invited and removed in place", async () => {
  await enableCreationFeatures(["messaging"]);
  const members: {
    id: string;
    name: string;
    profile: string;
    channel: string;
    cursor: string;
  }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const url = String(input);
      if (url.endsWith("/agents"))
        return Response.json([
          { sessionId: "worker", name: "Review changes", profile: "codex" },
        ]);
      if (url.includes("/members/worker") && init?.method === "DELETE") {
        members.length = 0;
        return Response.json({ ok: true });
      }
      if (url.endsWith("/members")) {
        if (init?.method === "POST") {
          const member = {
            id: "session:worker",
            name: "Codex",
            profile: "codex",
            channel: "general",
            cursor: "0",
          };
          members.push(member);
          return Response.json(member, { status: 201 });
        }
        return Response.json([...members]);
      }
      if (url.endsWith("/channels")) return Response.json(channels);
      return Response.json([message("1", "Current discussion")]);
    }),
  );
  render(Messaging);
  await expect
    .element(page.getByText("Current discussion", { exact: true }))
    .toBeVisible();
  await page.getByRole("button", { name: "频道 Agent", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog
    .getByRole("button", { name: "邀请 Review changes", exact: true })
    .click();
  await expect
    .element(dialog.getByRole("button", { name: "移除 Codex", exact: true }))
    .toBeVisible();
  await dialog.getByRole("button", { name: "移除 Codex", exact: true }).click();
  await expect
    .element(
      dialog.getByRole("button", { name: "邀请 Review changes", exact: true }),
    )
    .toBeVisible();
});

test("new agent replies arrive automatically while the draft stays intact", async () => {
  await enableCreationFeatures(["messaging"]);
  let replied = false;
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input) =>
      Response.json(
        String(input).endsWith("/channels")
          ? channels
          : [
              message("1", "Current discussion"),
              ...(replied
                ? [
                    {
                      ...message("2", "Review complete"),
                      agentId: "session:worker",
                      agentName: "Codex",
                    },
                  ]
                : []),
            ],
      ),
    ),
  );
  render(Messaging);
  await expect
    .element(page.getByText("Current discussion", { exact: true }))
    .toBeVisible();
  await box().fill("My unfinished question");
  replied = true;
  await expect
    .element(page.getByText("Review complete", { exact: true }))
    .toBeVisible();
  await expect.element(box()).toHaveValue("My unfinished question");
});
