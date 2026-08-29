UPDATE service_tokens
SET scopes = COALESCE((
    SELECT jsonb_agg(scope)
    FROM jsonb_array_elements(scopes) AS scope
    WHERE scope #>> '{}' NOT IN ('webhooks:read', 'webhooks:write')
), '[]'::jsonb);
