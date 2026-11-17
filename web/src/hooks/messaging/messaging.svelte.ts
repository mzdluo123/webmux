import { m } from "$lib/paraglide/messages.js";
import { onMount } from "svelte";
import { api } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import { stored, store } from "../../platform/storage";
import {
  MessagingChannelsSchema,
  MessagingChannelSchema,
  MessagingMessagesSchema,
  MessagingMessageSchema,
  MessagingSendSchema,
  MessagingCreateChannelSchema,
  MessagingViewSchema,
  type MessagingChannel,
  type MessagingMessage,
} from "../../contracts/messaging-schemas";

function rememberedChannel() {
  try {
    const parsed = MessagingViewSchema.safeParse(
      JSON.parse(stored("messaging-view") ?? "null"),
    );
    return parsed.success ? parsed.data.channel : "";
  } catch {
    return "";
  }
}

export function useMessaging() {
  const remembered = rememberedChannel();
  const state = $state({
    channels: [] as MessagingChannel[],
    messages: [] as MessagingMessage[],
    channel: remembered,
    showConversation: !!remembered,
    query: "",
    draft: "",
    newName: "",
    newTopic: "",
    createOpen: false,
    createError: "",
    loading: false,
    sending: false,
    creating: false,
    hasOlder: false,
    error: "",
    notice: "",
    pendingText: "",
    failed: null as { text: string; error: string } | null,
  });
  let mounted = false;
  let load: AbortController | undefined;
  let mutation: AbortController | undefined;
  const attempts: Record<string, { text: string; id: string } | undefined> =
    Object.create(null);
  const drafts: Record<string, string | undefined> = Object.create(null);
  const positions: Record<
    string,
    { top: number; bottom: boolean } | undefined
  > = Object.create(null);
  const path = (channel: string) =>
    `/messaging/channels/${encodeURIComponent(channel)}/messages`;
  function remember(channel: string) {
    if (channel) store("messaging-view", JSON.stringify({ channel }));
  }

  async function refresh(older = false) {
    load?.abort();
    const controller = new AbortController();
    load = controller;
    state.loading = true;
    state.error = "";
    try {
      const channels = await api(
        "/messaging/channels",
        MessagingChannelsSchema,
        "GET",
        undefined,
        controller.signal,
      );
      if (!mounted || load !== controller) return;
      state.channels = channels;
      if (!channels.some((item) => item.name === state.channel)) {
        drafts[state.channel] = state.draft;
        state.channel = channels[0]?.name ?? "";
        state.showConversation = false;
        state.draft = drafts[state.channel] ?? "";
        state.messages = [];
        older = false;
      }
      const channel = state.channel;
      const before = older ? state.messages[0]?.id : undefined;
      const url =
        path(channel) + (before ? `?before=${encodeURIComponent(before)}` : "");
      const messages = channel
        ? await api(
            url,
            MessagingMessagesSchema,
            "GET",
            undefined,
            controller.signal,
          )
        : [];
      if (!mounted || load !== controller) return;
      state.messages = older ? [...messages, ...state.messages] : messages;
      state.hasOlder = messages.length === 50;
    } catch (cause) {
      if (mounted && load === controller && !controller.signal.aborted)
        state.error = errorMessage(cause);
    } finally {
      if (mounted && load === controller) state.loading = false;
    }
  }

  function select(channel: string) {
    if (state.sending || state.creating) return;
    state.showConversation = true;
    remember(channel);
    if (channel === state.channel) return;
    drafts[state.channel] = state.draft;
    state.channel = channel;
    state.draft = drafts[channel] ?? "";
    state.messages = [];
    state.notice = "";
    state.failed = null;
    state.hasOlder = false;
    void refresh();
  }

  async function createChannel() {
    if (state.creating || state.sending || state.loading) return;
    const parsed = MessagingCreateChannelSchema.safeParse({
      name: state.newName,
      topic: state.newTopic,
    });
    if (!parsed.success) return;
    const controller = new AbortController();
    mutation = controller;
    state.creating = true;
    state.createError = "";
    state.notice = "";
    try {
      const channel = await api(
        "/messaging/channels",
        MessagingChannelSchema,
        "POST",
        parsed.data,
        controller.signal,
      );
      if (!mounted) return;
      drafts[state.channel] = state.draft;
      state.channel = channel.name;
      remember(channel.name);
      state.showConversation = true;
      state.draft = "";
      state.newName = "";
      state.newTopic = "";
      state.messages = [];
      state.failed = null;
      state.createOpen = false;
      await refresh();
    } catch (cause) {
      if (mounted) state.createError = errorMessage(cause);
    } finally {
      if (mounted) state.creating = false;
    }
  }

  async function send(text = state.draft) {
    if (state.sending || state.loading || state.creating || !state.channel)
      return;
    const channel = state.channel;
    let attempt = attempts[channel];
    if (!attempt || attempt.text !== text) {
      attempt = { text, id: crypto.randomUUID() };
      attempts[channel] = attempt;
    }
    const parsed = MessagingSendSchema.safeParse({
      text,
      clientRequestId: attempt.id,
    });
    if (!parsed.success) return;
    const controller = new AbortController();
    mutation = controller;
    state.sending = true;
    state.pendingText = text;
    state.failed = null;
    state.notice = "";
    try {
      const message = await api(
        path(channel),
        MessagingMessageSchema,
        "POST",
        parsed.data,
        controller.signal,
      );
      if (!mounted) return;
      const existed = state.messages.some((item) => item.id === message.id);
      state.messages = [
        ...state.messages.filter((item) => item.id !== message.id),
        message,
      ].sort((a, b) => a.id.length - b.id.length || a.id.localeCompare(b.id));
      state.channels = state.channels.map((item) =>
        item.name === channel
          ? {
              ...item,
              latestText: message.text,
              latestAt: message.createdAt,
              messageCount: item.messageCount + (existed ? 0 : 1),
            }
          : item,
      );
      if (state.draft === text) state.draft = "";
      delete drafts[channel];
      delete attempts[channel];
      state.notice = message.id;
      remember(channel);
    } catch (cause) {
      if (mounted)
        state.failed = {
          text,
          error: `${m.messaging_send_error()} ${errorMessage(cause)}`,
        };
    } finally {
      if (mounted) state.sending = false;
    }
  }

  onMount(() => {
    mounted = true;
    void refresh();
    return () => {
      mounted = false;
      load?.abort();
      mutation?.abort();
    };
  });
  return { state, refresh, select, send, createChannel, positions };
}
export type MessagingController = ReturnType<typeof useMessaging>;
