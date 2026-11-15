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
  A: { icon: IconSquareLetterA, label: "新增", color: "text-success" },
  M: { icon: IconSquareLetterM, label: "修改", color: "text-warning" },
  D: { icon: IconSquareLetterD, label: "删除", color: "text-destructive" },
  R: { icon: IconSquareLetterR, label: "重命名", color: "text-info" },
  C: { icon: IconSquareLetterC, label: "复制", color: "text-info" },
  T: { icon: IconSquareLetterT, label: "类型变更", color: "text-info" },
  U: { icon: IconAlertTriangle, label: "冲突", color: "text-destructive" },
  untracked: {
    icon: IconSquareLetterU,
    label: "未跟踪",
    color: "text-success",
  },
  unknown: {
    icon: IconSquareLetterX,
    label: "未知",
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
