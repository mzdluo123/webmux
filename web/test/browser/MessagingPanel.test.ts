import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import MessagingPanel from "../../src/components/settings/MessagingPanel.svelte";
import {
  enableCreationFeatures,
  resetCreationFeatures,
} from "./feature-fixture";

const message = (id: string, text: string) => ({
  id,
  text,
  agentId: "agent-1",
  agentName: "Alice",
  createdAt: "2026-11-17T08:00:00Z",
});

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  resetCreationFeatures();
  vi.unstubAllGlobals();
});

test("unavailable message storage reports its state without network I/O", async () => {
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  await enableCreationFeatures([]);
  render(MessagingPanel);
  await expect.element(page.getByText(/消息存储不可用/)).toBeVisible();
  expect(fetch).not.toHaveBeenCalled();
});

test("reads in chronological order, retains channel drafts and reports sends", async () => {
  await enableCreationFeatures(["messaging"]);
  let rejectSend = true;
  const calls: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const url = String(input);
      calls.push(`${init?.method ?? "GET"} ${url}`);
      if (init?.method === "POST") {
        if (rejectSend)
          return Response.json({ error: "消息存储不可用" }, { status: 503 });
        expect(JSON.parse(String(init.body))).toEqual({
          text: "hello team",
          clientRequestId: expect.any(String),
        });
        return Response.json(message("101", "hello team"), { status: 201 });
      }
      if (url.endsWith("/channels"))
        return Response.json([
          { id: "1", name: "general", topic: "Team coordination" },
          { id: "2", name: "builds", topic: "" },
        ]);
      if (url.includes("/general/"))
        return Response.json([
          message("99", "older message"),
          message("100", "newer message"),
        ]);
      return Response.json([]);
    }),
  );
  render(MessagingPanel);
  await expect
    .element(page.getByText("older message", { exact: true }))
    .toBeVisible();
  const paragraphs = [...document.querySelectorAll("article p")].map(
    (item) => item.textContent,
  );
  expect(paragraphs).toEqual(["older message", "newer message"]);
  await page
    .getByRole("textbox", { name: "消息", exact: true })
    .fill("hello team");
  await page.getByRole("combobox").selectOptions("builds");
  await expect
    .element(page.getByText("此频道暂无消息。", { exact: true }))
    .toBeVisible();
  await page
    .getByRole("textbox", { name: "消息", exact: true })
    .fill("build draft");
  await page.getByRole("combobox").selectOptions("general");
  await expect
    .element(page.getByText("older message", { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByRole("textbox", { name: "消息", exact: true }))
    .toHaveValue("hello team");
  await page.getByRole("button", { name: "发送消息", exact: true }).click();
  await expect.element(page.getByText(/未能确认发送结果/)).toBeVisible();
  await expect
    .element(page.getByRole("textbox", { name: "消息", exact: true }))
    .toHaveValue("hello team");
  rejectSend = false;
  await page.getByRole("button", { name: "发送消息", exact: true }).click();
  await expect
    .element(page.getByText("消息已发送。", { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByRole("textbox", { name: "消息", exact: true }))
    .toHaveValue("");
  expect(calls.filter((item) => item.startsWith("POST"))).toHaveLength(2);
});

test("a slow old channel response cannot overwrite the newly selected channel", async () => {
  await enableCreationFeatures(["messaging"]);
  let release: ((response: Response) => void) | undefined;
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input) => {
      const url = String(input);
      if (url.endsWith("/channels"))
        return Response.json([
          { id: "1", name: "general", topic: "" },
          { id: "2", name: "builds", topic: "" },
        ]);
      if (url.includes("/general/"))
        return new Promise<Response>((resolve) => {
          release = resolve;
        });
      return Response.json([message("102", "build complete")]);
    }),
  );
  render(MessagingPanel);
  await expect.poll(() => release).toBeDefined();
  await page.getByRole("combobox").selectOptions("builds");
  await expect
    .element(page.getByText("build complete", { exact: true }))
    .toBeVisible();
  release?.(Response.json([message("103", "stale message")]));
  await expect
    .element(page.getByText("stale message", { exact: true }))
    .not.toBeInTheDocument();
  await expect
    .element(page.getByText("build complete", { exact: true }))
    .toBeVisible();
});

test("creates the first local channel and can send without external setup", async () => {
  await enableCreationFeatures(["messaging"]);
  let created = false;
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const url = String(input);
      if (init?.method === "POST" && url.endsWith("/channels")) {
        expect(JSON.parse(String(init.body))).toEqual({
          name: "general",
          topic: "Local team",
        });
        created = true;
        return Response.json(
          { id: "1", name: "general", topic: "Local team" },
          { status: 201 },
        );
      }
      if (url.endsWith("/channels"))
        return Response.json(
          created ? [{ id: "1", name: "general", topic: "Local team" }] : [],
        );
      return Response.json([]);
    }),
  );
  render(MessagingPanel);
  await expect
    .element(page.getByText("创建一个频道开始交流。", { exact: true }))
    .toBeVisible();
  await page
    .getByRole("textbox", { name: "频道名称", exact: true })
    .fill("general");
  await page
    .getByRole("textbox", { name: "频道主题（可选）", exact: true })
    .fill("Local team");
  await page.getByRole("button", { name: "创建频道", exact: true }).click();
  await expect.element(page.getByRole("combobox")).toHaveValue("general");
  await expect
    .element(page.getByText("Local team", { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByRole("textbox", { name: "消息", exact: true }))
    .toBeVisible();
});

test("loads earlier history using the first message as a stable cursor", async () => {
  await enableCreationFeatures(["messaging"]);
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input) => {
      const url = String(input);
      if (url.endsWith("/channels"))
        return Response.json([{ id: "1", name: "general", topic: "" }]);
      if (url.includes("before=50"))
        return Response.json([message("49", "earlier history")]);
      return Response.json(
        Array.from({ length: 50 }, (_, i) =>
          message(String(i + 50), `message ${i + 50}`),
        ),
      );
    }),
  );
  render(MessagingPanel);
  await page.getByRole("button", { name: "加载更早消息", exact: true }).click();
  await expect
    .element(page.getByText("earlier history", { exact: true }))
    .toBeInTheDocument();
  await expect
    .element(page.getByRole("button", { name: "加载更早消息", exact: true }))
    .not.toBeInTheDocument();
});
