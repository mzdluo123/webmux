import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { tick } from "svelte";
import { cleanup, render } from "vitest-browser-svelte/pure";
import ProjectPicker from "../../src/components/workspace/ProjectPicker.svelte";
import type { Project } from "../../src/contracts/schemas";

vi.mock("../../src/platform/feature-host.svelte", () => ({
  featureAvailable: () => true,
  featureCapabilityRevision: () => 0,
  requestFeatureHostRefresh: async () => {},
}));
const project: Project = {
  id: "directory:demo",
  name: "Alpha",
  path: "/projects/alpha",
  repositoryId: "",
  availability: "available",
  error: "",
};
let list: Project[];
let requests: { path: string; method: string; body: Record<string, unknown> }[];
let cloneState: "running" | "succeeded" | "registration_failed";
const id = "a".repeat(32);
const onselect = vi.fn();

beforeEach(() => {
  localStorage.clear();
  requests = [];
  list = [
    project,
    { ...project, id: "directory:other", name: "Beta", path: "/projects/beta" },
  ];
  cloneState = "running";
  onselect.mockReset();
  vi.spyOn(window, "fetch").mockImplementation(async (input, init) => {
    const path = new URL(String(input), location.href).pathname.replace(
      /^\/api/,
      "",
    );
    const method = init?.method ?? "GET";
    const body = init?.body
      ? (JSON.parse(String(init.body)) as Record<string, unknown>)
      : {};
    requests.push({ path, method, body });
    let result: unknown;
    if (path === "/projects" && method === "GET") result = list;
    else if (path === "/projects" || path === "/projects/create")
      result = {
        ...project,
        path: body.path ?? `${String(body.parent)}/${String(body.name)}`,
      };
    else if (path === "/projects/clone")
      result = {
        id,
        state: cloneState,
        path: "/projects/cloned",
        error: "",
        projectId: cloneState === "succeeded" ? project.id : "",
      };
    else if (path.includes("/operations/"))
      result = {
        id,
        state: cloneState,
        path: "/projects/cloned",
        error: cloneState === "registration_failed" ? "登记失败，目录保留" : "",
        projectId: cloneState === "succeeded" ? project.id : "",
      };
    else if (path === "/projects/github")
      result = {
        available: false,
        reason: "服务用户尚未登录 gh",
        login: "",
        repositories: [],
        page: 1,
        hasMore: false,
      };
    else throw new Error(`Unexpected request: ${method} ${path}`);
    return new Response(JSON.stringify(result), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  });
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  localStorage.clear();
});

function mount() {
  return render(ProjectPicker, {
    defaultCwd: "/projects",
    recent: [],
    onselect,
  });
}
async function source(name: string) {
  await page.getByRole("button", { name: "添加项目…", exact: true }).click();
  await page.getByRole("option", { name: new RegExp(name) }).click();
}

test("searches project names and paths and selects the whole row", async () => {
  mount();
  await page
    .getByRole("combobox", { name: "搜索项目", exact: true })
    .fill("beta");
  await expect
    .element(page.getByRole("option", { name: /Alpha/ }))
    .not.toBeInTheDocument();
  await page.getByRole("option", { name: /Beta/ }).click();
  await expect.poll(() => onselect.mock.calls.length).toBe(1);
  expect(onselect.mock.calls[0][0].project.path).toBe("/projects/beta");
});

test("empty project list offers all four sources and local opening registers explicitly", async () => {
  list = [];
  mount();
  await page.getByRole("option", { name: /本地文件夹/ }).click();
  await page
    .getByRole("textbox", { name: "已有文件夹", exact: true })
    .fill("/projects/existing");
  expect(requests.filter((r) => r.method === "POST")).toHaveLength(0);
  await page
    .getByRole("button", { name: "打开文件夹并继续", exact: true })
    .click();
  await expect.poll(() => onselect.mock.calls.length).toBe(1);
  expect(requests.find((r) => r.method === "POST")?.body).toEqual({
    path: "/projects/existing",
  });
});

test("back navigation preserves new project fields and optional Git starts off", async () => {
  mount();
  await source("新建项目");
  await expect
    .element(page.getByRole("checkbox", { name: "初始化 Git 仓库" }))
    .not.toBeChecked();
  await page
    .getByRole("textbox", { name: "项目名称", exact: true })
    .fill("my-project");
  await page.getByRole("button", { name: "返回上一层", exact: true }).click();
  await page.getByRole("option", { name: /新建项目/ }).click();
  await expect
    .element(page.getByRole("textbox", { name: "项目名称", exact: true }))
    .toHaveValue("my-project");
  await page
    .getByRole("button", { name: "创建项目并继续", exact: true })
    .click();
  await expect.poll(() => onselect.mock.calls.length).toBe(1);
  expect(requests.find((r) => r.path === "/projects/create")?.body).toEqual({
    parent: "/projects",
    name: "my-project",
    initializeGit: false,
  });
});

test("GitHub configuration failure stays readable and other sources remain accessible", async () => {
  mount();
  await source("GitHub 仓库");
  await expect
    .element(page.getByRole("status"))
    .toHaveTextContent("服务用户尚未登录 gh");
  await page.getByRole("button", { name: "返回上一层", exact: true }).click();
  await page.getByRole("option", { name: /本地文件夹/ }).click();
  await expect
    .element(page.getByRole("textbox", { name: "已有文件夹", exact: true }))
    .toBeVisible();
});

test("editing the GitHub query immediately withdraws the previous options", async () => {
  const fetchMock = vi.mocked(window.fetch);
  const fetchProject = fetchMock.getMockImplementation()!;
  fetchMock.mockImplementation(async (input, init) => {
    const url = new URL(String(input), location.href);
    if (url.pathname !== "/api/projects/github")
      return fetchProject(input, init);
    const query = url.searchParams.get("query") || "old";
    return Response.json({
      available: true,
      reason: "",
      login: "audit",
      repositories: [
        {
          fullName: `audit/${query}`,
          description: "",
          private: false,
          cloneUrl: `https://github.com/audit/${query}.git`,
        },
      ],
      page: 1,
      hasMore: false,
    });
  });
  mount();
  await source("GitHub 仓库");
  await expect
    .element(page.getByRole("option", { name: /audit\/old/ }))
    .toBeVisible();
  const search = page.getByRole("combobox");
  const input = search.element() as HTMLInputElement;
  input.value = "new";
  input.dispatchEvent(new InputEvent("input", { bubbles: true }));
  await tick();
  expect(document.querySelector('[role="option"]')).toBeNull();
  input.dispatchEvent(
    new KeyboardEvent("keydown", { key: "Enter", bubbles: true }),
  );
  expect(document.body.textContent).not.toContain(
    "https://github.com/audit/old.git",
  );
  await expect
    .element(page.getByRole("option", { name: /audit\/new/ }))
    .toBeVisible();
});

test("browser reopening polls the admitted clone without another POST", async () => {
  mount();
  await source("Git URL");
  await page
    .getByRole("textbox", { name: "Git 仓库地址", exact: true })
    .fill("https://example.invalid/repo.git");
  await page
    .getByRole("textbox", { name: "克隆到新目录", exact: true })
    .fill("/projects/cloned");
  await page.getByRole("button", { name: "克隆并继续", exact: true }).click();
  await expect
    .poll(() => requests.filter((r) => r.path === "/projects/clone").length)
    .toBe(1);
  await expect
    .poll(() => localStorage.getItem("webmux.pendingProjectClone"))
    .toContain(id);
  await cleanup();
  cloneState = "succeeded";
  mount();
  await expect.poll(() => onselect.mock.calls.length).toBe(1);
  expect(requests.filter((r) => r.path === "/projects/clone")).toHaveLength(1);
  expect(localStorage.getItem("webmux.pendingProjectClone")).toBeNull();
});

test("registration failure retains path and retries registration without cloning", async () => {
  localStorage.setItem(
    "webmux.pendingProjectClone",
    JSON.stringify({
      requestId: "recover",
      url: "https://example.invalid/repo.git",
      path: "/projects/cloned",
      operationId: id,
    }),
  );
  cloneState = "registration_failed";
  mount();
  await expect
    .element(page.getByText("登记失败，目录保留", { exact: true }))
    .toBeVisible();
  cloneState = "succeeded";
  await page.getByRole("button", { name: "重试项目登记", exact: true }).click();
  await expect.poll(() => onselect.mock.calls.length).toBe(1);
  expect(requests.filter((r) => r.path === "/projects/clone")).toHaveLength(0);
  expect(
    requests.some((r) => r.path.endsWith("/register") && r.method === "POST"),
  ).toBe(true);
});

test("source search matches the visible Chinese labels", async () => {
  mount();
  await page.getByRole("button", { name: "添加项目…", exact: true }).click();
  await page
    .getByRole("combobox", { name: "搜索项目来源", exact: true })
    .fill("本地");
  await expect
    .element(page.getByRole("option", { name: /本地文件夹/ }))
    .toBeVisible();
  await expect
    .element(page.getByRole("option", { name: /新建项目/ }))
    .not.toBeInTheDocument();
});

test("remote URL suggests a destination but preserves an edited target", async () => {
  mount();
  await source("Git URL");
  await page
    .getByRole("textbox", { name: "Git 仓库地址", exact: true })
    .fill("https://example.invalid/demo/repo.git");
  const target = page.getByRole("textbox", {
    name: "克隆到新目录",
    exact: true,
  });
  await target.click();
  await expect.element(target).toHaveValue("/projects/repo");
  await target.fill("/projects/custom-target");
  await page
    .getByRole("textbox", { name: "Git 仓库地址", exact: true })
    .fill("https://example.invalid/demo/other.git");
  await target.click();
  await expect.element(target).toHaveValue("/projects/custom-target");
  expect(requests.some((request) => request.method === "POST")).toBe(false);
});
