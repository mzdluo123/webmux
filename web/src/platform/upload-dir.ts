import type { z } from "zod";
import { api } from "./api";
import {
  UploadDirRequestSchema,
  UploadDirResultSchema,
} from "../contracts/schemas";

/** Creates one directory inside a session's uploads/ folder for folder uploads. */
export function createUploadDir(
  id: string,
  name: string,
  parent?: string,
  signal?: AbortSignal,
): Promise<z.infer<typeof UploadDirResultSchema>> {
  return api(
    `/sessions/${id}/uploads/dir`,
    UploadDirResultSchema,
    "POST",
    parent ? { name, parent } : { name },
    signal,
    UploadDirRequestSchema,
  );
}
