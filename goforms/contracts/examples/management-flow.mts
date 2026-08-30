// Server-side example. Never bundle the management client or its credential into a browser.
import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import createClient from "openapi-fetch";
import type { components, paths } from "../generated/api.js";

async function main(): Promise<void> {
  const baseUrl = process.env.GOFORMX_API_URL;
  const token = process.env.GOFORMX_SERVICE_TOKEN;
  assert.equal(process.env.GOFORMX_ALLOW_EXAMPLE_WRITES, "1", "Explicitly allow writes to a disposable organization.");
  assert.ok(baseUrl && token, "GOFORMX_API_URL and GOFORMX_SERVICE_TOKEN are required.");
  const url = new URL(baseUrl);
  assert.ok(url.protocol === "https:" || (url.protocol === "http:" && ["localhost", "127.0.0.1", "[::1]"].includes(url.hostname)), "Use HTTPS outside loopback.");
  assert.ok(!url.username && !url.password && !url.search && !url.hash, "Use a service origin without credentials or query data.");
  assert.equal(url.pathname, "/", "The service origin must not include /v1.");
  assert.ok(token.startsWith("gfst_"), "This external-client example requires a scoped service token, not a first-party assertion.");

  const safeFetch = (request: Request): Promise<Response> => fetch(request, {
    redirect: "error", signal: AbortSignal.timeout(10_000),
  });
  const management = createClient<paths>({ baseUrl, headers: { Authorization: `Bearer ${token}` }, fetch: safeFetch });
  // Deliberately separate: public schema/submission calls carry no management credential.
  const publicClient = createClient<paths>({ baseUrl, fetch: safeFetch });
  const schema: components["schemas"]["FormDefinition"] = {
    $schema: "https://json-schema.org/draft/2020-12/schema",
    type: "object",
    properties: { email: { type: "string", format: "email" } },
    required: ["email"], additionalProperties: false,
  };
  const created = await management.POST("/v1/forms", {
    body: { name: `example-${randomUUID()}`, title: "Contract example", schema },
  });
  assert.equal(created.response.status, 201, "Create form failed; inspect status/request ID, not raw payload logs.");
  assert.ok(created.data);
  const form = created.data.data;
  assert.equal(form.status, "draft");

  const updatedSchema: components["schemas"]["FormDefinition"] = {
    ...schema, properties: { ...schema.properties, message: { type: "string", minLength: 1 } },
    required: ["email", "message"],
  };
  const draft = await management.POST("/v1/forms/{formId}/versions", {
    params: { path: { formId: form.id } }, body: { schema: updatedSchema },
  });
  assert.equal(draft.response.status, 201);
  assert.ok(draft.data);
  const version = draft.data.data.version;
  assert.equal(version, 2);
  assert.equal(draft.data.data.state, "draft");

  // Explicit publication is a separate operation, never a side effect of editing.
  const published = await management.POST("/v1/forms/{formId}/versions/{version}/publish", {
    params: { path: { formId: form.id, version } },
  });
  assert.equal(published.response.status, 200);
  assert.equal(published.data?.data.state, "published");
  const publicSchema = await publicClient.GET("/v1/public/forms/{publicKey}/schema", {
    params: { path: { publicKey: form.publicKey } },
  });
  assert.equal(publicSchema.response.status, 200);
  assert.equal(publicSchema.response.headers.get("X-GoFormX-Schema-Version"), String(version));

  const current = await management.GET("/v1/forms/{formId}", { params: { path: { formId: form.id } } });
  assert.equal(current.response.status, 200);
  const etag = current.response.headers.get("ETag");
  assert.ok(etag);
  const metadata = await management.PATCH("/v1/forms/{formId}", {
    params: { path: { formId: form.id }, header: { "If-Match": etag } },
    body: { title: "Contract example updated" },
  });
  assert.equal(metadata.response.status, 200);
  assert.equal(metadata.data?.data.title, "Contract example updated");

  const invalid = await publicClient.POST("/v1/public/forms/{publicKey}/submissions", {
    params: { path: { publicKey: form.publicKey }, header: { "Idempotency-Key": randomUUID(), "X-GoFormX-Schema-Version": version } },
    body: { data: { email: "not-an-email", message: "Synthetic example" } },
  });
  assert.equal(invalid.response.status, 422);
  assert.ok(invalid.error?.error.fields?.some(field => field.pointer.includes("email")));
  const submissionRequest = {
    params: { path: { publicKey: form.publicKey }, header: { "Idempotency-Key": randomUUID(), "X-GoFormX-Schema-Version": version } },
    body: { data: { email: "example@example.test", message: "Synthetic example" } },
  };
  const submitted = await publicClient.POST("/v1/public/forms/{publicKey}/submissions", submissionRequest);
  assert.equal(submitted.response.status, 202);
  assert.ok(submitted.data);
  const submission = submitted.data.data;
  const replay = await publicClient.POST("/v1/public/forms/{publicKey}/submissions", submissionRequest);
  assert.equal(replay.response.status, 202);
  assert.equal(replay.data?.data.id, submission.id);

  const detail = await management.GET("/v1/forms/{formId}/submissions/{submissionId}", {
    params: { path: { formId: form.id, submissionId: submission.id } },
  });
  assert.equal(detail.response.status, 200);
  assert.equal(detail.data?.data.schemaVersion, version);
  assert.equal(detail.data?.data.requestId, submission.requestId);
  assert.ok(submission.requestId);
  const listed = await management.GET("/v1/forms/{formId}/submissions", {
    params: { path: { formId: form.id }, query: { limit: 1 } },
  });
  assert.equal(listed.response.status, 200);
  assert.equal(listed.data?.data.length, 1);
  assert.equal(listed.data?.data[0]?.id, submission.id);

  // Only non-secret synthetic resource IDs are emitted; never log response bodies.
  console.log(JSON.stringify({ formId: form.id, submissionId: submission.id, schemaVersion: version }));
}

main().catch(() => {
  console.error("Contract example failed. Check configuration and API status; credentials and response payloads are intentionally omitted.");
  process.exitCode = 1;
});
