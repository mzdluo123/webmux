import { z } from "zod";
import { store, stored } from "../../platform/api";

const layoutSchema = z.enum(["queue", "workspaces"]);
type WorkbenchLayout = z.infer<typeof layoutSchema>;
let layout = $state(
  layoutSchema.safeParse(stored("workbenchLayout")).data ?? "queue",
);
let attentionOnly = $state(false);

export const workbenchLayout = {
  get value() {
    return layout;
  },
  set value(next: WorkbenchLayout) {
    layout = next;
    attentionOnly = false;
    store("workbenchLayout", next);
  },
  get attentionOnly() {
    return attentionOnly;
  },
  set attentionOnly(value: boolean) {
    attentionOnly = value;
  },
};
