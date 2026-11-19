import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { cleanup, render } from "vitest-browser-svelte/pure";
import TerminalUploadHarness from "./TerminalUploadHarness.svelte";
import type {
  useUpload,
  UploadInput,
} from "../../src/hooks/shared/upload.svelte";
import type { Connection } from "../../src/platform/ws";
import type {
  DraftOperation,
  InputDestination,
  TextInsertion,
} from "../../src/domains/terminal/terminal-draft.svelte";

class FakeXHR {
  static pending: FakeXHR[] = [];
  upload: { onprogress: ((event: ProgressEvent) => void) | null } = {
    onprogress: null,
  };
  url = "";
  status = 0;
  statusText = "";
  response: unknown;
  responseType = "";
  timeout = 0;
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onabort: (() => void) | null = null;
  ontimeout: (() => void) | null = null;

  open(_method: string, url: string) {
    this.url = url;
  }
  setRequestHeader() {}
  send() {
    FakeXHR.pending.push(this);
  }
  abort() {
    this.onabort?.();
  }
}

beforeEach(() => {
  FakeXHR.pending = [];
  vi.stubGlobal("XMLHttpRequest", FakeXHR);
});

afterEach(async () => {
  try {
    await cleanup();
  } finally {
    vi.unstubAllGlobals();
  }
});

function finish(path: string) {
  const xhr = FakeXHR.pending.at(-1)!;
  xhr.status = 201;
  xhr.response = { path };
  xhr.onload?.();
}

function setup(options?: {
  connection?: Connection;
  exitCode?: number | null;
  restoring?: boolean;
  destination?: InputDestination;
  insertResult?: TextInsertion;
}) {
  const destination: InputDestination = options?.destination ?? "terminal";
  const insertResult: TextInsertion = options?.insertResult ?? "terminal";
  const focus = vi.fn();
  const inserted: string[] = [];
  const errors: string[] = [];

  let seq = 0;
  const input: UploadInput = {
    begin: () => ({ id: ++seq }) as unknown as DraftOperation,
    destination: () => destination,
    insert: (text) => {
      inserted.push(text);
      return insertResult;
    },
  };

  let api!: {
    upload: ReturnType<typeof useUpload>;
    host: HTMLElement;
    setConnection: (c: Connection) => void;
    setExitCode: (code: number | null) => void;
    setRestoring: (r: boolean) => void;
  };
  render(TerminalUploadHarness, {
    initialConnection: options?.connection ?? "connected",
    initialExitCode: options?.exitCode ?? null,
    initialRestoring: options?.restoring ?? false,
    focus,
    input,
    setError: (msg: string) => errors.push(msg),
    oninit: (instance) => {
      api = instance;
    },
  });

  return {
    get upload() {
      return api?.upload;
    },
    get host() {
      return api?.host;
    },
    inserted,
    errors,
    focus,
    setConnection: (c: Connection) => api.setConnection(c),
    setExitCode: (code: number | null) => api.setExitCode(code),
    setRestoring: (r: boolean) => api.setRestoring(r),
  };
}

test("uploads a file, quotes single quotes in path, and inserts when connected", async () => {
  const ctx = setup();
  await expect.poll(() => Boolean(ctx.upload)).toBe(true);

  const promise = ctx.upload.uploadFiles([
    new File(["hello"], "it's.txt", { type: "text/plain" }),
  ]);
  await expect.poll(() => FakeXHR.pending.length).toBe(1);
  expect(ctx.upload.progress?.name).toBe("it's.txt");

  finish("/tmp/upload/it's.txt");
  await promise;

  expect(ctx.inserted).toEqual(["'/tmp/upload/it'\\''s.txt' "]);
  expect(ctx.upload.progress).toBeNull();
  expect(ctx.errors.filter(Boolean)).toEqual([]);
});

test("waits for restoring terminal to settle and rejects if session exits", async () => {
  const ctx = setup({ connection: "reconnecting", restoring: true });
  await expect.poll(() => Boolean(ctx.upload)).toBe(true);

  const promise = ctx.upload.uploadFiles([
    new File(["a"], "wait.txt", { type: "text/plain" }),
  ]);
  await expect.poll(() => FakeXHR.pending.length).toBe(1);
  finish("/tmp/upload/wait.txt");
  await expect.poll(() => ctx.upload.progress?.waiting).toBe(true);
  expect(ctx.inserted).toEqual([]);

  ctx.setConnection("connected");
  ctx.setRestoring(false);
  await promise;
  expect(ctx.inserted).toEqual(["'/tmp/upload/wait.txt' "]);

  // Refuses upload when session has already exited.
  ctx.setExitCode(1);
  await ctx.upload.uploadFiles([
    new File(["b"], "after-exit.txt", { type: "text/plain" }),
  ]);
  expect(ctx.errors.at(-1)).toBe("终端会话已退出或不存在，无法上传文件。");
});

