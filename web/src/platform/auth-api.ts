import { z } from "zod";
import {
  APITokenListResponseSchema,
  AuthLoginResponseSchema,
  AuthPasskeySchema,
  AuthStatusSchema,
  CreateAPITokenResponseSchema,
  OnboardingFinishResponseSchema,
  OnboardingVerifyResponseSchema,
  PairCreateResponseSchema,
  PasskeyListResponseSchema,
  RecoveryCodesResponseSchema,
  RevokeOthersResponseSchema,
  SessionListResponseSchema,
  StepUpFinishResponseSchema,
  WebAuthnAssertionOptionsSchema,
  WebAuthnCreationOptionsSchema,
  type AuthLoginResponse,
  type AuthPasskey,
  type AuthStatus,
  type CreateAPITokenResponse,
  type OnboardingFinishResponse,
  type PairCreateResponse,
} from "../contracts/auth-schemas";
import { ErrorSchema } from "../contracts/schemas";
import { APIError, store } from "./api";
import { authenticationHeaders } from "./relay-auth";
import { currentTransport } from "./transport";
import {
  createPasskey,
  defaultDeviceLabel,
  getPasskeyAssertion,
  type PasskeyRegistrationPayload,
} from "./webauthn";

// Consumes `#setup=` or `#code=` from the URL hash and immediately removes it
// from the address bar and history without sending it to the server.
export function consumeAuthFragment(prefix: "#setup=" | "#code="): string {
  const hash = location.hash;
  if (!hash.startsWith(prefix)) return "";
  history.replaceState(history.state, "", location.pathname + location.search);
  try {
    return decodeURIComponent(hash.slice(prefix.length)).trim();
  } catch {
    return "";
  }
}

async function authRequest<T extends z.ZodType>(
  path: string,
  schema: T,
  options: {
    method?: string;
    body?: unknown;
    authenticated?: boolean;
    stepUpToken?: string;
    signal?: AbortSignal;
  } = {},
): Promise<z.output<T>> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (options.authenticated) {
    Object.assign(headers, authenticationHeaders());
  }
  if (options.stepUpToken) {
    headers["X-WebMux-Step-Up"] = options.stepUpToken;
  }
  const response = await currentTransport().fetch(path, {
    method: options.method ?? "GET",
    signal: options.signal,
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });
  if (!response.ok) {
    const raw = await response.json().catch(() => null);
    const parsed = ErrorSchema.safeParse(raw);
    if (options.authenticated && response.status === 401) {
      window.dispatchEvent(new Event("webmux:auth"));
    }
    throw parsed.success
      ? new APIError(
          response.status,
          parsed.data.error,
          parsed.data.reason,
          parsed.data.notPersisted,
          raw,
        )
      : new APIError(response.status, response.statusText);
  }
  return schema.parse(
    response.status === 204 ? undefined : await response.json(),
  );
}

const VoidSchema = z.undefined();

export function fetchAuthStatus(signal?: AbortSignal): Promise<AuthStatus> {
  return authRequest("/auth/status", AuthStatusSchema, { signal });
}

export function verifyOnboardingCode(code: string) {
  return authRequest(
    "/auth/onboarding/verify",
    OnboardingVerifyResponseSchema,
    {
      method: "POST",
      body: { code },
    },
  );
}

function passkeyAttestationBody(
  challengeId: string,
  name: string,
  attested: PasskeyRegistrationPayload,
  extra: Record<string, string> = {},
) {
  return {
    ...extra,
    challengeId,
    name,
    clientDataJson: attested.clientDataJson,
    attestationObject: attested.attestationObject,
    transports: attested.transports,
  };
}

export async function registerFirstPasskey(
  ticket: string,
  passkeyName: string,
): Promise<OnboardingFinishResponse> {
  const creationOptions = await authRequest(
    "/auth/onboarding/passkey/begin",
    WebAuthnCreationOptionsSchema,
    { method: "POST", body: { ticket } },
  );
  const attested = await createPasskey(creationOptions);
  const result = await authRequest(
    "/auth/onboarding/passkey/finish",
    OnboardingFinishResponseSchema,
    {
      method: "POST",
      body: passkeyAttestationBody(
        creationOptions.challengeId,
        passkeyName,
        attested,
        { ticket, deviceName: defaultDeviceLabel() },
      ),
    },
  );
  store("token", result.sessionToken);
  return result;
}

export function completeOnboarding(): Promise<void> {
  return authRequest("/auth/onboarding/complete", VoidSchema, {
    method: "POST",
    authenticated: true,
  });
}

export async function loginWithPasskey(): Promise<AuthLoginResponse> {
  const assertionOptions = await authRequest(
    "/auth/login/begin",
    WebAuthnAssertionOptionsSchema,
    { method: "POST" },
  );
  const assertion = await getPasskeyAssertion(assertionOptions);
  const result = await authRequest(
    "/auth/login/finish",
    AuthLoginResponseSchema,
    {
      method: "POST",
      body: {
        challengeId: assertionOptions.challengeId,
        credentialId: assertion.credentialId,
        clientDataJson: assertion.clientDataJson,
        authenticatorData: assertion.authenticatorData,
        signature: assertion.signature,
        deviceName: defaultDeviceLabel(),
      },
    },
  );
  store("token", result.sessionToken);
  return result;
}

