import type { CodeWorkspaceTab } from "./code-workspace";

export function groupDocuments(
  tabs: CodeWorkspaceTab[],
  assignment: Record<string, number> | undefined,
  showEmpty: boolean,
) {
  const groups = new Map<number, CodeWorkspaceTab[]>();
  for (const tab of tabs) {
    const group = assignment?.[tab.key] ?? 0;
    const list = groups.get(group);
    if (list) list.push(tab);
    else groups.set(group, [tab]);
  }
  if (!groups.size && showEmpty) groups.set(0, []);
  return groups;
}

export function selectedDocument(
  tabs: CodeWorkspaceTab[],
  active: string,
  previous?: string,
) {
  return (
    tabs.find((tab) => tab.key === active) ??
    tabs.find((tab) => tab.key === previous) ??
    tabs[0]
  );
}
