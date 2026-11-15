import { m } from "$lib/paraglide/messages.js";
import IconAlertTriangle from "~icons/tabler/alert-triangle";
import IconSquareLetterA from "~icons/tabler/square-letter-a";
import IconSquareLetterC from "~icons/tabler/square-letter-c";
import IconSquareLetterD from "~icons/tabler/square-letter-d";
import IconSquareLetterM from "~icons/tabler/square-letter-m";
import IconSquareLetterR from "~icons/tabler/square-letter-r";
import IconSquareLetterT from "~icons/tabler/square-letter-t";
import IconSquareLetterU from "~icons/tabler/square-letter-u";
import IconSquareLetterX from "~icons/tabler/square-letter-x";

const statuses = {
  A: {
    icon: IconSquareLetterA,
    get label() {
      return m.s1_git_status_added();
    },
    color: "text-success",
  },
  M: {
    icon: IconSquareLetterM,
    get label() {
      return m.s1_git_status_modified();
    },
    color: "text-warning",
  },
  D: {
    icon: IconSquareLetterD,
    get label() {
      return m.s1_git_status_deleted();
    },
    color: "text-destructive",
  },
  R: {
    icon: IconSquareLetterR,
    get label() {
      return m.s1_git_status_renamed();
    },
    color: "text-info",
  },
  C: {
    icon: IconSquareLetterC,
    get label() {
      return m.s1_git_status_copied();
    },
    color: "text-info",
  },
  T: {
    icon: IconSquareLetterT,
    get label() {
      return m.s1_git_status_type_changed();
    },
    color: "text-info",
  },
  U: {
    icon: IconAlertTriangle,
    get label() {
      return m.s1_git_status_conflicted();
    },
    color: "text-destructive",
  },
  untracked: {
    icon: IconSquareLetterU,
    get label() {
      return m.s1_git_status_untracked();
    },
    color: "text-success",
  },
  unknown: {
    icon: IconSquareLetterX,
    get label() {
      return m.s1_git_status_unknown();
    },
    color: "text-muted-foreground",
  },
};
export function gitFileStatus(change: { status: string; group?: string }) {
  if (change.group === "conflicted") return statuses.U;
  if (change.group === "untracked") return statuses.untracked;
  return (
    statuses[change.status.charAt(0) as keyof typeof statuses] ??
    statuses.unknown
  );
}
