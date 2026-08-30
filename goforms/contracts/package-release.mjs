import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../", import.meta.url));
const git = (...args) => execFileSync("git", args, { cwd: root });
if (git("status", "--porcelain").toString().trim()) {
  throw new Error("Package only a clean, verified commit; commit source and generated artifacts first.");
}
const revision = git("rev-parse", "HEAD").toString().trim();
// Hash committed bytes, not a checkout's platform-dependent line endings.
const committed = name => git("show", `HEAD:goforms/contracts/generated/${name}`);
const openapi = committed("openapi.json");
const version = JSON.parse(openapi).info.version;
if (!/^\d+\.\d+\.\d+$/.test(version)) throw new Error("Contract release requires an explicit semantic version.");
const output = new URL("../.contract-release/", import.meta.url);
mkdirSync(output, { recursive: true });
const archiveName = `goformx-contract-${version}.zip`;
const archive = git("archive", "--format=zip", "HEAD", "LICENSE", "goforms/LICENSE", "docs/api-clients.md", "goforms/contracts", "goforms/package.json", "goforms/package-lock.json");
const sha256 = bytes => createHash("sha256").update(bytes).digest("hex");
const base = `https://raw.githubusercontent.com/goformx/goformx/${revision}/goforms/contracts/generated/`;
const manifest = Buffer.from(JSON.stringify({
  schema: "goformx.contract-release.v1", version, sourceRevision: revision,
  openapi: { url: `${base}openapi.json`, sha256: sha256(openapi) },
  formSchema: { url: `${base}schema/form-definition.schema.json`, sha256: sha256(committed("schema/form-definition.schema.json")) },
  assertionSchema: { url: `${base}auth/first-party-assertion.claims.schema.json`, sha256: sha256(committed("auth/first-party-assertion.claims.schema.json")) },
  clientTypes: { url: `${base}api.d.ts`, sha256: sha256(committed("api.d.ts")) },
  clientExample: { url: `https://github.com/goformx/goformx/releases/download/contract-v${version}/${archiveName}`, sha256: sha256(archive) },
}, null, 2) + "\n");
const files = { "openapi.json": openapi, "manifest.json": manifest, [archiveName]: archive };
for (const [name, bytes] of Object.entries(files)) writeFileSync(new URL(name, output), bytes);
writeFileSync(new URL("SHA256SUMS", output), Object.entries(files).map(([name, bytes]) => `${sha256(bytes)}  ${name}\n`).join(""));
console.log(`Packaged contract ${version} from ${revision}; no release or deployment was performed.`);
