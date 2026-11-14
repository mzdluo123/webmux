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
