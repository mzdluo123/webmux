<script lang="ts">
  import IconPlus from "~icons/tabler/plus";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import { Textarea } from "$lib/components/ui/textarea";
  import { locale } from "$lib/locale";
  import { m } from "$lib/paraglide/messages.js";
  import {
    scratchEntriesLimit,
    scratchEntryLimit,
    scratchStorageLimit,
  } from "../../domains/terminal/draft-storage";

  let {
    editor = $bindable(null),
    text = $bindable(),
    full,
    onadd,
  }: {
    editor: HTMLTextAreaElement | null;
    text: string;
    full: boolean;
    onadd: () => void;
  } = $props();
</script>

<div
  class="grid shrink-0 gap-2 border-t bg-muted/30 px-4 py-2 @max-[12rem]/scratch:gap-1 @max-[12rem]/scratch:px-2 @max-[12rem]/scratch:py-1 sm:px-6 sm:py-3"
>
  <Label for="scratch-new">{m.s4_scratch_new_label()}</Label>
  <Textarea
    id="scratch-new"
    bind:ref={editor}
    bind:value={text}
    rows={2}
    maxlength={scratchEntryLimit}
    placeholder={m.s4_scratch_new_placeholder()}
    code
    class="h-20 max-h-[16dvh] min-h-0 resize-none field-sizing-fixed @max-[12rem]/scratch:max-h-[12dvh]"
    aria-describedby="scratch-new-limits scratch-manual-send"
  />
  <div
    class="flex flex-wrap items-center justify-between gap-3 @max-[12rem]/scratch:gap-1"
  >
    <div class="min-w-0 text-xs text-muted-foreground">
      <p id="scratch-new-limits" class="tabular-nums">
        {m.s4_scratch_char_ratio({
          current: text.length.toLocaleString(locale.current),
          limit: scratchEntryLimit.toLocaleString(locale.current),
        })}<span class="sr-only"
          >{m.s4_scratch_limits_sr({
            entries: String(scratchEntriesLimit),
            total: scratchStorageLimit.toLocaleString(locale.current),
          })}</span
        >
      </p>
    </div>
    <Button
      size="sm-touch"
      disabled={!text || full}
      aria-describedby={full ? "scratch-full-state" : undefined}
      onclick={onadd}
      ><IconPlus aria-hidden="true" />{m.s4_scratch_add_button()}</Button
    >
  </div>
</div>
