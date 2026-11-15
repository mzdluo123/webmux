<script lang="ts">
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import IconX from "~icons/tabler/x";
  import { z } from "zod";
  import { Button } from "$lib/components/ui/button";
  import * as Command from "$lib/components/ui/command";
  import * as Dialog from "$lib/components/ui/dialog";
  import { api } from "../../platform/api";
  import { errorMessage } from "../../platform/error";
  import { pluginContributions } from "../../plugins/registry.svelte";
  import type { Repositories } from "../../hooks/workspace/repositories.svelte";
  import type { SessionList } from "../../hooks/sessions/session-list.svelte";
  import {
    SessionInfoSchema,
    type ChatMessageMatch,
    type Repository,
    type SessionInfo,
  } from "../../contracts/schemas";
  import { sessionPins } from "../../domains/sessions/session-pins.svelte";
  import SessionStatus from "../sessions/SessionStatus.svelte";
  import { m } from "$lib/paraglide/messages.js";

  let {
    open = $bindable(false),
    source,
    onselect,
    oncancel,
  }: {
    open: boolean;
    source?: { sessions: SessionList; repositories?: Repositories };
    onselect: (id: string, messageId?: string) => void;
    oncancel: () => void;
  } = $props();

  let input = $state<HTMLInputElement | null>(null);
  let snapshot = $state.raw<SessionInfo[] | null>(null);
  let repositories = $state.raw<Repository[]>([]);
  let loadingRepositories = $state(false);
  let error = $state("");
  let selected: { id: string; messageId?: string } | undefined;
  let query = $state("");
  let matches = $state.raw<ChatMessageMatch[]>([]);
  let searching = $state(false);
  let searchFailed = $state(false);
  const term = $derived(query.trim());
  $effect(() => {
    searching = false;
    searchFailed = false;
    const chat = pluginContributions.entries.chat;
    if (!chat || !open || [...term].length < 2 || [...term].length > 120) {
      matches = [];
      return;
    }
    const controller = new AbortController();
    const value = term;
    searching = true;
    const timer = setTimeout(() => {
      void chat
        .search(value, controller.signal)
        .then((items) => {
          if (!controller.signal.aborted) matches = items;
        })
        .catch(() => {
          if (!controller.signal.aborted) {
            matches = [];
            searchFailed = true;
          }
        })
        .finally(() => {
          if (!controller.signal.aborted) searching = false;
        });
    }, 200);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  });
  const sessions = $derived(source?.sessions.sessions ?? snapshot);
  const loading = $derived(source?.sessions.loading ?? snapshot === null);
  const loadError = $derived(source?.sessions.error ?? error);
  const loadingMetadata = $derived(
    source?.repositories ? !source.repositories.settled : loadingRepositories,
  );
  const metadata = $derived(source?.repositories?.list ?? repositories);
  const repositoryById = $derived(
    new Map(metadata.map((repository) => [repository.id, repository])),
  );
  const chatMatches = $derived(
    matches.flatMap((match) => {
      const session = sessions?.find((item) => item.id === match.sessionId);
      return session?.view === "chat" ? [{ match, session }] : [];
    }),
  );
  const pinned = $derived(sessionPins.ordered(sessions ?? []));
  const resumable = $derived(
    sessions?.filter(
      (session) => session.status !== "exited" && !sessionPins.has(session.id),
    ) ?? [],
  );
  const exited = $derived(
    sessions?.filter((session) => session.status === "exited") ?? [],
  );

  function scoreItem(value: string, search: string): number {
    const separator = value.indexOf(":");
    const kind = separator >= 0 ? value.slice(0, separator) : "";
    if (kind === "message") return 2;
    const text = separator >= 0 ? value.slice(separator + 1) : value;
    if (!text.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase())) {
      return 0;
    }
    if (kind === "pinned") return 4;
    if (kind === "resumable") return 3;
    return 1;
  }

  $effect(() => {
    snapshot = null;
    repositories = [];
    loadingRepositories = false;
    error = "";
    if (!open || source) return;
    const controller = new AbortController();
    const { signal } = controller;
    void api("/sessions", z.array(SessionInfoSchema), "GET", undefined, signal)
      .then((result) => {
        if (signal.aborted) return;
        snapshot = result;
        const workspace = pluginContributions.entries.workspace;
        if (workspace && result.some((session) => session.workspace)) {
          // A slow or unavailable Git registry should not block session access.
          loadingRepositories = true;
          void workspace
            .loadRepositories(signal)
            .then((result) => {
              if (!signal.aborted) repositories = result;
            })
            .catch(() => {})
            .finally(() => {
              if (!signal.aborted) loadingRepositories = false;
            });
        }
      })
      .catch((err: unknown) => {
        if (!signal.aborted) error = errorMessage(err);
      });
    return () => controller.abort();
  });
</script>

