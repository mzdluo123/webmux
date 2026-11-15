<script lang="ts">
  import { onMount } from "svelte";
  import { z } from "zod";
  import { api } from "../../platform/api";
  import { SessionInfoSchema, type SessionInfo } from "../../contracts/schemas";
  import type { SplitDirection } from "../../domains/terminal/terminal-layout";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as m from "$lib/paraglide/messages.js";
  import SplitDirectionChoice from "./SplitDirectionChoice.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import { errorMessage } from "../../platform/error";

  let {
    excluded,
    onclose,
    onselect,
  }: {
    excluded: string[];
    onclose: () => void;
    onselect: (id: string, direction: SplitDirection) => void;
  } = $props();
  let sessions = $state<SessionInfo[]>([]);
  let direction = $state<SplitDirection>("horizontal");
  let query = $state("");
  let loading = $state(true);
  let error = $state("");
  const choices = $derived(
    sessions.filter(
      (session) =>
        !excluded.includes(session.id) &&
        `${session.title} ${session.cwd}`
          .toLowerCase()
          .includes(query.toLowerCase()),
    ),
  );
  onMount(() => {
    const controller = new AbortController();
    void api(
      "/sessions",
      z.array(SessionInfoSchema),
      "GET",
      undefined,
      controller.signal,
    )
      .then((result) => {
        sessions = result;
      })
      .catch((err) => {
        if (!controller.signal.aborted) error = errorMessage(err);
      })
      .finally(() => {
        loading = false;
      });
    return () => controller.abort();
  });
</script>

<ResponsiveDialog
  title={m.s4_split_join_title()}
  description={m.s4_split_join_desc()}
  {onclose}
>
  <div class="space-y-4">
    <SplitDirectionChoice bind:direction />
    <Input
      aria-label={m.s4_split_search_aria()}
      placeholder={m.s4_split_search_placeholder()}
      bind:value={query}
    />
    <div class="max-h-[50dvh] space-y-1 overflow-auto">
      {#if loading}<p role="status">{m.s4_loading_sessions()}</p>
      {:else if error}<p role="alert" class="text-destructive">{error}</p>
      {:else if choices.length === 0}<p class="text-sm text-muted-foreground">
          {m.s4_split_no_sessions()}
        </p>
      {:else}{#each choices as session (session.id)}
          <ListButton
            variant="option"
            class="w-full"
            onclick={() => onselect(session.id, direction)}
          >
            <span class="min-w-0"
              ><span class="block truncate"
                >{session.title}{session.status === "exited"
                  ? m.s4_session_exited_suffix()
                  : ""}</span
              ><span class="block truncate text-xs text-muted-foreground"
                >{session.cwd}</span
              ></span
            >
          </ListButton>
        {/each}{/if}
    </div>
    <div class="flex justify-end">
      <Button variant="outline" onclick={onclose}>{m.s4_cancel()}</Button>
    </div>
  </div>
</ResponsiveDialog>
