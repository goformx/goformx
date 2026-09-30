#!/usr/bin/env python3
"""Small, local GoFormX contact-form client. No third-party dependencies."""

from __future__ import annotations

import argparse
from contextlib import closing
import hashlib
import json
import os
import secrets
import sqlite3
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path


class ClientError(Exception):
    pass


class NoRedirect(urllib.request.HTTPRedirectHandler):
    # urllib may otherwise forward Authorization to a different redirect target.
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        return None


def origin(value: str) -> str:
    if any(ord(char) <= 32 or char in "\\?#" for char in value):
        raise ClientError("API origin must be an HTTPS origin or loopback HTTP origin")
    parsed = urllib.parse.urlsplit(value)
    try:
        parsed.port
    except ValueError:
        raise ClientError("API origin has an invalid port") from None
    if parsed.scheme not in ("https", "http") or not parsed.hostname or parsed.username or parsed.password or parsed.path not in ("", "/") or parsed.query or parsed.fragment:
        raise ClientError("API origin must be an HTTPS origin or loopback HTTP origin")
    if parsed.scheme == "http" and parsed.hostname not in ("localhost", "127.0.0.1", "::1"):
        raise ClientError("HTTP is allowed only for loopback development")
    return value.rstrip("/")


def token_from_env(name: str) -> str:
    token = os.environ.get(name, "")
    if not token.startswith("gfst_") or len(token) <= 5:
        raise ClientError(f"{name} must contain a gfst_ service token from a user-only secret store")
    return token


class API:
    def __init__(self, base: str, token: str, organization_id: str, opener=None):
        self.base = origin(base)
        self.token = token
        self.organization_id = organization_id
        self.opener = opener or urllib.request.build_opener(NoRedirect).open

    def call(self, method: str, path: str, body=None, *, key=None, public=False, extra_headers=None, with_headers=False, expect_json=True):
        headers = {"Accept": "application/json", "User-Agent": "GoFormX-Codex-Contact/1.0"}
        if not public:
            headers["Authorization"] = "Bearer " + self.token
        if key:
            headers["Idempotency-Key"] = key
        if extra_headers:
            headers.update(extra_headers)
        data = None
        if body is not None:
            headers["Content-Type"] = "application/json"
            data = json.dumps(body, separators=(",", ":")).encode("utf-8")
        request = urllib.request.Request(self.base + path, data=data, headers=headers, method=method)
        try:
            with self.opener(request, timeout=15) as response:
                raw = response.read(1024 * 1024 + 1)
                if len(raw) > 1024 * 1024:
                    raise ClientError("API response exceeded 1 MiB")
                result = json.loads(raw) if expect_json else None
                return (result, response.headers) if with_headers else result
        except urllib.error.HTTPError as exc:
            exc.close()
            if exc.code == 401:
                raise ClientError("Connection unauthorized (401). Reauthorize or check revocation.") from None
            if exc.code == 403:
                raise ClientError("Operation forbidden (403). Check the connection grant.") from None
            if exc.code == 404:
                raise ClientError("Resource unavailable (404). Check its ID and authorized organization.") from None
            raise ClientError(f"API returned HTTP {exc.code}") from None
        except (urllib.error.URLError, TimeoutError, OSError) as exc:
            raise ClientError("Network outcome uncertain. Reconcile resource state before retrying this operation.") from None
        except (ValueError, UnicodeDecodeError):
            raise ClientError("API returned invalid JSON") from None

    def assert_organization(self):
        response = self.call("GET", "/v1/sites?limit=1&offset=0")
        if not isinstance(response, dict) or not isinstance(response.get("meta"), dict) or response["meta"].get("organizationId") != self.organization_id:
            raise ClientError("Token organization does not match the selected connection")


def header_value(headers, name: str) -> str:
    for key, value in headers.items():
        if key.lower() == name.lower():
            return value
    return ""


