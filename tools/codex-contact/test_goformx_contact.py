import io
import json
import multiprocessing
import os
import sqlite3
import tempfile
import unittest
import urllib.error
from contextlib import closing
from pathlib import Path

from goformx_contact import API, ClientError, NoRedirect, create, discover, ensure_site, integration, intent_state, origin, publish, synthetic_test


FORM_ID = "a171585f-e96d-476c-9df1-762e9e156e80"
SITE_ID = "94745d4c-53a9-40f3-9a9f-f83e6925b899"
ORG_ID = "519d2911-26aa-4106-8e4d-ab4b1b05f1b0"
KEY = "gfpk_" + "A" * 24
SCHEMA = {"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"email": {"type": "string"}}, "required": ["email"]}


def state_worker(path, output):
    output.put(intent_state(Path(path), "digest-1", "changed")["key"])


def crash_after_commit_worker(path):
    intent_state(Path(path), "digest-1", "changed")
    os._exit(17)


class Response:
    def __init__(self, value, headers=None):
        self.stream = io.BytesIO(json.dumps(value).encode() if value is not None else b"")
        self.headers = headers or {}

    def __enter__(self):
        return self

    def __exit__(self, *_):
        return False

    def read(self, size):
        return self.stream.read(size)


class FakeServer:
    def __init__(self):
        self.requests = []
        self.post_count = 0
        self.uncertain_once = False
        self.uncertain_submission_once = False
        self.published = False
        self.status = None
        self.site_post_count = 0
        self.token_org = ORG_ID
        self.cors_origin = "https://site-a.test"
        self.cors_post_origin = "https://site-a.test"
        self.submission_version = 1

    def __call__(self, request, timeout):
        self.requests.append(request)
        path = request.full_url.removeprefix("https://api.example.test")
        if self.status:
            raise urllib.error.HTTPError(request.full_url, self.status, "blocked", {}, io.BytesIO(b"secret response"))
        if path == "/v1/sites?limit=1&offset=0":
            return Response({"data": [], "meta": {"limit": 1, "offset": 0, "total": 0, "organizationId": self.token_org}})
        if path == "/v1/sites" and request.get_method() == "POST":
            self.site_post_count += 1
            return Response({"data": self.site()})
        if path == "/v1/sites/" + SITE_ID:
            return Response({"data": self.site()})
        if path == "/v1/forms" and request.get_method() == "POST":
            self.post_count += 1
            if self.uncertain_once:
                self.uncertain_once = False
                raise urllib.error.URLError("timeout")
            return Response({"data": self.form()})
        if path == "/v1/forms/" + FORM_ID:
            return Response({"data": self.form()})
        if path == f"/v1/forms/{FORM_ID}/versions/1":
            return Response({"data": {"formId": FORM_ID, "version": 1, "state": "published" if self.published else "draft", "schema": SCHEMA}})
        if path == f"/v1/forms/{FORM_ID}/versions/1/publish":
            self.published = True
            return Response({"data": {"formId": FORM_ID, "version": 1, "state": "published"}})
        if path.startswith("/v1/forms?limit=100&offset="):
            return Response({"data": [self.form()], "meta": {"total": 1, "offset": 0, "limit": 100}})
        if path == f"/v1/public/forms/{KEY}/submissions":
            if request.get_method() == "OPTIONS":
                return Response(None, {"Access-Control-Allow-Origin": self.cors_origin,
                    "Access-Control-Allow-Methods": "POST, OPTIONS",
                    "Access-Control-Allow-Headers": "Content-Type, Idempotency-Key, X-GoFormX-Schema-Version"})
            if self.uncertain_submission_once:
                self.uncertain_submission_once = False
                raise urllib.error.URLError("timeout")
            return Response({"data": {"id": "submission-1", "schemaVersion": self.submission_version, "status": "accepted", "data": {"email": "sensitive@example.test"}}},
                {"Access-Control-Allow-Origin": self.cors_post_origin})
        raise AssertionError(path)

    def form(self):
        return {"id": FORM_ID, "siteId": SITE_ID, "organizationId": ORG_ID, "publicKey": KEY,
            "allowedOrigins": ["https://site-a.test"], "status": "published" if self.published else "draft", "currentVersion": 1}

    def site(self):
        return {"id": SITE_ID, "organizationId": ORG_ID, "name": "Site A", "origin": "https://site-a.test"}


class ClientTests(unittest.TestCase):
    def setUp(self):
        self.fake = FakeServer()
        self.token = "gfst_" + "private-token"
        self.api = API("https://api.example.test", self.token, ORG_ID, opener=self.fake)
        self.spec = {"name": "my-contact", "title": "My Contact", "siteId": SITE_ID,
            "allowedOrigins": ["https://site-a.test"], "schema": SCHEMA}

    def test_origin_rejects_remote_plain_http_and_credentials(self):
        for value in ("http://api.example.test", "https://user:pass@api.example.test", "https://api.example.test/v1"):
            with self.assertRaises(ClientError):
                origin(value)
        self.assertEqual(origin("http://127.0.0.1:8080"), "http://127.0.0.1:8080")
        self.assertIsNone(NoRedirect().redirect_request(None, None, 302, "redirect", {}, "https://elsewhere.test"))

    def test_create_reuses_durable_key_after_uncertain_response(self):
        self.fake.uncertain_once = True
        with tempfile.TemporaryDirectory() as folder:
            with self.assertRaisesRegex(ClientError, "uncertain"):
                create(self.api, Path(folder), "new-contact-site-a", self.spec)
            first_key = next(request.get_header("Idempotency-key") for request in self.fake.requests if request.get_method() == "POST")
            result = create(self.api, Path(folder), "new-contact-site-a", self.spec)
            self.assertEqual(result["id"], FORM_ID)
            self.assertEqual(self.fake.post_count, 2)
            post_requests = [request for request in self.fake.requests if request.get_method() == "POST"]
            self.assertEqual(post_requests[1].get_header("Idempotency-key"), first_key)
            self.assertGreaterEqual(len(first_key), 16)
            self.assertLessEqual(len(first_key), 128)
            self.assertNotIn(self.token, Path(folder).glob("*.json").__iter__().__next__().read_text())
            with self.assertRaisesRegex(ClientError, "different create spec"):
                create(self.api, Path(folder), "new-contact-site-a", {**self.spec, "title": "Different"})

    def test_scope_and_revocation_fail_without_exposing_body_or_token(self):
        for status, expected in ((401, "reauthorize"), (403, "grant"), (404, "Resource unavailable")):
            self.fake.status = status
            with self.assertRaises(ClientError) as caught:
                discover(self.api, form_id=FORM_ID)
            message = str(caught.exception)
            self.assertIn(expected.lower(), message.lower())
            self.assertNotIn(self.token, message)
            self.assertNotIn("secret response", message)

    def test_generate_public_code_and_never_publish_automatically(self):
        output = integration(self.api, FORM_ID)
        self.assertIn(KEY, output)
        self.assertIn("acceptedSchema", output)
        self.assertIn("siteOrigin", output)
        self.assertIn("idempotencyKey", output)
        self.assertNotIn(self.token, output)
        self.assertNotIn("Authorization", output)
        self.assertFalse(any("/publish" in request.full_url for request in self.fake.requests))
        with self.assertRaisesRegex(ClientError, "requires --confirm"):
            publish(self.api, FORM_ID, 1, False)
        self.assertFalse(any("/publish" in request.full_url for request in self.fake.requests))

    def test_public_origin_requires_exact_server_cors_value(self):
        with tempfile.TemporaryDirectory() as folder:
            for allowed in ("https://site-a.test/", "https://site-a.test:443"):
                with self.subTest(allowed=allowed):
                    with self.assertRaisesRegex(ClientError, "allow the selected site origin"):
                        create(self.api, Path(folder), "origin-mismatch", {**self.spec, "allowedOrigins": [allowed]})
            self.assertEqual(self.fake.post_count, 0)
        original_form = self.fake.form

        def mismatched_form():
            return {**original_form(), "allowedOrigins": ["https://site-a.test/"]}

        self.fake.form = mismatched_form
        with self.assertRaisesRegex(ClientError, "allow the selected site origin"):
            integration(self.api, FORM_ID)

    def test_site_origin_must_match_browser_serialization(self):
        original_site = self.fake.site

        def default_port_site():
            return {**original_site(), "origin": "https://site-a.test:443"}

        self.fake.site = default_port_site
        with self.assertRaisesRegex(ClientError, "browser Origin header"):
            integration(self.api, FORM_ID)

    def test_test_submission_requires_publish_and_uses_public_endpoint(self):
        with tempfile.TemporaryDirectory() as folder:
            with self.assertRaisesRegex(ClientError, "published schema"):
                synthetic_test(self.api, FORM_ID, {"email": "synthetic@example.test"}, Path(folder), "site-a-test")
        self.assertFalse(any("/submissions" in request.full_url for request in self.fake.requests))
        result = publish(self.api, FORM_ID, 1, True)
        self.assertEqual(result["schemaState"], "published")
        with tempfile.TemporaryDirectory() as folder:
            result = synthetic_test(self.api, FORM_ID, {"email": "synthetic@example.test"}, Path(folder), "site-a-test")
        self.assertEqual(result["status"], "accepted")
        self.assertNotIn("email", json.dumps(result))
        request = self.fake.requests[-1]
        self.assertIsNone(request.get_header("Authorization"))
        self.assertGreaterEqual(len(request.get_header("Idempotency-key")), 16)
        self.assertEqual(request.get_header("Origin"), "https://site-a.test")
        self.assertEqual(request.get_header("X-goformx-schema-version"), "1")

    def test_discover_is_exact_by_site_or_form(self):
        self.assertEqual(discover(self.api, site_id=SITE_ID)[0]["id"], FORM_ID)
        with self.assertRaisesRegex(ClientError, "does not belong"):
            discover(self.api, form_id=FORM_ID, site_id="another-site")

    def test_site_creation_reconciles_and_reads_back(self):
        self.assertEqual(ensure_site(self.api, "Site A", "https://site-a.test/")["id"], SITE_ID)
        self.assertEqual(ensure_site(self.api, "Site A", "https://site-a.test")["id"], SITE_ID)
        self.assertEqual(self.fake.site_post_count, 2)
        self.assertNotIn(self.token, json.dumps(self.fake.site()))

    def test_synthetic_retry_keeps_key_after_uncertain_response(self):
        self.fake.published = True
        self.fake.uncertain_submission_once = True
        with tempfile.TemporaryDirectory() as folder:
            with self.assertRaisesRegex(ClientError, "uncertain"):
                synthetic_test(self.api, FORM_ID, {"email": "synthetic@example.test"}, Path(folder), "same-test")
            first = self.fake.requests[-1].get_header("Idempotency-key")
            synthetic_test(self.api, FORM_ID, {"email": "synthetic@example.test"}, Path(folder), "same-test")
            self.assertEqual(self.fake.requests[-1].get_header("Idempotency-key"), first)

    def test_wrong_organization_stops_all_mutations_before_post(self):
        self.fake.token_org = "different-organization"
        self.fake.published = True
        with tempfile.TemporaryDirectory() as folder:
            actions = (
                lambda: ensure_site(self.api, "Site A", "https://site-a.test"),
                lambda: create(self.api, Path(folder), "contact", self.spec),
                lambda: publish(self.api, FORM_ID, 1, True),
                lambda: synthetic_test(self.api, FORM_ID, {"email": "synthetic@example.test"}, Path(folder), "wrong-org"),
            )
            for action in actions:
                with self.assertRaisesRegex(ClientError, "Token organization"):
                    action()
        self.assertFalse(any(request.get_method() == "POST" for request in self.fake.requests))

    def test_browser_cors_failure_does_not_report_accepted(self):
        self.fake.published = True
        self.fake.cors_origin = "https://other-site.test"
        with tempfile.TemporaryDirectory() as folder:
            with self.assertRaisesRegex(ClientError, "CORS preflight"):
                synthetic_test(self.api, FORM_ID, {"email": "synthetic@example.test"}, Path(folder), "cors-test")
        self.assertFalse(any(request.get_method() == "POST" and "/submissions" in request.full_url for request in self.fake.requests))

    def test_schema_version_race_is_unverified(self):
        self.fake.published = True
        self.fake.submission_version = 2
        with tempfile.TemporaryDirectory() as folder:
            with self.assertRaisesRegex(ClientError, "different schema version"):
                synthetic_test(self.api, FORM_ID, {"email": "synthetic@example.test"}, Path(folder), "version-race")

    def test_cors_response_failure_is_not_green(self):
        self.fake.published = True
        self.fake.cors_post_origin = "https://other-site.test"
        with tempfile.TemporaryDirectory() as folder:
            with self.assertRaisesRegex(ClientError, "CORS response"):
                synthetic_test(self.api, FORM_ID, {"email": "synthetic@example.test"}, Path(folder), "cors-response")

    def test_two_processes_reuse_the_same_intent_key(self):
        with tempfile.TemporaryDirectory() as folder:
            context = multiprocessing.get_context("spawn")
            output = context.Queue()
            path = str(Path(folder) / "same-intent.json")
            processes = [context.Process(target=state_worker, args=(path, output)) for _ in range(8)]
            for process in processes:
                process.start()
            for process in processes:
                process.join(timeout=10)
                self.assertEqual(process.exitcode, 0)
            keys = [output.get(timeout=1) for _ in processes]
            self.assertEqual(len(set(keys)), 1)
            self.assertEqual(intent_state(Path(path), "digest-1", "changed")["key"], keys[0])

            # A new process has no cached state, so this proves restart reuse.
            restarted = context.Process(target=state_worker, args=(path, output))
            restarted.start()
            restarted.join(timeout=10)
            self.assertEqual(restarted.exitcode, 0)
            self.assertEqual(output.get(timeout=1), keys[0])

    def test_process_crash_after_commit_preserves_winning_key(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "crashed-intent.json"
            context = multiprocessing.get_context("spawn")
            worker = context.Process(target=crash_after_commit_worker, args=(str(path),))
            worker.start()
            worker.join(timeout=10)
            self.assertEqual(worker.exitcode, 17)
            database = path.parent / "intents.sqlite3"
            self.assertTrue(database.is_file())
            with closing(sqlite3.connect(database)) as connection:
                saved = connection.execute("SELECT key FROM intents WHERE name = ?", (path.name,)).fetchone()[0]
            self.assertGreaterEqual(len(saved), 16)
            self.assertEqual(intent_state(path, "digest-1", "changed")["key"], saved)

    def test_partial_intent_state_never_silently_mints_a_new_key(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "damaged-intent.json"
            for partial in ('{"digest":', '{"digest":"digest-1"}', '{"digest":"digest-1","key":"short"}'):
                with self.subTest(partial=partial):
                    path.write_text(partial, encoding="utf-8")
                    with self.assertRaisesRegex(ClientError, "Intent state"):
                        intent_state(path, "digest-1", "changed")
                    self.assertEqual(path.read_text(encoding="utf-8"), partial)

    def test_existing_json_key_migrates_without_change(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "legacy-intent.json"
            prior = {"digest": "digest-1", "key": "stable-legacy-idempotency-key", "formId": FORM_ID}
            path.write_text(json.dumps(prior), encoding="utf-8")
            self.assertEqual(intent_state(path, "digest-1", "changed"), prior)
            self.assertEqual(intent_state(path, "digest-1", "changed")["key"], prior["key"])

    def test_corrupt_intent_database_fails_closed(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            database = root / "intents.sqlite3"
            database.write_bytes(b"not a sqlite database")
            with self.assertRaisesRegex(ClientError, "Intent database is unavailable"):
                intent_state(root / "same-intent.json", "digest-1", "changed")
            self.assertEqual(database.read_bytes(), b"not a sqlite database")


if __name__ == "__main__":
    unittest.main()
