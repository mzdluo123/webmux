import { m } from "$lib/paraglide/messages.js";
import type { CodeWorkspaceTab } from "./code-workspace";

export function fileTabLabels(tabs: CodeWorkspaceTab[]) {
  const groups = new Map<
    string,
    { key: string; parts: string[]; source: string; absolute: string }[]
  >();
  for (const tab of tabs) {
    const doc = tab.file?.document ?? tab.diff?.new;
    const parts = (doc?.path ?? "").split("/");
    const name = parts.at(-1) ?? "";
    const entry = {
      key: tab.key,
      parts,
      source:
        tab.rootKind === "artifacts"
          ? m.s5_artifacts_folder()
          : (tab.diff?.change.group ?? doc?.source ?? ""),
      absolute: doc?.absolutePath ?? "",
    };
    const group = groups.get(name);
    if (group) group.push(entry);
    else groups.set(name, [entry]);
  }
  const result = new Map<string, string>();
  for (const [name, group] of groups) {
    for (const item of group) {
      let label = name;
      for (
        let depth = 2;
        depth <= item.parts.length &&
        group.some(
          (other) =>
            other !== item && other.parts.slice(1 - depth).join("/") === label,
        );
        depth++
      )
        label = item.parts.slice(-depth).join("/");
      if (
        group.some(
          (other) =>
            other !== item &&
            other.parts.slice(-label.split("/").length).join("/") === label,
        )
      ) {
        label = group.some(
          (other) =>
            other !== item &&
            other.parts.join("/") === item.parts.join("/") &&
            other.source === item.source,
        )
          ? item.absolute
          : `${label} · ${item.source}`;
      }
      result.set(item.key, label);
    }
  }
  return result;
}
