-- Reference ACL contract, NOT a migration or an existing-role repair script.
-- See docs/database-permissions.md for provisioning/ownership preconditions.
-- psql identifier variables: database, owner, migrator, runtime, token_operator,
-- key_operator, backup. Execute atomically as the dedicated object owner.

REVOKE ALL ON DATABASE :"database" FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT CONNECT ON DATABASE :"database" TO :"migrator", :"runtime", :"token_operator", :"key_operator", :"backup";
GRANT USAGE ON SCHEMA public TO :"runtime", :"token_operator", :"key_operator", :"backup";

REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM PUBLIC;
REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA public FROM PUBLIC;
-- Global (not IN SCHEMA): a schema-local revoke cannot remove the default
-- global PUBLIC EXECUTE grant. Only objects made AS owner inherit these ACLs.
ALTER DEFAULT PRIVILEGES FOR ROLE :"owner" REVOKE ALL ON TABLES FROM PUBLIC;
ALTER DEFAULT PRIVILEGES FOR ROLE :"owner" REVOKE ALL ON SEQUENCES FROM PUBLIC;
ALTER DEFAULT PRIVILEGES FOR ROLE :"owner" REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE ON public.forms TO :"runtime";
GRANT SELECT, INSERT ON public.form_schemas TO :"runtime";
GRANT UPDATE (state, published_at) ON public.form_schemas TO :"runtime";
GRANT SELECT, INSERT ON public.form_submissions TO :"runtime";
GRANT SELECT, INSERT ON public.service_tokens TO :"runtime", :"token_operator";
GRANT UPDATE (last_used_at, revoked_at, revocation_reason) ON public.service_tokens TO :"runtime";
GRANT UPDATE (revoked_at, revocation_reason, replaced_by_token_id) ON public.service_tokens TO :"token_operator";
GRANT SELECT, INSERT, DELETE ON public.first_party_assertion_replays TO :"runtime";
GRANT SELECT, INSERT ON public.management_audit TO :"runtime", :"token_operator";
GRANT SELECT, INSERT ON public.submission_export_audit TO :"runtime";

GRANT SELECT, INSERT, DELETE ON public.webhook_endpoints TO :"runtime";
GRANT UPDATE (destination_origin, encrypted_config, enabled, updated_at) ON public.webhook_endpoints TO :"runtime";
GRANT SELECT, INSERT ON public.webhook_deliveries TO :"runtime";
GRANT UPDATE (status, attempt_count, next_attempt_at, locked_at, updated_at,
              delivered_at, last_http_status, last_error_category)
    ON public.webhook_deliveries TO :"runtime";

-- PG17 MAINTAIN permits rotation's SHARE ROW EXCLUSIVE table locks without
-- granting UPDATE of delivery snapshots/metadata beyond encrypted_config.
GRANT SELECT, MAINTAIN ON public.webhook_endpoints, public.webhook_deliveries TO :"key_operator";
GRANT UPDATE (encrypted_config) ON public.webhook_endpoints, public.webhook_deliveries TO :"key_operator";

-- Backup is intentionally a complete data reader, not a restore/owner role.
-- Future tables need an explicit reviewed ACL refresh before backup succeeds.
GRANT SELECT ON ALL TABLES IN SCHEMA public TO :"backup";
