import { SvelteSet } from "svelte/reactivity";
import { z } from "zod";
import { onActiveHostChange, store, stored } from "../../platform/storage";

const layoutSchema = z.enum(["queue", "workspaces"]);
type WorkbenchLayout = z.infer<typeof layoutSchema>;
let layout = $state(
  layoutSchema.safeParse(stored("workbenchLayout")).data ?? "queue",
);
let attentionOnly = $state(false);

const projectStorageKey = "workbenchCollapsedProjects";
const collapsedProjectsSchema = z.array(z.string());

function readCollapsedProjects() {
  try {
    return new SvelteSet(
      collapsedProjectsSchema.parse(
        JSON.parse(stored(projectStorageKey, "[]")),
      ),
    );
  } catch {
    return new SvelteSet<string>();
  }
}

let collapsedProjects = $state(readCollapsedProjects());
onActiveHostChange(() => {
  collapsedProjects = readCollapsedProjects();
});

export const workbenchLayout = {
  get value() {
    return layout;
  },
  set value(next: WorkbenchLayout) {
    layout = next;
    attentionOnly = false;
    store("workbenchLayout", next);
  },
  isProjectOpen(id: string) {
    return !collapsedProjects.has(id);
  },
  setProjectOpen(id: string, open: boolean) {
    if (open) collapsedProjects.delete(id);
    else collapsedProjects.add(id);
    store(projectStorageKey, JSON.stringify([...collapsedProjects]));
  },
  get attentionOnly() {
    return attentionOnly;
  },
  set attentionOnly(value: boolean) {
    attentionOnly = value;
  },
};
