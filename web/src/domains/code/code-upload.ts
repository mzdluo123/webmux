import { m } from "$lib/paraglide/messages.js";
import { api, uploadForm } from "../../platform/api";
import {
  resolveUploadTree,
  type DroppedSource,
  type UploadNode,
} from "../../platform/upload-entries";
import {
  CodeMkdirRequestSchema,
  CodeMutationSchema,
} from "../../contracts/schemas";
import type { FileTarget } from "./code-file-actions";

interface CodeUploadProgress {
  name: string;
  percent: number;
  index: number;
  total: number;
  waiting: boolean;
  scanning?: boolean;
}

function throwCancelled(): never {
  throw new DOMException(m.s1_upload_cancelled(), "AbortError");
}

function buildUploadQuery(contextId: string, nodeId: string) {
  return new URLSearchParams({ contextId, nodeId });
}

export async function uploadCodeEntriesBatch(input: {
  base: string;
  contextId: string;
  into: FileTarget;
  source: DroppedSource;
  signal: AbortSignal;
  onProgress: (progress: CodeUploadProgress) => void;
  onCleanup: (storedCount: number) => void;
}): Promise<string> {
  const storedTopLevel: string[] = [];
  let storedCount = 0;
  let fileIndex = 0;

  try {
    if (input.source.hasDirectory) {
      input.onProgress({
        name: input.source.primaryName,
        percent: 0,
        index: 0,
        total: 0,
        waiting: false,
        scanning: true,
      });
    }
    const tree = await resolveUploadTree(input.source, {
      skipGit: true,
      signal: input.signal,
    });
    if (!tree.nodes.length) {
      if (tree.skippedGit) {
        throw new Error(m.s5_folder_only_git());
      }
      return "";
    }

    const uploadNodes = async (
      nodes: readonly UploadNode[],
      parentNodeId: string,
      topLevel: boolean,
    ) => {
      for (const node of nodes) {
        if (input.signal.aborted) throwCancelled();
        if (node.kind === "directory") {
          const result = await api(
            `${input.base}/mkdir`,
            CodeMutationSchema,
            "POST",
            {
              contextId: input.contextId,
              parentNodeId,
              name: node.name,
            },
            input.signal,
            CodeMkdirRequestSchema,
          );
          const created = result.nodes[0];
          if (!created) continue;
          storedCount += 1;
          if (topLevel) storedTopLevel.push(created.name);
          await uploadNodes(node.children, created.nodeId, false);
          continue;
        }
        fileIndex += 1;
        const current: CodeUploadProgress = {
          name: node.relativePath,
          percent: 0,
          index: fileIndex,
          total: tree.fileCount,
          waiting: false,
        };
        input.onProgress(current);
        const result = await uploadForm(
          `${input.base}/upload?${buildUploadQuery(input.contextId, parentNodeId)}`,
          node.file,
          CodeMutationSchema,
          (percent) => input.onProgress({ ...current, percent }),
          input.signal,
        );
        storedCount += result.nodes.length || 1;
        if (topLevel) {
          storedTopLevel.push(...result.nodes.map((n) => n.name));
        }
      }
    };

    await uploadNodes(tree.nodes, input.into.nodeId, true);
    return storedTopLevel.length
      ? m.s5_uploaded_files({
          names: storedTopLevel.join(m.s5_list_separator()),
        })
      : "";
  } finally {
    input.onCleanup(storedCount);
  }
}
