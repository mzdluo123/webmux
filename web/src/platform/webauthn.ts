import type {
  WebAuthnAssertionOptions,
  WebAuthnCreationOptions,
} from "../contracts/auth-schemas";
import { decodeBase64Url, encodeBase64UrlBytes } from "./base64url";

export interface PasskeyRegistrationPayload {
  clientDataJson: string;
  attestationObject: string;
  transports: string[];
}

export interface PasskeyAssertionPayload {
  credentialId: string;
  clientDataJson: string;
  authenticatorData: string;
  signature: string;
}

export function isWebAuthnSupported(): boolean {
  return (
    typeof window !== "undefined" &&
    window.isSecureContext &&
    typeof PublicKeyCredential !== "undefined" &&
    typeof navigator !== "undefined" &&
    typeof navigator.credentials?.create === "function" &&
    typeof navigator.credentials?.get === "function"
  );
}

function isLoopbackIPHost(hostname: string): boolean {
  const h = hostname.toLowerCase();
  return h === "127.0.0.1" || h === "::1" || h === "[::1]";
}

export function localhostRedirectUrl(loc: Location = location): string | null {
  if (!isLoopbackIPHost(loc.hostname)) return null;
  const port = loc.port ? `:${loc.port}` : "";
  return `${loc.protocol}//localhost${port}${loc.pathname}${loc.search}${loc.hash}`;
}

export function defaultDeviceLabel(): string {
  if (typeof navigator === "undefined") return "Browser";
  const ua = navigator.userAgent;
  let browser = "Browser";
  if (ua.includes("Edg/")) browser = "Edge";
  else if (ua.includes("Chrome/") && !ua.includes("Chromium"))
    browser = "Chrome";
  else if (ua.includes("Firefox/")) browser = "Firefox";
  else if (ua.includes("Safari/") && !ua.includes("Chrome/"))
    browser = "Safari";

  let platform = "";
  if (ua.includes("iPhone")) platform = "iPhone";
  else if (ua.includes("iPad")) platform = "iPad";
  else if (ua.includes("Android")) platform = "Android";
  else if (ua.includes("Macintosh") || ua.includes("Mac OS"))
    platform = "macOS";
  else if (ua.includes("Windows")) platform = "Windows";
  else if (ua.includes("Linux")) platform = "Linux";

  return platform ? `${browser} on ${platform}` : browser;
}

export async function createPasskey(
  options: WebAuthnCreationOptions,
): Promise<PasskeyRegistrationPayload> {
  if (!isWebAuthnSupported()) {
    throw new Error(
      "WebAuthn is not available in this browser or context (requires localhost or HTTPS).",
    );
  }
  const credential = (await navigator.credentials.create({
    publicKey: {
      challenge: decodeBase64Url(options.challenge),
      rp: { id: options.rpId, name: options.rpName },
      user: {
        id: decodeBase64Url(options.userId),
        name: options.userName,
        displayName: options.displayName,
      },
      pubKeyCredParams: options.pubKeyAlgs.map((alg) => ({
        type: "public-key" as const,
        alg,
      })),
      excludeCredentials: options.excludeIds.map((id) => ({
        type: "public-key" as const,
        id: decodeBase64Url(id),
      })),
      authenticatorSelection: {
        residentKey: "required",
        userVerification: "preferred",
      },
      attestation: "none",
      timeout: options.timeoutMs,
    },
  })) as PublicKeyCredential | null;

  if (!credential) {
    throw new Error("Passkey creation was cancelled.");
  }
  const response = credential.response as AuthenticatorAttestationResponse;
  const transports =
    typeof response.getTransports === "function"
      ? response.getTransports()
      : [];
  return {
    clientDataJson: encodeBase64UrlBytes(response.clientDataJSON),
    attestationObject: encodeBase64UrlBytes(response.attestationObject),
    transports,
  };
}

export async function getPasskeyAssertion(
  options: WebAuthnAssertionOptions,
): Promise<PasskeyAssertionPayload> {
  if (!isWebAuthnSupported()) {
    throw new Error(
      "WebAuthn is not available in this browser or context (requires localhost or HTTPS).",
    );
  }
  const credential = (await navigator.credentials.get({
    publicKey: {
      challenge: decodeBase64Url(options.challenge),
      rpId: options.rpId,
      allowCredentials: options.allowIds.map((id) => ({
        type: "public-key" as const,
        id: decodeBase64Url(id),
      })),
      userVerification: "preferred",
      timeout: options.timeoutMs,
    },
  })) as PublicKeyCredential | null;

  if (!credential) {
    throw new Error("Passkey sign-in was cancelled.");
  }
  const response = credential.response as AuthenticatorAssertionResponse;
  return {
    credentialId: encodeBase64UrlBytes(credential.rawId),
    clientDataJson: encodeBase64UrlBytes(response.clientDataJSON),
    authenticatorData: encodeBase64UrlBytes(response.authenticatorData),
    signature: encodeBase64UrlBytes(response.signature),
  };
}
