import { z } from "zod";
import { api, store, stored, APIError } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import {
  CreateProjectRequestSchema,
  ProjectCreationFailureSchema,
  GitHubProjectsSchema,
  OpenProjectRequestSchema,
  ProjectCloneOperationSchema,
  ProjectCloneRequestSchema,
  ProjectSchema,
  ProjectsSchema,
  RepositorySchema,
  type GitHubProjects,
  type Project,
  type ProjectCloneOperation,
} from "../../contracts/schemas";

import type { ProjectSelection } from "../../domains/sessions/project-selection";
const PendingCloneSchema = ProjectCloneRequestSchema.extend({
  operationId: z
    .string()
    .regex(/^[a-f0-9]{32}$/)
    .optional(),
});
type PendingClone = z.infer<typeof PendingCloneSchema>;
const pendingKey = "pendingProjectClone";
function savedClone(): PendingClone | null {
  try {
    return (
      PendingCloneSchema.safeParse(JSON.parse(stored(pendingKey))).data ?? null
    );
  } catch {
    return null;
  }
}

export type ProjectSources = ReturnType<typeof createProjectSources>;

// One owner for HTTP cancellation and polling. A persisted request survives
// browser closure; replay keeps the original request ID instead of recloning.
export function createProjectSources(
  onselect: (selection: ProjectSelection) => void,
) {
  let projects = $state.raw<Project[]>([]);
  let loading = $state(true);
  let busy = $state(false);
  let error = $state("");
  let github = $state.raw<GitHubProjects | null>(null);
  let githubLoading = $state(false);
  let operation = $state.raw<ProjectCloneOperation | null>(null);
  let pending = $state.raw(savedClone());
  let disposed = false;
  let githubGeneration = 0;
  let loadGeneration = 0;
  let cloneEpoch = 0;
  let pollingEpoch: number | null = null;
  let cloneAbort = new AbortController();
  let githubAbort: AbortController | undefined;
  let pollTimer: ReturnType<typeof setTimeout> | undefined;
  const abort = new AbortController();

  function stopObserving() {
    cloneEpoch++;
    cloneAbort.abort();
    cloneAbort = new AbortController();
    if (pollTimer) clearTimeout(pollTimer);
    pollTimer = undefined;
  }
  function remember(next: PendingClone | null) {
    pending = next;
    store(pendingKey, next ? JSON.stringify(next) : null);
  }
  async function select(project: Project) {
    if (busy || disposed || project.availability !== "available") return;
    stopObserving();
    busy = true;
    error = "";
    try {
      const repository = project.repositoryId
        ? await api(
            `/repositories/${encodeURIComponent(project.repositoryId)}/worktrees`,
            RepositorySchema,
            "GET",
            undefined,
            abort.signal,
          )
        : undefined;
      if (!disposed) {
        remember(null);
        operation = null;
        onselect({ project, repository });
      }
    } catch (err) {
      if (!disposed) error = errorMessage(err);
    } finally {
      if (!disposed) busy = false;
    }
  }
  async function load() {
    const generation = ++loadGeneration;
    loading = true;
    error = "";
    try {
      const result = await api(
        "/projects",
        ProjectsSchema,
        "GET",
        undefined,
        abort.signal,
      );
      if (!disposed && generation === loadGeneration) projects = result;
    } catch (err) {
      if (!disposed && generation === loadGeneration) error = errorMessage(err);
    } finally {
      if (!disposed && generation === loadGeneration) loading = false;
    }
  }
  async function complete(op: ProjectCloneOperation, epoch: number) {
    if (disposed || epoch !== cloneEpoch || !pending) return;
    operation = op;
    if (op.state === "succeeded") {
      await load();
      if (disposed || epoch !== cloneEpoch || !pending) return;
      const project = projects.find((p) => p.id === op.projectId);
      if (project) {
        remember(null);
        await select(project);
      } else {
        error ||= "克隆已完成，但项目暂未载入，请刷新项目列表";
      }
    } else if (op.state === "running" || op.state === "registering") {
      if (pollTimer) clearTimeout(pollTimer);
      pollTimer = setTimeout(() => void poll(op.id, epoch), 750);
    }
  }
  async function poll(id: string, epoch = cloneEpoch) {
    if (disposed || epoch !== cloneEpoch || pollingEpoch === epoch || !pending)
      return;
    if (pollTimer) clearTimeout(pollTimer);
    pollTimer = undefined;
    pollingEpoch = epoch;
    try {
      const op = await api(
        `/projects/operations/${encodeURIComponent(id)}`,
        ProjectCloneOperationSchema,
        "GET",
        undefined,
        cloneAbort.signal,
      );
      if (disposed || epoch !== cloneEpoch) return;
      error = "";
      await complete(op, epoch);
    } catch (err) {
      if (!disposed && epoch === cloneEpoch) error = errorMessage(err);
    } finally {
      if (pollingEpoch === epoch) pollingEpoch = null;
    }
  }
  async function submitClone(request: PendingClone) {
    if (
      busy ||
      disposed ||
      (pending && pending.requestId !== request.requestId)
    )
      return;
    stopObserving();
    const epoch = cloneEpoch;
    busy = true;
    error = "";
    remember(request);
    let admitted: ProjectCloneOperation | null = null;
    try {
      admitted = await api(
        "/projects/clone",
        ProjectCloneOperationSchema,
        "POST",
        request,
        cloneAbort.signal,
        ProjectCloneRequestSchema,
      );
      if (!disposed && epoch === cloneEpoch)
        remember({ ...request, operationId: admitted.id });
    } catch (err) {
      if (!disposed && epoch === cloneEpoch) {
        error = errorMessage(err);
        if (
          err instanceof APIError &&
          [400, 404, 409, 413, 422].includes(err.status)
        )
          remember(null);
      }
    } finally {
      if (!disposed && epoch === cloneEpoch) busy = false;
    }
    if (admitted) await complete(admitted, epoch);
  }
  return {
    get projects() {
      return projects;
    },
    get loading() {
      return loading;
    },
    get busy() {
      return busy;
    },
    get error() {
      return error;
    },
    get github() {
      return github;
    },
    get githubLoading() {
      return githubLoading;
    },
    get operation() {
      return operation;
    },
    get pending() {
      return pending;
    },
    load,
    select,
    async open(path: string) {
      if (busy) return;
      busy = true;
      error = "";
      let project: Project | undefined;
      try {
        project = await api(
          "/projects",
          ProjectSchema,
          "POST",
          { path },
          abort.signal,
          OpenProjectRequestSchema,
        );
      } catch (err) {
        if (!disposed) error = errorMessage(err);
      } finally {
        if (!disposed) busy = false;
      }
      if (project && !disposed) await select(project);
    },
    async create(parent: string, name: string, initializeGit: boolean) {
      if (busy) return;
      busy = true;
      error = "";
      let project: Project | undefined;
      try {
        project = await api(
          "/projects/create",
          ProjectSchema,
          "POST",
          { parent, name, initializeGit },
          abort.signal,
          CreateProjectRequestSchema,
        );
      } catch (err) {
        if (!disposed) {
          const retained =
            err instanceof APIError
              ? ProjectCreationFailureSchema.safeParse(err.body)
              : null;
          error = retained?.success
            ? `${errorMessage(err)}；目录已保留：${retained.data.path}。可返回本地文件夹来源重新登记。`
            : errorMessage(err);
        }
      } finally {
        if (!disposed) busy = false;
      }
      if (project && !disposed) await select(project);
    },
    clone(url: string, path: string, source?: "github") {
      return submitClone({ requestId: crypto.randomUUID(), url, path, source });
    },
    resume() {
      if (!pending || busy) return Promise.resolve();
      return pending.operationId
        ? poll(pending.operationId)
        : submitClone(pending);
    },
    async retryRegistration() {
      if (!operation || busy) return;
      const epoch = cloneEpoch;
      busy = true;
      error = "";
      let op: ProjectCloneOperation | undefined;
      try {
        op = await api(
          `/projects/operations/${encodeURIComponent(operation.id)}/register`,
          ProjectCloneOperationSchema,
          "POST",
          undefined,
          cloneAbort.signal,
        );
      } catch (err) {
        if (!disposed) error = errorMessage(err);
      } finally {
        if (!disposed) busy = false;
      }
      if (op) await complete(op, epoch);
    },
    clearClone() {
      if (busy) return;
      stopObserving();
      remember(null);
      operation = null;
      error = "";
    },
    async searchGitHub(query: string, page = 1) {
      const generation = ++githubGeneration;
      githubAbort?.abort();
      githubAbort = new AbortController();
      githubLoading = true;
      try {
        const result = await api(
          `/projects/github?query=${encodeURIComponent(query)}&page=${page}`,
          GitHubProjectsSchema,
          "GET",
          undefined,
          githubAbort.signal,
        );
        if (!disposed && generation === githubGeneration) github = result;
      } catch (err) {
        if (!disposed && generation === githubGeneration)
          error = errorMessage(err);
      } finally {
        if (!disposed && generation === githubGeneration) githubLoading = false;
      }
    },
    dispose() {
      disposed = true;
      stopObserving();
      abort.abort();
      githubAbort?.abort();
      if (pollTimer) clearTimeout(pollTimer);
    },
  };
}
