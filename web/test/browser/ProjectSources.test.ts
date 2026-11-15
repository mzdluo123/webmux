import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { cleanup, render } from "vitest-browser-svelte/pure";
import { page } from "vite-plus/test/browser";
import {
  createProjectSources,
  type ProjectSources,
} from "../../src/hooks/workspace/project-sources.svelte";
import ProjectPicker from "../../src/components/workspace/ProjectPicker.svelte";
import {
  setFeatureCapabilities,
  resetFeatureCapabilities,
} from "../../src/platform/feature-host.svelte";
import type { Project } from "../../src/contracts/schemas";

const project: Project = {
  id: "directory:cloned",
  name: "Cloned",
  path: "/projects/cloned",
  repositoryId: "",
  availability: "available",
  error: "",
};
const other: Project = {
  ...project,
  id: "directory:other",
  name: "Other",
  path: "/projects/other",
};
const id = "a".repeat(32);
const saved = {
  requestId: "saved-request",
  url: "https://example.invalid/repo.git",
  path: project.path,
  operationId: id,
};
let controllers: ProjectSources[];
let requests: string[];
let readProjects: () => Promise<Response>;
let readOperation: () => Promise<Response>;
let admitClone: () => Promise<Response>;
const onselect = vi.fn();
function operation(state = "succeeded") {
  return Response.json({
    id,
    state,
    path: project.path,
    error: "",
    projectId: state === "succeeded" ? project.id : "",
  });
}
function controller() {
  const result = createProjectSources(onselect);
  controllers.push(result);
  return result;
}
function capability(clone: boolean) {
  setFeatureCapabilities({
    version: 2,
    features: [{ id: "workspace", enabled: true, status: "ready" }],
    capabilities: [
      { id: "workspace.projects", available: true },
      {
        id: "workspace.clone",
        available: clone,
        ...(clone ? {} : { reason: "command_unavailable" as const }),
      },
    ],
  });
}
beforeEach(() => {
  localStorage.clear();
  onselect.mockReset();
  controllers = [];
  requests = [];
  readProjects = async () => Response.json([project, other]);
  readOperation = async () => operation();
  admitClone = async () => operation();
  capability(false);
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const path = new URL(String(input), location.href).pathname;
      const method = init?.method ?? "GET";
      requests.push(`${method} ${path}`);
      if (path === "/api/projects" && method === "GET") return readProjects();
      if (path === "/api/projects" && method === "POST")
        return Response.json(other);
      if (path === "/api/projects/clone") return admitClone();
      if (path.includes("/operations/")) return readOperation();
      return Response.json({ error: "unexpected endpoint" }, { status: 404 });
    }),
  );
});
afterEach(async () => {
  await cleanup();
  for (const source of controllers) source.dispose();
  vi.unstubAllGlobals();
  resetFeatureCapabilities();
  localStorage.clear();
});

test("clearing a completed clone while project loading wins over its late selection", async () => {
  localStorage.setItem("webmux.pendingProjectClone", JSON.stringify(saved));
  const delayed = Promise.withResolvers<Response>();
  readProjects = () => delayed.promise;
  const source = controller();
  const completion = source.resume();
  await expect.poll(() => requests.includes("GET /api/projects")).toBe(true);
  source.clearClone();
  delayed.resolve(Response.json([project, other]));
  await completion;
  expect(onselect).not.toHaveBeenCalled();
  expect(source.pending).toBeNull();
});

test("a user's other project selection wins over clone completion", async () => {
  localStorage.setItem("webmux.pendingProjectClone", JSON.stringify(saved));
  const delayed = Promise.withResolvers<Response>();
  readProjects = () => delayed.promise;
  const source = controller();
  const completion = source.resume();
  await expect.poll(() => requests.includes("GET /api/projects")).toBe(true);
  await source.select(other);
  delayed.resolve(Response.json([project, other]));
  await completion;
  expect(onselect).toHaveBeenCalledTimes(1);
  expect(onselect.mock.calls[0][0].project.id).toBe(other.id);
  expect(source.pending).toBeNull();
});

test("poll retry has one active request, clears old error and stops timers on clear", async () => {
  localStorage.setItem("webmux.pendingProjectClone", JSON.stringify(saved));
  readOperation = async () =>
    Response.json({ error: "temporarily offline" }, { status: 503 });
  const source = controller();
  await source.resume();
  expect(source.error).toContain("temporarily offline");
  const delayed = Promise.withResolvers<Response>();
  readOperation = () => delayed.promise;
  const first = source.resume();
  const duplicate = source.resume();
  expect(
    requests.filter((value) => value.includes("/operations/")).length,
  ).toBe(2);
  delayed.resolve(operation("running"));
  await Promise.all([first, duplicate]);
  expect(source.error).toBe("");
  source.clearClone();
  await new Promise((resolve) => setTimeout(resolve, 850));
  expect(
    requests.filter((value) => value.includes("/operations/")).length,
  ).toBe(2);
});

test("definitive 413 admission rejection frees local project creation", async () => {
  admitClone = async () =>
    Response.json({ error: "clone ledger full" }, { status: 413 });
  const source = controller();
  await source.clone(saved.url, saved.path);
  expect(source.pending).toBeNull();
  expect(localStorage.getItem("webmux.pendingProjectClone")).toBeNull();
  await source.open(other.path);
  expect(onselect).toHaveBeenCalledTimes(1);
});

test("a stale record can be discarded while clone capability is unavailable", async () => {
  localStorage.setItem(
    "webmux.pendingProjectClone",
    JSON.stringify({ ...saved, operationId: undefined }),
  );
  render(ProjectPicker, { defaultCwd: "/projects", onselect });
  await page
    .getByRole("button", { name: "暂不跟踪此克隆", exact: true })
    .click();
  expect(localStorage.getItem("webmux.pendingProjectClone")).toBeNull();
  await page.getByRole("button", { name: "添加项目…", exact: true }).click();
  await page.getByRole("option", { name: /本地文件夹/ }).click();
  await expect
    .element(
      page.getByRole("button", { name: "打开文件夹并继续", exact: true }),
    )
    .toBeEnabled();
  expect(requests.some((value) => value.includes("/clone"))).toBe(false);
});

test("capability arrival resumes saved work and unmount does not dispose a borrowed controller", async () => {
  localStorage.setItem("webmux.pendingProjectClone", JSON.stringify(saved));
  readOperation = async () => operation("running");
  const source = controller();
  const dispose = vi.spyOn(source, "dispose");
  render(ProjectPicker, {
    defaultCwd: "/projects",
    onselect,
    controller: source,
  });
  await expect.poll(() => source.loading).toBe(false);
  expect(requests.some((value) => value.includes("/operations/"))).toBe(false);
  capability(true);
  await expect
    .poll(() => requests.some((value) => value.includes("/operations/")))
    .toBe(true);
  await cleanup();
  expect(dispose).not.toHaveBeenCalled();
  await source.load();
  expect(source.loading).toBe(false);
});