def segment(value: str) -> str:
    if not value or "/" in value or value in (".", ".."):
        raise ClientError("Invalid resource ID")
    return urllib.parse.quote(value, safe="")


def data(response):
    if not isinstance(response, dict) or not isinstance(response.get("data"), dict):
        raise ClientError("API response has no resource data")
    return response["data"]


def form_details(api: API, form_id: str):
    form = data(api.call("GET", "/v1/forms/" + segment(form_id)))
    if form.get("id") != form_id:
        raise ClientError("Form readback ID mismatch")
    if form.get("organizationId") != api.organization_id:
        raise ClientError("Form organization does not match the selected connection")
    version = form.get("currentVersion") or 1
    schema = data(api.call("GET", f"/v1/forms/{segment(form_id)}/versions/{version}"))
    if schema.get("formId") != form_id or schema.get("version") != version:
        raise ClientError("Schema readback mismatch")
    return form, schema


def site_details(api: API, site_id: str):
    site = data(api.call("GET", "/v1/sites/" + segment(site_id)))
    if site.get("id") != site_id or site.get("organizationId") != api.organization_id:
        raise ClientError("Site readback does not match the selected connection")
    return site


def same_origin(left: str, right: str) -> bool:
    a, b = urllib.parse.urlsplit(origin(left)), urllib.parse.urlsplit(origin(right))
    def port(parts):
        return parts.port or (443 if parts.scheme == "https" else 80)
    return (a.scheme, a.hostname, port(a)) == (b.scheme, b.hostname, port(b))


def browser_origin(value: str) -> str:
    result = origin(value)
    parsed = urllib.parse.urlsplit(result)
    host = parsed.hostname
    if not host or not host.isascii():
        raise ClientError("Site origin must use a browser-serialized ASCII host")
    host = host.lower()
    if ":" in host:
        host = f"[{host}]"
    default_port = 443 if parsed.scheme == "https" else 80
    suffix = f":{parsed.port}" if parsed.port and parsed.port != default_port else ""
    canonical = f"{parsed.scheme}://{host}{suffix}"
    if result != canonical:
        raise ClientError("Site origin must match the browser Origin header exactly")
    return canonical


def form_site_origin(api: API, form: dict) -> str:
    site_id = form.get("siteId")
    if not isinstance(site_id, str) or not site_id:
        raise ClientError("Form has no selected site")
    site_origin = browser_origin(site_details(api, site_id).get("origin", ""))
    allowed = form.get("allowedOrigins")
    # Go compares the raw Origin header to each stored allowedOrigins value.
    if not isinstance(allowed, list) or site_origin not in allowed:
        raise ClientError("Form does not allow the selected site origin")
    return site_origin


def ensure_site(api: API, name: str, site_origin: str):
    if not name:
        raise ClientError("Site name is required")
    requested_origin = origin(site_origin)
    api.assert_organization()
    # The server reconciles the same normalized origin and name to one site ID.
    site = data(api.call("POST", "/v1/sites", {"name": name, "origin": requested_origin}))
    site_id = site.get("id")
    if not isinstance(site_id, str):
        raise ClientError("Site create response has no ID")
    accepted = site_details(api, site_id)
    if accepted.get("name") != name:
        raise ClientError("Site readback name mismatch")
    actual_origin = origin(accepted.get("origin", ""))
    if not same_origin(requested_origin, actual_origin):
        raise ClientError("Site readback origin mismatch")
    return {"id": accepted["id"], "name": accepted["name"], "origin": actual_origin}


def summary(form, schema=None, **more):
    result = {k: form.get(k) for k in ("id", "siteId", "publicKey", "status") if k in form}
    if schema:
        result["schemaVersion"] = schema.get("version")
        result["schemaState"] = schema.get("state")
    result.update(more)
    return result


def state_file(root: Path, intent: str) -> Path:
    if not intent or len(intent) > 128:
        raise ClientError("Intent must have 1 to 128 characters")
    return root / (hashlib.sha256(intent.encode()).hexdigest() + ".json")


