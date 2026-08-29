-- Preserve existing webhook integrations while future token issuance uses the
-- narrower webhook scope family explicitly.
UPDATE service_tokens
SET scopes = scopes || '["webhooks:read"]'::jsonb
WHERE scopes ? 'forms:read' AND NOT scopes ? 'webhooks:read';

UPDATE service_tokens
SET scopes = scopes || '["webhooks:write"]'::jsonb
WHERE scopes ? 'forms:write' AND NOT scopes ? 'webhooks:write';
