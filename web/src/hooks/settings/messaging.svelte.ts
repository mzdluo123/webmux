import { m } from "$lib/paraglide/messages.js";
import { onMount } from "svelte";
import { api } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import {
  MessagingChannelsSchema,
  MessagingChannelSchema,
  MessagingMessagesSchema,
  MessagingMessageSchema,
  MessagingSendSchema,
  MessagingCreateChannelSchema,
  type MessagingChannel,
  type MessagingMessage,
} from "../../contracts/messaging-schemas";

export function useMessaging() {
  const state = $state({
    channels: [] as MessagingChannel[],
    messages: [] as MessagingMessage[],
    channel: "",
    draft: "",
    newName: "",
    newTopic: "",
    loading: false,
    sending: false,
    creating: false,
    hasOlder: false,
    error: "",
    notice: "",
  });
  let mounted = false;
  let load: AbortController | undefined;
  let mutation: AbortController | undefined;
  const attempts: Record<string, { text: string; id: string } | undefined> =
    Object.create(null);
  const drafts: Record<string, string | undefined> = Object.create(null);
  const path = (channel: string) =>
    `/messaging/channels/${encodeURIComponent(channel)}/messages`;

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
    if (state.sending || state.creating || channel === state.channel) return;
    drafts[state.channel] = state.draft;
    state.channel = channel;
    state.draft = drafts[channel] ?? "";
    state.messages = [];
    state.notice = "";
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
    state.error = "";
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
      state.draft = "";
      state.newName = "";
      state.newTopic = "";
      state.messages = [];
      await refresh();
    } catch (cause) {
      if (mounted) state.error = errorMessage(cause);
    } finally {
      if (mounted) state.creating = false;
    }
  }

  async function send() {
    if (state.sending || state.loading || state.creating || !state.channel)
      return;
    const channel = state.channel;
    const draft = state.draft;
    let attempt = attempts[channel];
    if (!attempt || attempt.text !== draft) {
      attempt = { text: draft, id: crypto.randomUUID() };
      attempts[channel] = attempt;
    }
    const parsed = MessagingSendSchema.safeParse({
      text: draft,
      clientRequestId: attempt.id,
    });
    if (!parsed.success) return;
    const controller = new AbortController();
    mutation = controller;
    state.sending = true;
    state.error = "";
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
      state.messages = [
        ...state.messages.filter((item) => item.id !== message.id),
        message,
      ].sort((a, b) => a.id.length - b.id.length || a.id.localeCompare(b.id));
      if (state.draft === draft) state.draft = "";
      delete drafts[channel];
      delete attempts[channel];
      state.notice = message.id;
    } catch (cause) {
      if (mounted)
        state.error = `${m.messaging_send_error()} ${errorMessage(cause)}`;
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
  return { state, refresh, select, send, createChannel };
}
