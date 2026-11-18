import { z } from "zod";
import { store, stored } from "../../platform/api";

const view = z.enum(["compact", "preview"]);
let compact = $state(
  view.safeParse(stored("homeSessionView")).data === "compact",
);

export const homeSessionView = {
  get compact() {
    return compact;
  },
  set compact(value: boolean) {
    compact = value;
    store("homeSessionView", value ? "compact" : "preview");
  },
};
