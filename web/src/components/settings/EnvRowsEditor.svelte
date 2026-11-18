<script lang="ts">
  import { tick, type Snippet } from "svelte";
  import IconEye from "~icons/tabler/eye";
  import IconEyeOff from "~icons/tabler/eye-off";
  import IconPlus from "~icons/tabler/plus";
  import IconTrash from "~icons/tabler/trash";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import type { EnvRow } from "../../domains/settings/launch-configs";

  // The environment variable rows of a form: a name and a value (masked until
  // revealed) per row, with per-row and whole-list errors.
  let {
    rows = $bindable(),
    errors = {},
    error = "",
    max = Infinity,
    disabled = false,
    makeRow,
    help,
  }: {
    rows: EnvRow[];
    /** Problems of single rows, keyed by the row's key. */
    errors?: Readonly<
      Record<number, { name?: string; value?: string } | undefined>
    >;
    /** A problem of the list as a whole. */
    error?: string;
    max?: number;
    disabled?: boolean;
    /** Makes an empty row with a key no other row has. */
    makeRow: () => EnvRow;
    help?: Snippet;
  } = $props();
  let revealed = $state<Record<number, boolean>>({});
  const id = $props.id();
  let addButton = $state<HTMLButtonElement | null>(null);

  async function focusName(key: number | undefined) {
    await tick();
    if (key === undefined) addButton?.focus();
    else document.getElementById(`${id}-name-${key}`)?.focus();
  }

  function addRow() {
    const row = makeRow();
    rows = [...rows, row];
    void focusName(row.key);
  }

  function removeRow(index: number, key: number) {
    delete revealed[key];
    rows = rows.filter((row) => row.key !== key);
    void focusName((rows[index] ?? rows[index - 1])?.key);
  }
</script>

<fieldset class="grid min-w-0 gap-2" {disabled}>
  <legend class="mb-2 text-sm font-medium">
    {m.s2_env_vars()}
    <span class="font-normal text-muted-foreground">{m.s2_optional()}</span>
  </legend>
  <div>
    <Button
      bind:ref={addButton}
      type="button"
      variant="outline"
      size="sm"
      disabled={rows.length >= max}
      onclick={addRow}><IconPlus aria-hidden="true" />{m.s2_env_add()}</Button
    >
  </div>
  {#if rows.length}
    <ScrollArea class="max-h-64" viewportClasses="max-h-64">
      <div class="grid min-w-0 gap-2 p-1">
        {#each rows as envRow, index (envRow.key)}
          {@const rowErrors = errors[envRow.key]}
          {@const rowErrorText = [rowErrors?.name, rowErrors?.value]
            .filter(Boolean)
            .join(m.s2_env_error_sep())}
          {@const errorId = `${id}-error-${envRow.key}`}
          {@const rowIndex = String(index + 1)}
          <div class="grid min-w-0 gap-1">
            <div class="flex min-w-0 flex-wrap items-center gap-2">
              <Input
                id={`${id}-name-${envRow.key}`}
                mono
                class="min-w-0 flex-1 basis-32"
                aria-label={m.s2_env_name_aria({ index: rowIndex })}
                bind:value={envRow.name}
                placeholder="NAME"
                autocomplete="off"
                autocapitalize="off"
                autocorrect="off"
                spellcheck={false}
                aria-invalid={!!rowErrors?.name}
                aria-describedby={rowErrors?.name ? errorId : undefined}
              />
              <div class="flex min-w-0 flex-2 basis-48 items-center gap-1">
                <Input
                  mono
                  class="min-w-0 flex-1"
                  type={revealed[envRow.key] ? "text" : "password"}
                  aria-label={m.s2_env_value_aria({ index: rowIndex })}
                  bind:value={envRow.value}
                  placeholder={m.s2_env_value_placeholder()}
                  autocomplete="off"
                  autocapitalize="off"
                  autocorrect="off"
                  spellcheck={false}
                  aria-invalid={!!rowErrors?.value}
                  aria-describedby={rowErrors?.value ? errorId : undefined}
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label={m.s2_env_reveal_aria({ index: rowIndex })}
                  aria-pressed={!!revealed[envRow.key]}
                  title={revealed[envRow.key]
                    ? m.s2_env_hide_title()
                    : m.s2_env_reveal_title()}
                  onclick={() => {
                    revealed[envRow.key] = !revealed[envRow.key];
                  }}
                  >{#if revealed[envRow.key]}<IconEyeOff
                      aria-hidden="true"
                    />{:else}<IconEye aria-hidden="true" />{/if}</Button
                >
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label={m.s2_env_remove_aria({ index: rowIndex })}
                  title={m.s2_env_remove_title()}
                  onclick={() => removeRow(index, envRow.key)}
                  ><IconTrash aria-hidden="true" /></Button
                >
              </div>
            </div>
            {#if rowErrorText}
              <p id={errorId} class="text-xs wrap-anywhere text-destructive">
                {rowErrorText}
              </p>
            {/if}
          </div>
        {/each}
      </div>
    </ScrollArea>
  {/if}
  {#if error}<p class="text-xs text-destructive">{error}</p>{/if}
  {#if help}
    <p class="text-xs text-muted-foreground">{@render help()}</p>
  {/if}
</fieldset>
