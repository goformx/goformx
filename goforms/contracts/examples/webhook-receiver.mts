import { createHmac, timingSafeEqual } from "node:crypto";

export const deliveryIdHeader = "X-GoFormX-Delivery-ID";
export const timestampHeader = "X-GoFormX-Timestamp";
export const signatureHeader = "X-GoFormX-Signature";
export const defaultToleranceSeconds = 300n;

export type WebhookEvent = {
  id: string;
  type: string;
  createdAt: string;
  submissionId: string;
  formId: string;
  schemaVersion: number;
  data: Record<string, unknown>;
};

export class WebhookVerificationError extends Error {
  constructor(readonly code: "missing_header" | "invalid_timestamp" | "stale_timestamp" |
    "invalid_signature" | "invalid_payload" | "delivery_id_mismatch" | "secret_unavailable") {
    super("Webhook verification failed.");
    this.name = "WebhookVerificationError";
  }
}

export type VerifyWebhookInput = {
  deliveryId: string | undefined;
  timestamp: string | undefined;
  signature: string | undefined;
  rawBody: Uint8Array;
  /** Current secret first, then a previous secret during a bounded rotation overlap. */
  signingSecrets: readonly string[];
  nowSeconds?: bigint;
  toleranceSeconds?: bigint;
};

/** Verify before parsing or acting on the body. Never reconstruct the signed JSON. */
export function verifyWebhook(input: VerifyWebhookInput): WebhookEvent {
  const { deliveryId, timestamp, signature } = input;
  if (!deliveryId || !timestamp || !signature) throw new WebhookVerificationError("missing_header");
  if (!/^(0|[1-9][0-9]{0,15})$/.test(timestamp)) throw new WebhookVerificationError("invalid_timestamp");
  if (!/^v1=[0-9a-f]{64}$/.test(signature)) throw new WebhookVerificationError("invalid_signature");
  if (input.signingSecrets.length === 0 || input.signingSecrets.some(secret => {
    const length = Array.from(secret).length;
    return length < 32 || length > 256;
  })) throw new WebhookVerificationError("secret_unavailable");

  const now = input.nowSeconds ?? BigInt(Math.floor(Date.now() / 1000));
  const tolerance = input.toleranceSeconds ?? defaultToleranceSeconds;
  if (tolerance < 0n) throw new WebhookVerificationError("invalid_timestamp");
  const signedAt = BigInt(timestamp);
  if (signedAt < now - tolerance || signedAt > now + tolerance) {
    throw new WebhookVerificationError("stale_timestamp");
  }

  const supplied = Buffer.from(signature.slice(3), "hex");
  const prefix = Buffer.from(`${deliveryId}.${timestamp}.`, "utf8");
  let matched = 0;
  for (const secret of input.signingSecrets) {
    const expected = createHmac("sha256", secret).update(prefix).update(input.rawBody).digest();
    matched |= Number(timingSafeEqual(expected, supplied));
  }
  if (matched !== 1) throw new WebhookVerificationError("invalid_signature");

  let event: unknown;
  try {
    event = JSON.parse(Buffer.from(input.rawBody).toString("utf8"));
  } catch {
    throw new WebhookVerificationError("invalid_payload");
  }
  if (!event || typeof event !== "object" || Array.isArray(event) ||
      !("id" in event) || typeof event.id !== "string" ||
      !("type" in event) || typeof event.type !== "string" ||
      !("createdAt" in event) || typeof event.createdAt !== "string" ||
      !("submissionId" in event) || typeof event.submissionId !== "string" ||
      !("formId" in event) || typeof event.formId !== "string" ||
      !("schemaVersion" in event) || typeof event.schemaVersion !== "number" ||
        !Number.isInteger(event.schemaVersion) || event.schemaVersion < 1 ||
      !("data" in event) || !event.data || typeof event.data !== "object" || Array.isArray(event.data)) {
    throw new WebhookVerificationError("invalid_payload");
  }
  if (event.id !== deliveryId) throw new WebhookVerificationError("delivery_id_mismatch");
  return event as WebhookEvent;
}
