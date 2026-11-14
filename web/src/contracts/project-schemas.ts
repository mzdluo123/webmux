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
