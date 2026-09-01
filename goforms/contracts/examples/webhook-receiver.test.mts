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

const fixtures = JSON.parse(await readFile(new URL("../../contracts/examples/webhook-signature.fixtures.json", import.meta.url), "utf8")) as Fixture[];
const nowSeconds = 1_800_000_000n;

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
  rejects("stale_timestamp", () => verify(fixture, { nowSeconds: nowSeconds + 301n }));
  rejects("stale_timestamp", () => verify(fixture, { nowSeconds: nowSeconds - 301n }));
  rejects("invalid_signature", () => verify(fixture, { rawBody: Buffer.from(`${fixture.body}\n`) }));
  rejects("invalid_signature", () => verify(fixture, { signingSecrets: ["wrong-signing-secret"] }));
  const other = fixtures[1]!;
  const mismatchedBody = Buffer.from(other.body);
  const mismatchedSignature = `v1=${createHmac("sha256", fixture.secret)
    .update(`${fixture.deliveryId}.${fixture.timestamp}.`).update(mismatchedBody).digest("hex")}`;
  rejects("delivery_id_mismatch", () => verify(fixture, { rawBody: mismatchedBody, signature: mismatchedSignature }));
});

test("accepts old immutable deliveries during a bounded signing-secret overlap", () => {
  const oldDelivery = fixtures[0]!;
  assert.equal(verify(oldDelivery, { signingSecrets: ["new-signing-secret-not-yet-used", oldDelivery.secret] }).id,
    oldDelivery.deliveryId);
  rejects("invalid_signature", () => verify(oldDelivery, { signingSecrets: ["new-signing-secret-not-yet-used"] }));
});

test("a legitimate retry keeps one delivery identity", () => {
  const fixture = fixtures[0]!;
  const applied = new Set<string>();
  for (let attempt = 0; attempt < 2; attempt++) {
    const event = verify(fixture);
    if (!applied.has(event.id)) applied.add(event.id);
  }
  assert.deepEqual([...applied], [fixture.deliveryId]);
});
