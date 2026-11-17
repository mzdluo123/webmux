import { z } from "zod";

export const MessagingCreateChannelSchema = z.object({
  name: z.string().regex(/^[a-z0-9][a-z0-9_-]{0,63}$/),
  topic: z
    .string()
    .refine((text) => new TextEncoder().encode(text).length <= 1024),
});
export const MessagingChannelSchema = MessagingCreateChannelSchema.extend({
  id: z.string().min(1),
  latestText: z.string().default(""),
  latestAt: z.string().default(""),
  messageCount: z.number().int().nonnegative().default(0),
});
export const MessagingMessageSchema = z.object({
  id: z.string().min(1),
  agentId: z.string(),
  agentName: z.string(),
  text: z.string(),
  createdAt: z.string(),
});
export const MessagingSendSchema = z.object({
  text: z
    .string()
    .min(1)
    .refine(
      (text) =>
        text.trim().length > 0 &&
        new TextEncoder().encode(text).length <= 32000,
    ),
  clientRequestId: z.string().regex(/^[a-zA-Z0-9_-]{8,128}$/),
});
export const MessagingChannelsSchema = z.array(MessagingChannelSchema);
export const MessagingMessagesSchema = z.array(MessagingMessageSchema);
export type MessagingChannel = z.infer<typeof MessagingChannelSchema>;
export type MessagingMessage = z.infer<typeof MessagingMessageSchema>;

export const MessagingViewSchema = z.object({
  channel: z.string().regex(/^[a-z0-9][a-z0-9_-]{0,63}$/),
});