{#snippet sessionOption(
  session: SessionInfo,
  kind: "pinned" | "resumable" | "exited",
)}
  {@const repository = repositoryById.get(
    session.workspace?.repositoryId ?? "",
  )}
  {@const branch = repository?.worktrees.find(
    (tree) => tree.id === session.workspace?.worktreeId,
  )?.branch}
  {@const detail = [repository?.name, branch, session.cwd]
    .filter(Boolean)
    .join(" · ")}
  <Command.Item
    value={`${kind}:${session.title} ${detail} ${session.id}`}
    onSelect={() => {
      selected = { id: session.id };
      open = false;
    }}
  >
    <IconTerminal2 class="size-4 text-muted-foreground" aria-hidden="true" />
    <span class="min-w-0 flex-1">
      <span class="block truncate text-sm font-medium" title={session.title}>
        {session.title}
      </span>
      <span class="block truncate text-xs text-muted-foreground" title={detail}>
        {detail}
      </span>
    </span>
    <SessionStatus {session} />
  </Command.Item>
{/snippet}

<Dialog.Root bind:open>
  <Dialog.Content
    class="flex flex-col sm:max-w-lg"
    showCloseButton={false}
    onOpenAutoFocus={(event) => {
      event.preventDefault();
      input?.focus();
    }}
    onCloseAutoFocus={(event) => {
      event.preventDefault();
      const target = selected;
      selected = undefined;
      query = "";
      if (target) onselect(target.id, target.messageId);
      else oncancel();
    }}
  >
    <Dialog.Header class="shrink-0 pr-8 text-left">
      <Dialog.Title>{m.s1_quick_switch_session()}</Dialog.Title>
      <Dialog.Description>{m.s1_switcher_description()}</Dialog.Description>
    </Dialog.Header>
    <Dialog.Close>
      {#snippet child({ props })}
        <Button
          {...props}
          variant="ghost"
          size="icon-sm"
          class="absolute top-2 right-2"
          aria-label={m.s1_close_session_switcher()}
          title={m.s1_close_session_switcher()}
        >
          <IconX class="size-4" aria-hidden="true" />
        </Button>
      {/snippet}
    </Dialog.Close>
    <Command.Root
      label={m.s1_search_all_sessions()}
      class="min-h-0"
      filter={scoreItem}
    >
      <Command.Input
        bind:ref={input}
        bind:value={query}
        placeholder={m.s1_switcher_placeholder()}
        autocomplete="off"
        autocapitalize="off"
        spellcheck={false}
      />
      <Command.List
        aria-label={m.s1_matching_sessions()}
        class="max-h-[min(25rem,50dvh)] min-h-0 overscroll-contain"
      >
        {#if sessions?.length}
          {#if !chatMatches.length}
            <Command.Empty>
              {searching
                ? m.s1_searching_chat_messages()
                : searchFailed
                  ? m.s1_search_chat_failed()
                  : loadingMetadata
                    ? m.s1_loading_repos_and_branches()
                    : m.s1_no_matching_sessions()}
            </Command.Empty>
          {/if}
          {#if pinned.length}
            <Command.Group heading={m.s1_group_pinned()} value="pinned">
              {#each pinned as session (session.id)}
                {@render sessionOption(session, "pinned")}
              {/each}
            </Command.Group>
          {/if}
          {#if resumable.length}
            <Command.Group heading={m.s1_group_resumable()} value="resumable">
              {#each resumable as session (session.id)}
                {@render sessionOption(session, "resumable")}
              {/each}
            </Command.Group>
          {/if}
          {#if chatMatches.length}
            <Command.Group
              heading={m.s1_group_found_in_messages()}
              value="messages"
            >
              {#each chatMatches as { match, session } (`${session.id}:${match.role}:${match.messageId}`)}
                <Command.Item
                  value={`message:${session.id}:${match.role}:${match.messageId}`}
                  onSelect={() => {
                    selected = { id: session.id, messageId: match.messageId };
                    open = false;
                  }}
                >
                  <span class="min-w-0 flex-1">
                    <span class="block truncate font-medium"
                      >{session.title}</span
                    >
                    <span
                      class="block truncate text-muted-foreground"
                      title={match.excerpt}
                    >
                      {match.role === "user"
                        ? m.s1_role_you()
                        : m.s1_role_agent()} · {match.excerpt}
                    </span>
                  </span>
                </Command.Item>
              {/each}
            </Command.Group>
          {/if}
          {#if exited.length}
            <Command.Group heading={m.s1_group_exited()} value="exited">
              {#each exited as session (session.id)}
                {@render sessionOption(session, "exited")}
              {/each}
            </Command.Group>
          {/if}
        {/if}
      </Command.List>
      {#if loadError && !sessions?.length}
        <p role="alert" class="py-3 text-sm text-destructive">
          {m.s1_load_sessions_failed({ error: loadError })}
        </p>
      {:else if loading}
        <p role="status" class="py-3 text-sm text-muted-foreground">
          {m.s1_loading_sessions()}
        </p>
      {:else if !sessions?.length}
        <p role="status" class="py-3 text-sm text-muted-foreground">
          {m.s1_no_sessions_yet()}
        </p>
      {/if}
    </Command.Root>
    <div
      class="hidden shrink-0 items-center gap-4 text-xs text-muted-foreground sm:flex"
    >
      <span><kbd>↑ ↓</kbd> {m.s1_kbd_select()}</span>
      <span><kbd>Enter</kbd> {m.s1_kbd_open()}</span>
      <span class="ml-auto"><kbd>Esc</kbd> {m.s1_close()}</span>
    </div>
  </Dialog.Content>
</Dialog.Root>
