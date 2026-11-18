import { SvelteSet } from "svelte/reactivity";
import { z } from "zod";
import { onActiveHostChange, store, stored } from "../../platform/storage";

const layoutSchema = z.enum(["queue", "workspaces"]);
type WorkbenchLayout = z.infer<typeof layoutSchema>;
let layout = $state(
  layoutSchema.safeParse(stored("workbenchLayout")).data ?? "queue",
);
let attentionOnly = $state(false);
const collapsedAttentionProjects = new SvelteSet<string>();

function filterAttention(value: boolean) {
  if (value === attentionOnly) return;
  attentionOnly = value;
  collapsedAttentionProjects.clear();
}

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
  filterAttention(false);
});

export const workbenchLayout = {
  get value() {
    return layout;
  },
  set value(next: WorkbenchLayout) {
    layout = next;
    store("workbenchLayout", next);
  },
  isProjectOpen(id: string) {
    return !(
      attentionOnly ? collapsedAttentionProjects : collapsedProjects
    ).has(id);
  },
  setProjectOpen(id: string, open: boolean) {
    const projects = attentionOnly
      ? collapsedAttentionProjects
      : collapsedProjects;
    if (open) projects.delete(id);
    else projects.add(id);
    if (!attentionOnly) store(projectStorageKey, JSON.stringify([...projects]));
  },
  get attentionOnly() {
    return attentionOnly;
  },
  set attentionOnly(value: boolean) {
    filterAttention(value);
  },
};
