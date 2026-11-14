import type { Project, Repository } from "../../contracts/schemas";

// A project may have just been registered and not yet reached the host's
// repository list. Carry its validated snapshot through session setup.
export interface ProjectSelection {
  project: Project;
  repository?: Repository;
}
