import assert from "node:assert/strict";
import { createHmac } from "node:crypto";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { WebhookVerificationError, verifyWebhook } from "./webhook-receiver.mjs";

type Fixture = {
  name: string;
  secret: string;
  deliveryId: string;
  timestamp: string;
  body: string;
  signature: string;
};

// tsc preserves the source/output depth: contracts/examples -> .contract-client/examples.
const fixtures = JSON.parse(await readFile(new URL("../../contracts/examples/webhook-signature.fixtures.json", import.meta.url), "utf8")) as Fixture[];
const nowSeconds = 1_800_000_000n;

function sign(fixture: Fixture, timestamp: string, body: Uint8Array): string {
  return `v1=${createHmac("sha256", fixture.secret)
    .update(`${fixture.deliveryId}.${timestamp}.`).update(body).digest("hex")}`;
}

function verify(fixture: Fixture, overrides: Partial<Parameters<typeof verifyWebhook>[0]> = {}) {
  return verifyWebhook({
    deliveryId: fixture.deliveryId,
    timestamp: fixture.timestamp,
    signature: fixture.signature,
    rawBody: Buffer.from(fixture.body),
    signingSecrets: [fixture.secret],
    nowSeconds,
    ...overrides,
  });
}

function rejects(code: WebhookVerificationError["code"], operation: () => unknown): void {
  assert.throws(operation, error => error instanceof WebhookVerificationError && error.code === code);
}

test("accepts production signature fixtures and exact raw bodies", () => {
  for (const fixture of fixtures) assert.equal(verify(fixture).id, fixture.deliveryId);
});

test("rejects missing, malformed, stale, future, tampered, and mismatched inputs", () => {
  const fixture = fixtures[0]!;
  rejects("missing_header", () => verify(fixture, { signature: undefined }));
  rejects("invalid_timestamp", () => verify(fixture, { timestamp: "+1800000000" }));
  rejects("invalid_timestamp", () => verify(fixture, { timestamp: "1".repeat(17) }));
  rejects("invalid_signature", () => verify(fixture, { signature: `v2=${fixture.signature.slice(3)}` }));
  assert.equal(verify(fixture, { nowSeconds: nowSeconds + 300n }).id, fixture.deliveryId);
  assert.equal(verify(fixture, { nowSeconds: nowSeconds - 300n }).id, fixture.deliveryId);
  rejects("stale_timestamp", () => verify(fixture, { nowSeconds: nowSeconds + 301n }));
  rejects("stale_timestamp", () => verify(fixture, { nowSeconds: nowSeconds - 301n }));
  rejects("invalid_signature", () => verify(fixture, { rawBody: Buffer.from(`${fixture.body}\n`) }));
  rejects("invalid_signature", () => verify(fixture, { signingSecrets: ["wrong-signing-secret-000000000000"] }));
  rejects("secret_unavailable", () => verify(fixture, { signingSecrets: [] }));
  rejects("secret_unavailable", () => verify(fixture, { signingSecrets: [""] }));
  rejects("secret_unavailable", () => verify(fixture, { signingSecrets: ["x".repeat(31)] }));
  const invalidJson = Buffer.from("not-json");
  rejects("invalid_payload", () => verify(fixture, { rawBody: invalidJson, signature: sign(fixture, fixture.timestamp, invalidJson) }));
  const other = fixtures[1]!;
  const mismatchedBody = Buffer.from(other.body);
  const mismatchedSignature = sign(fixture, fixture.timestamp, mismatchedBody);
  rejects("delivery_id_mismatch", () => verify(fixture, { rawBody: mismatchedBody, signature: mismatchedSignature }));
});

test("accepts old immutable deliveries during a bounded signing-secret overlap", () => {
  const oldDelivery = fixtures[0]!;
  assert.equal(verify(oldDelivery, { signingSecrets: ["new-signing-secret-not-yet-used-0", oldDelivery.secret] }).id,
    oldDelivery.deliveryId);
  rejects("invalid_signature", () => verify(oldDelivery, { signingSecrets: ["new-signing-secret-not-yet-used-0"] }));
});

test("a legitimate retry keeps one delivery identity as its timestamp and signature change", () => {
  const fixture = fixtures[0]!;
  const first = verify(fixture);
  const retryTimestamp = String(nowSeconds + 120n);
  const retrySignature = sign(fixture, retryTimestamp, Buffer.from(fixture.body));
  const retry = verify(fixture, { timestamp: retryTimestamp, signature: retrySignature });
  assert.notEqual(retrySignature, fixture.signature);
  assert.equal(first.id, retry.id);
});
