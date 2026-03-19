export interface User {
    id: string;
    name: string;
    email: string;
    email_verified_at: string | null;
    two_factor_confirmed_at: string | null;
    stripe_id: string | null;
    plan_override: string | null;
    created_at: string;
    updated_at: string;
}

export interface Auth {
    user: User;
}

export interface PageProps {
    auth: Auth;
    errors: Record<string, string>;
    [key: string]: unknown;
}