test("handles drag-and-drop and clipboard paste on the terminal host", async () => {
  const ctx = setup();
  await expect.poll(() => Boolean(ctx.host)).toBe(true);

  const transfer = new DataTransfer();
  transfer.items.add(new File(["d"], "drop.txt", { type: "text/plain" }));
  const child = ctx.host.appendChild(document.createElement("div"));
  function drag(target: HTMLElement, type: string, dataTransfer = transfer) {
    const event = new DragEvent(type, {
      bubbles: true,
      cancelable: true,
      dataTransfer,
    });
    target.dispatchEvent(event);
    return event;
  }

  drag(ctx.host, "dragenter");
  expect(ctx.upload.dragging).toBe(true);
  drag(child, "dragenter");
  drag(ctx.host, "dragleave");
  expect(ctx.upload.dragging).toBe(true);
  drag(child, "dragleave");
  expect(ctx.upload.dragging).toBe(false);

  const childDrop = vi.fn();
  child.addEventListener("drop", childDrop);
  const text = new DataTransfer();
  text.setData("text/plain", "normal drag");
  expect(drag(child, "drop", text).defaultPrevented).toBe(false);
  expect(childDrop).toHaveBeenCalledTimes(1);
  expect(FakeXHR.pending).toHaveLength(0);

  drag(child, "dragenter");
  expect(drag(child, "drop").defaultPrevented).toBe(true);
  expect(childDrop).toHaveBeenCalledTimes(1);
  expect(ctx.upload.dragging).toBe(false);
  await expect.poll(() => FakeXHR.pending.length).toBe(1);
  finish("/tmp/upload/drop.txt");
  await expect.poll(() => ctx.inserted.length).toBe(1);

  const pasteTransfer = new DataTransfer();
  pasteTransfer.items.add(new File(["p"], "paste.txt", { type: "text/plain" }));
  const pasteEvent = new ClipboardEvent("paste", {
    bubbles: true,
    cancelable: true,
    clipboardData: pasteTransfer,
  });
  expect(ctx.upload.filePaste(pasteEvent)).toBe(true);
  await expect.poll(() => FakeXHR.pending.length).toBe(2);
  ctx.upload.cancel();
  await expect.poll(() => ctx.upload.progress).toBeNull();
});

test("dropping a folder onto the terminal uploads its nested tree and inserts the folder path", async () => {
  const createdDirs: { name: string; parent?: string }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      const body = JSON.parse(String(init?.body ?? "{}")) as {
        name: string;
        parent?: string;
      };
      createdDirs.push(body);
      const rel = body.parent ? `${body.parent}/${body.name}` : body.name;
      return Response.json(
        { path: `/tmp/uploads/${rel}`, relativePath: rel },
        { status: 201 },
      );
    }),
  );

  const ctx = setup();
  await expect.poll(() => Boolean(ctx.host)).toBe(true);

  const fileEntry = (name: string, text: string): FileSystemFileEntry => ({
    isFile: true,
    isDirectory: false,
    name,
    fullPath: `/${name}`,
    filesystem: {} as FileSystem,
    getParent: () => {},
    file(success) {
      success(new File([text], name, { type: "text/plain" }));
    },
  });
  const dirEntry = (
    name: string,
    children: FileSystemEntry[],
  ): FileSystemDirectoryEntry => ({
    isFile: false,
    isDirectory: true,
    name,
    fullPath: `/${name}`,
    filesystem: {} as FileSystem,
    getParent: () => {},
    getFile: () => {},
    getDirectory: () => {},
    createReader() {
      let done = false;
      return {
        readEntries(success) {
          if (done) return success([]);
          done = true;
          success(children);
        },
      };
    },
  });

  const folder = dirEntry("specs", [
    fileEntry("a.md", "hello"),
    dirEntry("sub", [fileEntry("b.md", "world")]),
    dirEntry("empty", []),
  ]);
  const event = new Event("drop", { bubbles: true, cancelable: true });
  Object.defineProperty(event, "dataTransfer", {
    value: {
      types: ["Files"],
      items: [{ kind: "file", webkitGetAsEntry: () => folder }],
      files: [],
    },
  });
  ctx.host.dispatchEvent(event);

  await expect.poll(() => FakeXHR.pending.length).toBe(1);
  expect(FakeXHR.pending[0]?.url).toContain("dir=specs");
  finish("/tmp/uploads/specs/a.md");

  await expect.poll(() => FakeXHR.pending.length).toBe(2);
  expect(FakeXHR.pending[1]?.url).toContain("dir=specs%2Fsub");
  finish("/tmp/uploads/specs/sub/b.md");

  await expect.poll(() => ctx.inserted.length).toBe(1);
  expect(createdDirs).toEqual([
    { name: "specs" },
    { name: "empty", parent: "specs" },
    { name: "sub", parent: "specs" },
  ]);
  expect(ctx.inserted).toEqual(["'/tmp/uploads/specs' "]);
  expect(ctx.errors.filter(Boolean)).toEqual([]);
});

test("an unreadable file handle reports a local read error instead of a network error", async () => {
  const ctx = setup();
  await expect.poll(() => Boolean(ctx.upload)).toBe(true);

  const unreadable = new File([], "broken-folder");
  vi.spyOn(unreadable, "slice").mockReturnValue({
    arrayBuffer: () =>
      Promise.reject(
        new DOMException(
          "The requested file could not be read",
          "NotReadableError",
        ),
      ),
  } as unknown as Blob);

  const promise = ctx.upload.uploadFiles([unreadable]);
  await expect.poll(() => FakeXHR.pending.length).toBe(1);
  FakeXHR.pending[0]?.onerror?.();
  await promise;

  expect(ctx.errors.at(-1)).toBe(
    "无法读取文件 broken-folder（可能是文件夹或无访问权限）",
  );
  expect(ctx.inserted).toEqual([]);
});
