<script lang="ts">
  import CodePdfView from "../../src/components/code/CodePdfView.svelte";
  import { codeRawPath } from "../../src/domains/code/code-image";
  import type { CodeDocument, CodePdfState } from "../../src/contracts/schemas";
  let {
    document: doc,
    restore,
    hidden = false,
  }: {
    document: CodeDocument;
    restore?: CodePdfState;
    hidden?: boolean;
  } = $props();
  let view = $state<CodePdfView>();
  export function capture() {
    return view!.capture();
  }
</script>

<div
  style="height:650px;width:650px;flex-direction:column"
  style:display={hidden ? "none" : "flex"}
>
  <CodePdfView
    bind:this={view}
    source={codeRawPath("session", "snap-1")}
    document={doc}
    {restore}
    ondownloaderror={() => {}}
  />
</div>
