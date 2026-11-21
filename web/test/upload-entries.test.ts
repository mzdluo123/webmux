import { expect, test, vi } from "vite-plus/test";
import {
  captureDroppedSource,
  ensureReadableFile,
  filesToDroppedSource,
  flattenUploadFiles,
  resolveUploadTree,
  UploadLimitError,
} from "../src/platform/upload-entries";

function fakeFileEntry(name: string, content = "x"): FileSystemFileEntry {
  return {
    isFile: true,
    isDirectory: false,
    name,
    fullPath: `/${name}`,
    filesystem: {} as FileSystem,
    getParent: () => {},
    file(success) {
      success(new File([content], name, { type: "text/plain" }));
    },
  };
}

function fakeDirEntry(
  name: string,
  batches: FileSystemEntry[][],
): FileSystemDirectoryEntry {
  return {
    isFile: false,
    isDirectory: true,
    name,
    fullPath: `/${name}`,
    filesystem: {} as FileSystem,
    getParent: () => {},
    getFile: () => {},
    getDirectory: () => {},
    createReader() {
      let index = 0;
      return {
        readEntries(success) {
          success(batches[index++] ?? []);
        },
      };
    },
  };
}

test("captureDroppedSource extracts entries and falls back to plain files", () => {
  const dir = fakeDirEntry("docs", [[fakeFileEntry("a.md")]]);
  const fromItems = captureDroppedSource({
    items: [
      { kind: "string", webkitGetAsEntry: () => null },
      { kind: "file", webkitGetAsEntry: () => dir },
    ],
    files: [],
  } as unknown as DataTransfer);
  expect(fromItems.hasDirectory).toBe(true);
  expect(fromItems.primaryName).toBe("docs");
  expect(fromItems.items).toHaveLength(1);

  const plain = new File(["hello"], "plain.txt");
  const fallback = captureDroppedSource({
    items: [],
    files: [plain],
  } as unknown as DataTransfer);
  expect(fallback.hasDirectory).toBe(false);
  expect(fallback.items).toEqual([{ kind: "file", file: plain }]);
});

test("resolveUploadTree walks multi-batch nested directories, preserves empty dirs, and skips .git", async () => {
  const tree = fakeDirEntry("project", [
    [
      fakeFileEntry("z.txt", "z"),
      fakeDirEntry(".git", [[fakeFileEntry("HEAD")]]),
    ],
    [
      fakeDirEntry("empty", []),
      fakeDirEntry("src", [[fakeFileEntry("main.ts", "export {}")]]),
    ],
  ]);
  const source = captureDroppedSource({
    items: [{ kind: "file", webkitGetAsEntry: () => tree }],
    files: [],
  } as unknown as DataTransfer);

  const resolved = await resolveUploadTree(source, { skipGit: true });
  expect(resolved.hadDirectory).toBe(true);
  expect(resolved.skippedGit).toBe(true);
  expect(resolved.fileCount).toBe(2);
  expect(resolved.directoryCount).toBe(3);
  expect(resolved.nodes).toMatchObject([
    {
      kind: "directory",
      name: "project",
      relativePath: "project",
      children: [
        {
          kind: "directory",
          name: "empty",
          relativePath: "project/empty",
          children: [],
        },
        {
          kind: "directory",
          name: "src",
          relativePath: "project/src",
          children: [
            {
              kind: "file",
              name: "main.ts",
              relativePath: "project/src/main.ts",
            },
          ],
        },
        { kind: "file", name: "z.txt", relativePath: "project/z.txt" },
      ],
    },
  ]);
  expect(flattenUploadFiles(resolved.nodes).map((f) => f.name)).toEqual([
    "main.ts",
    "z.txt",
  ]);
});

test("resolveUploadTree enforces maxFiles early, maxEntries, maxFileBytes, and abort", async () => {
  const many = fakeDirEntry("many", [
    [fakeFileEntry("a.txt"), fakeFileEntry("b.txt"), fakeFileEntry("c.txt")],
  ]);
  const source = captureDroppedSource({
    items: [{ kind: "file", webkitGetAsEntry: () => many }],
    files: [],
  } as unknown as DataTransfer);

  await expect(
    resolveUploadTree(source, { maxFiles: 2 }),
  ).rejects.toMatchObject({
    kind: "maxFiles",
  });
  await expect(
    resolveUploadTree(source, { maxEntries: 2 }),
  ).rejects.toBeInstanceOf(UploadLimitError);

  const oversized = filesToDroppedSource([
    new File([new Uint8Array(20)], "big.bin"),
  ]);
  await expect(
    resolveUploadTree(oversized, { maxFileBytes: 10 }),
  ).rejects.toMatchObject({
    kind: "maxFileBytes",
    detail: { name: "big.bin", relativePath: "big.bin" },
  });

  const controller = new AbortController();
  controller.abort();
  await expect(
    resolveUploadTree(source, { signal: controller.signal }),
  ).rejects.toMatchObject({ name: "AbortError" });
});

test("ensureReadableFile succeeds for empty and normal files and rejects unreadable handles", async () => {
  await expect(
    ensureReadableFile(new File([], "empty.txt")),
  ).resolves.toBeUndefined();
  await expect(
    ensureReadableFile(new File(["ok"], "ok.txt")),
  ).resolves.toBeUndefined();

  const broken = new File([], "folder-handle");
  vi.spyOn(broken, "slice").mockReturnValue({
    arrayBuffer: () =>
      Promise.reject(
        new DOMException(
          "The requested file could not be read",
          "NotReadableError",
        ),
      ),
  } as unknown as Blob);
  await expect(ensureReadableFile(broken)).rejects.toThrow(/folder-handle/);
});
