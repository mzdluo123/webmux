import { z } from "zod";

export const ProjectSchema = z.object({
  id: z.string(),
  name: z.string(),
  path: z.string(),
  repositoryId: z.string(),
  availability: z.enum(["available", "unavailable"]),
  error: z.string(),
});
export const ProjectsSchema = z.array(ProjectSchema);

export const ProjectCloneOperationSchema = z.object({
  id: z.string(),
  state: z.enum([
    "running",
    "registering",
    "succeeded",
    "failed",
    "interrupted",
    "registration_failed",
  ]),
  path: z.string(),
  error: z.string(),
  projectId: z.string(),
});

export const GitHubProjectsSchema = z.object({
  available: z.boolean(),
  reason: z.string(),
  login: z.string(),
  repositories: z.array(
    z.object({
      fullName: z.string(),
      description: z.string(),
      private: z.boolean(),
      cloneUrl: z.string(),
    }),
  ),
  page: z.number().int().min(1).max(50),
  hasMore: z.boolean(),
});

export type Project = z.infer<typeof ProjectSchema>;
export type ProjectCloneOperation = z.infer<typeof ProjectCloneOperationSchema>;
export type GitHubProjects = z.infer<typeof GitHubProjectsSchema>;
export const OpenProjectRequestSchema = z.object({ path: z.string().min(1) });
export const CreateProjectRequestSchema = z.object({
  parent: z.string().min(1),
  name: z.string().min(1),
  initializeGit: z.boolean(),
});
export const ProjectCloneRequestSchema = z.object({
  requestId: z.string().regex(/^[a-zA-Z0-9][a-zA-Z0-9_-]{0,119}$/),
  url: z.string().min(1),
  path: z.string().min(1),
  source: z.literal("github").optional(),
});
export const ProjectCreationFailureSchema = z.object({
  error: z.string(),
  path: z.string().min(1),
});
