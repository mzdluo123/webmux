import { z } from "zod";

export const ProjectSchema = z.object({
  id: z.string(),
  name: z.string(),
  path: z.string(),
  repositoryId: z.string(),
  availability: z.enum(["available", "unavailable"]),
  error: z.string(),
});
export type Project = z.infer<typeof ProjectSchema>;
export const ProjectsSchema = z.array(ProjectSchema);
export const OpenProjectRequestSchema = z.object({ path: z.string().min(1) });
export const CreateProjectRequestSchema = z.object({
  parent: z.string().min(1),
  name: z.string().min(1),
  initializeGit: z.boolean(),
});
export const ProjectCreationFailureSchema = z.object({
  error: z.string(),
  path: z.string(),
});
