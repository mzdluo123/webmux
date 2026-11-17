import { APIError } from "../../platform/api";
import {
  SessionConflictSchema,
  type SessionConflict,
} from "../../contracts/schemas";

/** The session conflict a 409 carries, or undefined for any other failure. */
export function sessionConflict(err: unknown): SessionConflict | undefined {
  if (!(err instanceof APIError) || err.status !== 409) return undefined;
  const conflict = SessionConflictSchema.safeParse(err.body);
  return conflict.success ? conflict.data : undefined;
}

export function isDirectoryNotFoundError(err: unknown): boolean {
  return (
    err instanceof APIError &&
    err.status === 400 &&
    err.message.startsWith("directory:") &&
    err.message.includes("no such file or directory")
  );
}