export async function pairDeviceWithPasskey(
  code: string,
  passkeyName: string,
): Promise<AuthLoginResponse> {
  const creationOptions = await authRequest(
    "/auth/pair/begin",
    WebAuthnCreationOptionsSchema,
    { method: "POST", body: { code } },
  );
  const attested = await createPasskey(creationOptions);
  const result = await authRequest(
    "/auth/pair/finish",
    AuthLoginResponseSchema,
    {
      method: "POST",
      body: passkeyAttestationBody(
        creationOptions.challengeId,
        passkeyName,
        attested,
        { deviceName: defaultDeviceLabel() },
      ),
    },
  );
  store("token", result.sessionToken);
  return result;
}

export async function loginWithRecoveryCode(
  code: string,
): Promise<AuthLoginResponse> {
  const result = await authRequest("/auth/recovery", AuthLoginResponseSchema, {
    method: "POST",
    body: { code, deviceName: defaultDeviceLabel() },
  });
  store("token", result.sessionToken);
  return result;
}

let cachedStepUpToken = "";
let cachedStepUpExpires = 0;

export function clearCachedStepUp() {
  cachedStepUpToken = "";
  cachedStepUpExpires = 0;
}

async function ensureStepUpToken(): Promise<string> {
  if (cachedStepUpToken && Date.now() + 15_000 < cachedStepUpExpires) {
    return cachedStepUpToken;
  }
  const assertionOptions = await authRequest(
    "/auth/step-up/begin",
    WebAuthnAssertionOptionsSchema,
    { method: "POST", authenticated: true },
  );
  const assertion = await getPasskeyAssertion(assertionOptions);
  const finished = await authRequest(
    "/auth/step-up/finish",
    StepUpFinishResponseSchema,
    {
      method: "POST",
      authenticated: true,
      body: {
        challengeId: assertionOptions.challengeId,
        credentialId: assertion.credentialId,
        clientDataJson: assertion.clientDataJson,
        authenticatorData: assertion.authenticatorData,
        signature: assertion.signature,
      },
    },
  );
  cachedStepUpToken = finished.stepUpToken;
  cachedStepUpExpires = Date.parse(finished.expiresAt);
  return cachedStepUpToken;
}

export async function logoutCurrentSession(): Promise<void> {
  try {
    await authRequest("/auth/logout", VoidSchema, {
      method: "POST",
      authenticated: true,
    });
  } finally {
    clearCachedStepUp();
    store("token", null);
    store("connected", null);
    window.dispatchEvent(new Event("webmux:auth"));
  }
}

export function listPasskeys() {
  return authRequest("/auth/passkeys", PasskeyListResponseSchema, {
    authenticated: true,
  });
}

export async function addPasskey(name: string): Promise<AuthPasskey> {
  const stepUpToken = await ensureStepUpToken();
  const creationOptions = await authRequest(
    "/auth/passkeys/begin",
    WebAuthnCreationOptionsSchema,
    { method: "POST", authenticated: true, stepUpToken },
  );
  const attested = await createPasskey(creationOptions);
  return authRequest("/auth/passkeys/finish", AuthPasskeySchema, {
    method: "POST",
    authenticated: true,
    stepUpToken,
    body: passkeyAttestationBody(creationOptions.challengeId, name, attested),
  });
}

export async function deletePasskey(id: string): Promise<void> {
  const stepUpToken = await ensureStepUpToken();
  await authRequest(`/auth/passkeys/${encodeURIComponent(id)}`, VoidSchema, {
    method: "DELETE",
    authenticated: true,
    stepUpToken,
  });
}

export async function createPairingCode(
  targetUrl = "",
): Promise<PairCreateResponse> {
  const stepUpToken = await ensureStepUpToken();
  return authRequest("/auth/pair/create", PairCreateResponseSchema, {
    method: "POST",
    authenticated: true,
    stepUpToken,
    body: targetUrl ? { targetUrl } : {},
  });
}

export function listAuthSessions() {
  return authRequest("/auth/sessions", SessionListResponseSchema, {
    authenticated: true,
  });
}

export function revokeAuthSession(id: string) {
  return authRequest(`/auth/sessions/${encodeURIComponent(id)}`, VoidSchema, {
    method: "DELETE",
    authenticated: true,
  });
}

export function revokeOtherAuthSessions() {
  return authRequest(
    "/auth/sessions/revoke-others",
    RevokeOthersResponseSchema,
    { method: "POST", authenticated: true },
  );
}

export function listAPITokens() {
  return authRequest("/auth/tokens", APITokenListResponseSchema, {
    authenticated: true,
  });
}

export async function createAPIToken(
  name: string,
  expiresDays: number,
): Promise<CreateAPITokenResponse> {
  const stepUpToken = await ensureStepUpToken();
  return authRequest("/auth/tokens", CreateAPITokenResponseSchema, {
    method: "POST",
    authenticated: true,
    stepUpToken,
    body: { name, expiresDays },
  });
}

export function revokeAPIToken(id: string) {
  return authRequest(`/auth/tokens/${encodeURIComponent(id)}`, VoidSchema, {
    method: "DELETE",
    authenticated: true,
  });
}

export async function regenerateRecoveryCodes(): Promise<string[]> {
  const stepUpToken = await ensureStepUpToken();
  const res = await authRequest(
    "/auth/recovery-codes/regenerate",
    RecoveryCodesResponseSchema,
    { method: "POST", authenticated: true, stepUpToken },
  );
  return res.recoveryCodes;
}
