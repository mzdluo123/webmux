<script lang="ts">
  import type { Snippet } from "svelte";
  import IconFile from "~icons/tabler/file";
  import IconDiff from "~icons/tabler/git-compare";
  import { m } from "$lib/paraglide/messages.js";
  import DockPanel from "../shared/DockPanel.svelte";
  import type { CodeWorkspaceTab } from "../../domains/code/code-workspace";

  let {
    panel,
    title,
    doc,
    onclose,
    onactivate,
    header,
    commands,
    children,
  }: {
    panel: string;
    title: string;
    doc: CodeWorkspaceTab;
    onclose: () => void;
    onactivate: () => void;
    header: Snippet;
    commands: Snippet;
    children: Snippet;
  } = $props();
  const file = $derived(doc.file?.document ?? doc.diff?.new);
  const source = $derived(
    doc.rootKind === "artifacts"
      ? m.s5_artifacts_folder()
      : doc.rootKind === "filesystem"
        ? m.s5_external_file()
        : (doc.diff?.change.group ?? file?.source),
  );
</script>

<DockPanel
  id={panel}
  {title}
  {onclose}
  {onactivate}
  {commands}
  {header}
  icon={doc.diff ? IconDiff : IconFile}
  tabLabel={`${file?.path} · ${source}${doc.status !== "unchanged" ? " ●" : ""}`}
  tooltip={file?.absolutePath}
>
  {@render children()}
</DockPanel>
