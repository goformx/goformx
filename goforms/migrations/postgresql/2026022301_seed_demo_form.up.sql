-- Seed a demo user and demo form for the public /demo page when GOFORMS_DEMO_FORM_ID is not set.
-- Demo user is for form ownership only; Laravel does not use this for login.
-- Bcrypt hash for placeholder password (user is for form ownership only, not login).
INSERT INTO users (
    uuid,
    email,
    hashed_password,
    first_name,
    last_name,
    role,
    active,
    created_at,
    updated_at
) VALUES (
    '11111111-1111-4111-8111-111111111111',
    'demo@goformx.internal',
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
    'Demo',
    'User',
    'user',
    true,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
) ON CONFLICT (uuid) DO NOTHING;

-- Minimal Form.io schema: one email field and submit button.
INSERT INTO forms (
    uuid,
    user_id,
    title,
    description,
    schema,
    active,
    status,
    created_at,
    updated_at,
    cors_origins,
    cors_methods,
    cors_headers
) VALUES (
    '22222222-2222-4222-8222-222222222222',
    '11111111-1111-4111-8111-111111111111',
    'Demo',
    'Try GoFormX',
    '{"display":"form","components":[{"type":"textfield","key":"email","label":"Email","input":true,"validate":{"required":true}},{"type":"button","key":"submit","label":"Submit","action":"submit"}]}',
    true,
    'draft',
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP,
    '["https://goformx.com"]',
    '["GET", "POST", "OPTIONS"]',
    '["Content-Type", "Accept", "Origin"]'
) ON CONFLICT (uuid) DO NOTHING;
