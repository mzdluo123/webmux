import { z } from "zod";

const AuthStateSchema = z.enum([
  "needs_onboarding",
  "ready",
  "storage_error",
  "disabled",
  "legacy_token",
]);

export const AuthStatusSchema = z.object({
  state: AuthStateSchema,
  rpId: z.string().optional(),
  rpIdValid: z.boolean(),
  rpIdError: z.string().optional(),
  hasPasskeyForRpId: z.boolean(),
  onboardingCompleted: z.boolean(),
  passkeyCount: z.number(),
  sessionCount: z.number(),
  recoveryCodesRemaining: z.number(),
  hostname: z.string().optional(),
  error: z.string().optional(),
});

export const OnboardingVerifyResponseSchema = z.object({
  ticket: z.string(),
  expiresAt: z.string(),
});

export const WebAuthnCreationOptionsSchema = z.object({
  challengeId: z.string(),
  challenge: z.string(),
  rpId: z.string(),
  rpName: z.string(),
  userId: z.string(),
  userName: z.string(),
  displayName: z.string(),
  excludeIds: z.array(z.string()),
  pubKeyAlgs: z.array(z.number()),
  timeoutMs: z.number(),
});

export const WebAuthnAssertionOptionsSchema = z.object({
  challengeId: z.string(),
  challenge: z.string(),
  rpId: z.string(),
  allowIds: z.array(z.string()),
  timeoutMs: z.number(),
});

export const AuthPasskeySchema = z.object({
  id: z.string(),
  rpId: z.string(),
  name: z.string(),
  transports: z.array(z.string()).optional(),
  backupEligible: z.boolean(),
  backupState: z.boolean(),
  createdAt: z.string(),
  lastUsedAt: z.string().optional(),
});

const AuthSessionSchema = z.object({
  id: z.string(),
  name: z.string(),
  rpId: z.string(),
  current: z.boolean(),
  createdAt: z.string(),
  lastUsedAt: z.string(),
  expiresAt: z.string(),
});

export const OnboardingFinishResponseSchema = z.object({
  sessionToken: z.string(),
  session: AuthSessionSchema,
  passkey: AuthPasskeySchema,
  recoveryCodes: z.array(z.string()),
});

export const AuthLoginResponseSchema = z.object({
  sessionToken: z.string(),
  session: AuthSessionSchema,
  passkey: AuthPasskeySchema.optional(),
  recoveryCodesRemaining: z.number(),
});

export const StepUpFinishResponseSchema = z.object({
  stepUpToken: z.string(),
  expiresAt: z.string(),
});

export const PairCreateResponseSchema = z.object({
  code: z.string(),
  displayCode: z.string(),
  pairUrl: z.string().optional(),
  expiresAt: z.string(),
});

export const RecoveryCodesResponseSchema = z.object({
  recoveryCodes: z.array(z.string()),
});

export const PasskeyListResponseSchema = z.object({
  passkeys: z.array(AuthPasskeySchema),
});

export const SessionListResponseSchema = z.object({
  sessions: z.array(AuthSessionSchema),
});

export const RevokeOthersResponseSchema = z.object({
  revoked: z.number(),
});

const AuthAPITokenSchema = z.object({
  id: z.string(),
  name: z.string(),
  createdAt: z.string(),
  lastUsedAt: z.string().optional(),
  expiresAt: z.string().optional(),
});

export const APITokenListResponseSchema = z.object({
  tokens: z.array(AuthAPITokenSchema),
});

export const CreateAPITokenResponseSchema = z.object({
  token: z.string(),
  info: AuthAPITokenSchema,
});

export type AuthStatus = z.infer<typeof AuthStatusSchema>;
export type WebAuthnCreationOptions = z.infer<
  typeof WebAuthnCreationOptionsSchema
>;
export type WebAuthnAssertionOptions = z.infer<
  typeof WebAuthnAssertionOptionsSchema
>;
export type AuthPasskey = z.infer<typeof AuthPasskeySchema>;
export type AuthSession = z.infer<typeof AuthSessionSchema>;
export type OnboardingFinishResponse = z.infer<
  typeof OnboardingFinishResponseSchema
>;
export type AuthLoginResponse = z.infer<typeof AuthLoginResponseSchema>;
export type PairCreateResponse = z.infer<typeof PairCreateResponseSchema>;
export type AuthAPIToken = z.infer<typeof AuthAPITokenSchema>;
export type CreateAPITokenResponse = z.infer<
  typeof CreateAPITokenResponseSchema
>;