def save_state(path: Path, value: dict):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    fd, name = tempfile.mkstemp(dir=path.parent, prefix=".intent-", text=True)
    try:
        os.chmod(name, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as stream:
            json.dump(value, stream, sort_keys=True)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(name, path)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def intent_state(path: Path, digest: str, changed_message: str):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    # The SQLite transaction is the authoritative key record. FULL sync commits
    # it before a POST can begin, including on Windows where portable directory
    # fsync for a newly linked JSON file is unavailable.
    existing = None
    try:
        existing = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        pass
    except (ValueError, UnicodeDecodeError):
        raise ClientError("Intent state is incomplete; do not retry with a new key") from None
    if existing is not None:
        if not isinstance(existing, dict) or existing.get("digest") != digest:
            raise ClientError(changed_message)
        old_key = existing.get("key")
        if not isinstance(old_key, str) or not 16 <= len(old_key) <= 128:
            raise ClientError("Intent state has no valid key; do not retry with a new key")
    database = path.parent / "intents.sqlite3"
    try:
        with closing(sqlite3.connect(database, timeout=15)) as connection:
            with connection:
                connection.execute("PRAGMA synchronous=FULL")
                connection.execute("BEGIN IMMEDIATE")
                connection.execute("CREATE TABLE IF NOT EXISTS intents (name TEXT PRIMARY KEY, digest TEXT NOT NULL, key TEXT NOT NULL)")
                row = connection.execute("SELECT digest, key FROM intents WHERE name = ?", (path.name,)).fetchone()
                if row is None:
                    key = existing["key"] if existing is not None else secrets.token_urlsafe(32)
                    connection.execute("INSERT INTO intents (name, digest, key) VALUES (?, ?, ?)", (path.name, digest, key))
                else:
                    stored_digest, key = row
                    if stored_digest != digest:
                        raise ClientError(changed_message)
                    if existing is not None and existing["key"] != key:
                        raise ClientError("Intent state key conflicts with durable record; do not retry")
        return existing if existing is not None else {"digest": digest, "key": key}
    except sqlite3.DatabaseError:
        raise ClientError("Intent database is unavailable; do not retry with a new key") from None


def create(api: API, state_root: Path, intent: str, spec: dict):
    if not isinstance(spec, dict) or not all(k in spec for k in ("name", "title", "schema")):
        raise ClientError("Create spec needs name, title, and JSON Schema")
    api.assert_organization()
    site_id = spec.get("siteId")
    if not isinstance(site_id, str) or not site_id:
        raise ClientError("Create spec needs the selected siteId")
    site_origin = browser_origin(site_details(api, site_id).get("origin", ""))
    allowed = spec.get("allowedOrigins")
    if not isinstance(allowed, list) or site_origin not in allowed:
        raise ClientError("Create spec must allow the selected site origin")
    digest = hashlib.sha256(json.dumps({"origin": api.base, "organizationId": api.organization_id, "spec": spec}, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    path = state_file(state_root, intent)
    state = intent_state(path, digest, "Intent already belongs to a different create spec")
    if "formId" not in state:
        created = data(api.call("POST", "/v1/forms", spec, key=state["key"]))
        state["formId"] = created["id"]
        save_state(path, state)
    form, schema = form_details(api, state["formId"])
    if spec.get("siteId") != form.get("siteId"):
        raise ClientError("Form site association does not match the requested site")
    form_site_origin(api, form)
    if schema.get("schema") != spec["schema"]:
        raise ClientError("Accepted schema differs from the requested schema")
    return summary(form, schema)


def discover(api: API, *, form_id=None, site_id=None):
    if form_id:
        form, schema = form_details(api, form_id)
        if site_id and form.get("siteId") != site_id:
            raise ClientError("Form does not belong to the requested site")
        return [summary(form, schema)]
    if not site_id:
        raise ClientError("Specify an exact form ID or site ID")
    site_details(api, site_id)
    found = []
    offset = 0
    while offset <= 10000:
        path = f"/v1/forms?limit=100&offset={offset}"
        page = api.call("GET", path)
        entries = page.get("data")
        if not isinstance(entries, list):
            raise ClientError("Invalid forms page")
        found.extend(summary(item) for item in entries if item.get("siteId") == site_id and item.get("organizationId") == api.organization_id)
        offset += len(entries)
        if len(entries) < 100:
            return found
    raise ClientError("Form listing exceeded the API offset bound")


def integration(api: API, form_id: str) -> str:
    form, schema = form_details(api, form_id)
    site_origin = form_site_origin(api, form)
    key = form.get("publicKey", "")
    if not isinstance(key, str) or not key.startswith("gfpk_"):
        raise ClientError("Form has no browser-safe public key")
    if not isinstance(schema.get("schema"), dict):
        raise ClientError("Form has no accepted schema")
    # The generated file is pure browser code. No Authorization header or management token.
    return """// GoFormX public contact form. Publish the reviewed schema before use.
const apiOrigin = %s;
const siteOrigin = %s;
const publicKey = %s;
const schemaVersion = %s;
const acceptedSchema = %s;

// Caller keeps the same key when retrying an uncertain submission.
export async function submitContact(data, idempotencyKey) {
  if (window.location.origin !== siteOrigin) throw new Error('Unexpected site origin');
  if (typeof idempotencyKey !== 'string' || idempotencyKey.length < 16 || idempotencyKey.length > 128)
    throw new Error('A stable 16-128 character idempotency key is required');
  const response = await fetch(`${apiOrigin}/v1/public/forms/${encodeURIComponent(publicKey)}/submissions`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': idempotencyKey,
      'X-GoFormX-Schema-Version': String(schemaVersion),
    },
    body: JSON.stringify({ data }),
  });
  if (!response.ok) throw new Error(`GoFormX submission failed: ${response.status}`);
  return (await response.json()).data.id;
}
""" % (json.dumps(api.base), json.dumps(site_origin), json.dumps(key), json.dumps(schema["version"]), json.dumps(schema["schema"], separators=(",", ":")))


def publish(api: API, form_id: str, version: int, confirmed: bool):
    if not confirmed:
        raise ClientError("Publication requires --confirm and a forms:publish grant")
    api.assert_organization()
    form, schema = form_details(api, form_id)
    if schema.get("version") != version:
        raise ClientError("Review the exact schema version before publishing")
    if schema.get("state") == "published":
        return {"formId": form_id, "siteId": form.get("siteId"), "schemaVersion": version, "schemaState": "published"}
    result = data(api.call("POST", f"/v1/forms/{segment(form_id)}/versions/{version}/publish"))
    return {"formId": form_id, "siteId": form.get("siteId"), "schemaVersion": result.get("version"), "schemaState": result.get("state")}


def synthetic_test(api: API, form_id: str, payload: dict, state_root: Path, intent: str):
    api.assert_organization()
    form, schema = form_details(api, form_id)
    if form.get("status") != "published" or schema.get("state") != "published":
        raise ClientError("Public submission requires a published schema; review and publish separately first")
    site_origin = form_site_origin(api, form)
    key = form.get("publicKey", "")
    digest = hashlib.sha256(json.dumps({"origin": api.base, "formId": form_id, "version": schema["version"], "data": payload}, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    path = state_file(state_root, "submission:" + intent)
    state = intent_state(path, digest, "Test intent already belongs to a different submission")
    public_path = "/v1/public/forms/" + segment(key) + "/submissions"
    _, preflight_headers = api.call("OPTIONS", public_path, public=True, expect_json=False, with_headers=True,
        extra_headers={"Origin": site_origin, "Access-Control-Request-Method": "POST",
            "Access-Control-Request-Headers": "content-type,idempotency-key,x-goformx-schema-version"})
    allowed_headers = {item.strip().lower() for item in header_value(preflight_headers, "Access-Control-Allow-Headers").split(",")}
    allowed_methods = {item.strip().upper() for item in header_value(preflight_headers, "Access-Control-Allow-Methods").split(",")}
    if header_value(preflight_headers, "Access-Control-Allow-Origin") != site_origin or "POST" not in allowed_methods or not {"content-type", "idempotency-key", "x-goformx-schema-version"}.issubset(allowed_headers):
        raise ClientError("Browser CORS preflight rejected the selected site origin or headers")
    # Caller supplies only synthetic data. Do not echo response data or payload.
    envelope, response_headers = api.call("POST", public_path, {"data": payload}, key=state["key"], public=True,
        extra_headers={"Origin": site_origin, "X-GoFormX-Schema-Version": str(schema["version"])}, with_headers=True)
    if header_value(response_headers, "Access-Control-Allow-Origin") != site_origin:
        raise ClientError("Browser CORS response rejected the selected site origin")
    response = data(envelope)
    if response.get("schemaVersion") != schema["version"]:
        raise ClientError("Submission used a different schema version; result is unverified")
    return {"formId": form_id, "siteId": form.get("siteId"), "schemaVersion": response.get("schemaVersion"), "submissionId": response.get("id"), "status": response.get("status")}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--api-origin", required=True)
    parser.add_argument("--organization-id", required=True)
    parser.add_argument("--token-env", default="GOFORMX_SERVICE_TOKEN")
    parser.add_argument("--state-dir", type=Path, default=Path.home() / ".goformx" / "codex-intents")
    commands = parser.add_subparsers(dest="command", required=True)
    lookup = commands.add_parser("discover")
    lookup.add_argument("--form-id")
    lookup.add_argument("--site-id")
    site = commands.add_parser("ensure-site")
    site.add_argument("--name", required=True)
    site.add_argument("--site-origin", required=True)
    draft = commands.add_parser("create-draft")
    draft.add_argument("--intent", required=True)
    draft.add_argument("--spec", type=Path, required=True)
    generate = commands.add_parser("generate")
    generate.add_argument("--form-id", required=True)
    generate.add_argument("--output", type=Path, required=True)
    publication = commands.add_parser("publish")
    publication.add_argument("--form-id", required=True)
    publication.add_argument("--version", type=int, required=True)
    publication.add_argument("--confirm", action="store_true")
    trial = commands.add_parser("test-submission")
    trial.add_argument("--form-id", required=True)
    trial.add_argument("--intent", required=True)
    trial.add_argument("--synthetic-data", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        api = API(args.api_origin, token_from_env(args.token_env), args.organization_id)
        if args.command == "discover":
            output = discover(api, form_id=args.form_id, site_id=args.site_id)
        elif args.command == "ensure-site":
            output = ensure_site(api, args.name, args.site_origin)
        elif args.command == "create-draft":
            output = create(api, args.state_dir, args.intent, json.loads(args.spec.read_text(encoding="utf-8")))
        elif args.command == "generate":
            code = integration(api, args.form_id)
            args.output.parent.mkdir(parents=True, exist_ok=True)
            args.output.write_text(code, encoding="utf-8")
            output = {"generated": str(args.output), "formId": args.form_id}
        elif args.command == "publish":
            output = publish(api, args.form_id, args.version, args.confirm)
        else:
            output = synthetic_test(api, args.form_id, json.loads(args.synthetic_data.read_text(encoding="utf-8")), args.state_dir, args.intent)
        print(json.dumps(output, sort_keys=True))
        return 0
    except (ClientError, ValueError, KeyError, OSError) as exc:
        # Never include a server response body, request header, payload, or token in errors.
        print(str(exc) if isinstance(exc, ClientError) else "Invalid local input or API response", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
