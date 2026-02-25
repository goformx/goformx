export interface PlanTier {
    name: string;
    tier: string;
    description: string;
    monthlyPrice: number | null;
    annualPrice: number | null;
    monthlyPriceId: string | null;
    annualPriceId: string | null;
    limits: {
        forms: number | string;
        submissions: number | string;
    };
    features: string[];
    highlighted?: boolean;
    cta: string;
    ctaVariant: 'default' | 'outline' | 'secondary';
}

export interface BillingUsage {
    forms: number;
    submissions: number;
}

export interface SubscriptionInfo {
    stripe_status: string | null;
    ends_at: string | null;
    trial_ends_at: string | null;
}
