<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import * as Card from "$lib/components/ui/card";

  const groups = $derived([
    {
      title: m.s2_sc_g_panes(),
      scope: m.s2_sc_g_panes_scope(),
      shortcuts: [
        { action: m.s2_sc_split_h(), keys: ["Ctrl + B", "%"] },
        { action: m.s2_sc_split_v(), keys: ["Ctrl + B", '"'] },
        { action: m.s2_sc_pane_adj(), keys: ["Ctrl + B", "↑ ↓ ← →"] },
        { action: m.s2_sc_pane_next(), keys: ["Ctrl + B", "O"] },
        { action: m.s2_sc_pane_zoom(), keys: ["Ctrl + B", "Z"] },
        { action: m.s2_sc_pane_close(), keys: ["Ctrl + B", "X"] },
        { action: m.s2_sc_send_ctrl_b(), keys: ["Ctrl + B", "Ctrl + B"] },
      ],
    },
    {
      title: m.s2_sc_g_term(),
      scope: m.s2_sc_g_term_scope(),
      shortcuts: [
        { action: m.s2_sc_copy_win(), keys: ["Ctrl + C"] },
        { action: m.s2_sc_copy_mac(), keys: ["⌘ + C"] },
        { action: m.s2_sc_paste(), keys: ["Ctrl / ⌘ + V"] },
        { action: m.s2_sc_paste_entry(), keys: ["Ctrl / ⌘ + Shift + V"] },
        { action: m.s2_sc_move_word(), keys: ["Alt + ← / →"] },
        {
          action: m.s2_sc_scroll_page(),
          keys: ["Shift + PageUp / PageDown"],
        },
      ],
    },
    {
      title: m.s2_sc_g_find(),
      scope: m.s2_sc_g_find_scope(),
      shortcuts: [
        { action: m.s2_sc_find_open(), keys: [m.s2_sc_key_find_open()] },
        { action: m.s2_sc_find_nav(), keys: ["Enter / Shift + Enter"] },
        { action: m.s2_sc_find_close(), keys: ["Esc"] },
      ],
    },
    {
      title: m.s2_code_reader_title(),
      scope: m.s2_sc_g_code_scope(),
      shortcuts: [
        { action: m.s2_sc_code_find(), keys: ["Ctrl / ⌘ + F"] },
        { action: m.s2_sc_code_next(), keys: [m.s2_sc_key_code_next()] },
        {
          action: m.s2_sc_code_prev(),
          keys: [m.s2_sc_key_code_prev()],
        },
        {
          action: m.s2_sc_code_box_nav(),
          keys: ["Enter / Shift + Enter"],
        },
        { action: m.s2_sc_code_goto(), keys: ["Ctrl / ⌘ + Alt + G"] },
        { action: m.s2_sc_code_sel_next(), keys: ["Ctrl / ⌘ + D"] },
        { action: m.s2_sc_code_sel_all(), keys: ["Ctrl / ⌘ + Shift + L"] },
        { action: m.s2_sc_code_close(), keys: ["Esc"] },
        { action: m.s2_sc_code_tab_nav(), keys: ["← / →"] },
        { action: m.s2_sc_code_tab_ends(), keys: ["Home / End"] },
        {
          action: m.s2_sc_code_tree_move(),
          keys: [m.s2_sc_key_up_down_ends()],
        },
        { action: m.s2_sc_code_tree_fold(), keys: ["→ / ←"] },
      ],
    },
    {
      title: m.s2_sc_g_layout(),
      scope: m.s2_sc_g_layout_scope(),
      shortcuts: [
        { action: m.s2_sc_pane_resize_5(), keys: [m.s2_sc_key_arrows()] },
        { action: m.s2_sc_pane_minmax(), keys: ["Home / End"] },
        { action: m.s2_sc_pane_equal(), keys: ["Enter"] },
        { action: m.s2_sc_panel_resize_10(), keys: [m.s2_sc_key_arrows()] },
        {
          action: m.s2_sc_panel_minmax(),
          keys: [m.s2_sc_key_ends_shift_arrows()],
        },
        { action: m.s2_sc_panel_cycle(), keys: ["F6 / Shift + F6"] },
        { action: m.s2_sc_nav_resize_16(), keys: ["← / →"] },
        { action: m.s2_sc_nav_resize_64(), keys: ["Shift + ← / →"] },
        { action: m.s2_sc_nav_minmax(), keys: ["Home / End"] },
      ],
    },
    {
      title: m.s2_sc_g_chat(),
      scope: m.s2_sc_g_chat_scope(),
      shortcuts: [
        { action: m.s2_sc_chat_send(), keys: ["Enter"] },
        { action: m.s2_sc_chat_newline(), keys: ["Shift + Enter"] },
        { action: m.s2_sc_chat_steer(), keys: ["Ctrl / ⌘ + Enter"] },
        { action: m.s2_sc_slash_nav(), keys: ["↑ / ↓"] },
        { action: m.s2_sc_slash_fill(), keys: ["Tab / Esc"] },
      ],
    },
    {
      title: m.s2_sc_g_dialogs(),
      scope: m.s2_sc_g_dialogs_scope(),
      shortcuts: [
        {
          action: m.s2_sc_switch_session(),
          keys: ["⌘ + K / Ctrl + Alt + K"],
        },
        { action: m.s2_sc_focus_search(), keys: ["/"] },
        { action: m.s2_sc_clear_search(), keys: ["Esc"] },
        { action: m.s2_sc_close_modal(), keys: ["Esc"] },
        { action: m.s2_sc_dir_up(), keys: ["Alt + ↑"] },
        { action: m.s2_sc_dir_complete(), keys: ["Tab"] },
        { action: m.s2_sc_perm_digit(), keys: ["1–9"] },
      ],
    },
  ]);
</script>

<Card.Root flat id="shortcuts" class="scroll-mt-4">
  <Card.Header>
    <Card.Title>{m.s2_shortcuts_title()}</Card.Title>
    <Card.Description>{m.s2_shortcuts_desc()}</Card.Description>
  </Card.Header>
  <Card.Content>
    <div>
      {#each groups as group (group.title)}
        <section
          class="space-y-3 border-b py-5 last:border-0"
          aria-label={group.title}
        >
          <div class="space-y-1">
            <h3 class="text-sm font-medium">{group.title}</h3>
            <p class="text-xs text-muted-foreground">{group.scope}</p>
          </div>
          <dl class="space-y-3">
            {#each group.shortcuts as shortcut (shortcut.action)}
              <div class="flex items-start justify-between gap-3">
                <dt class="min-w-0 flex-1 text-sm">{shortcut.action}</dt>
                <dd
                  class="flex max-w-[60%] flex-wrap items-center justify-end gap-1 text-right"
                >
                  {#each shortcut.keys as key, index (index)}
                    {#if index > 0}<span class="text-xs text-muted-foreground"
                        >{m.s2_sc_then()}</span
                      >{/if}
                    <kbd
                      class="rounded-sm border bg-muted px-1.5 py-0.5 font-mono text-xs"
                      >{key}</kbd
                    >
                  {/each}
                </dd>
              </div>
            {/each}
          </dl>
        </section>
      {/each}
    </div>
    <p class="mt-4 text-xs text-muted-foreground">
      {m.s2_sc_footer()}
    </p>
  </Card.Content>
</Card.Root>
