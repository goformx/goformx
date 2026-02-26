<script setup lang="ts">
import { Head, Link, router, usePage } from '@inertiajs/vue3';
import { Check } from 'lucide-vue-next';
import { computed, ref } from 'vue';
import PublicFooter from '@/components/PublicFooter.vue';
import PublicHeader from '@/components/PublicHeader.vue';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardFooter,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { register } from '@/routes';
import { checkout } from '@/routes/billing';
import type { PlanTier } from '@/types/billing';

const props = defineProps<{
    currentTier: string;
    prices: Record<string, string | null>;
}>();

const page = usePage();
const isAnnual = ref(false);
const isAuthenticated = computed(() => !!page.props.auth?.user);

const plans = computed<PlanTier[]>(() => [
    {
        name: 'Free',
        tier: 'free',
        description: 'For personal projects and testing.',
        monthlyPrice: 0,
        annualPrice: 0,
        monthlyPriceId: null,
        annualPriceId: null,
        limits: { forms: 3, submissions: 100 },
        features: [
            'Up to 3 forms',
            '100 submissions/month',
            'Basic form fields',
            'Public form links',
            'Email notifications',
        ],
        cta: 'Get Started',
        ctaVariant: 'outline',
    },
    {
        name: 'Pro',
        tier: 'pro',
        description: 'For professionals and small teams.',
        monthlyPrice: 9,
        annualPrice: 90,
        monthlyPriceId: props.prices.pro_monthly ?? null,
        annualPriceId: props.prices.pro_annual ?? null,
        limits: { forms: 10, submissions: '1,000' },
        features: [
            'Up to 10 forms',
            '1,000 submissions/month',
            'File uploads',
            'Signature fields',
            'Custom CORS origins',
            'Basic analytics',
        ],
        highlighted: true,
        cta: 'Subscribe',
        ctaVariant: 'default',
    },
    {
        name: 'Business',
        tier: 'business',
        description: 'For growing businesses.',
        monthlyPrice: 29,
        annualPrice: 290,
        monthlyPriceId: props.prices.business_monthly ?? null,
        annualPriceId: props.prices.business_annual ?? null,
        limits: { forms: 50, submissions: '10,000' },
        features: [
            'Up to 50 forms',
            '10,000 submissions/month',
            'Everything in Pro',
            'Webhooks',
            'Full analytics',
            'Team collaboration',
            'Priority support',
        ],
        cta: 'Subscribe',
        ctaVariant: 'default',
    },
    {
        name: 'Growth',
        tier: 'growth',
        description: 'For scaling teams and agencies.',
        monthlyPrice: 59,
        annualPrice: 590,
        monthlyPriceId: props.prices.growth_monthly ?? null,
        annualPriceId: props.prices.growth_annual ?? null,
        limits: { forms: 150, submissions: '50,000' },
        features: [
            'Up to 150 forms',
            '50,000 submissions/month',
            'Everything in Business',
            'Advanced integrations',
            'Audit logs',
            'Custom branding',
            'Multi-team roles',
        ],
        cta: 'Subscribe',
        ctaVariant: 'default',
    },
    {
        name: 'Enterprise',
        tier: 'enterprise',
        description: 'For large organizations.',
        monthlyPrice: null,
        annualPrice: null,
        monthlyPriceId: null,
        annualPriceId: null,
        limits: { forms: 'Unlimited', submissions: 'Unlimited' },
        features: [
            'Unlimited forms',
            'Unlimited submissions',
            'Everything in Growth',
            'SSO / SAML',
            'Dedicated support',
            'Custom SLA',
        ],
        cta: 'Contact Us',
        ctaVariant: 'secondary',
    },
]);

function priceId(plan: PlanTier): string | null {
    return isAnnual.value ? plan.annualPriceId : plan.monthlyPriceId;
}

function displayPrice(plan: PlanTier): string {
    const price = isAnnual.value ? plan.annualPrice : plan.monthlyPrice;
    if (price === null) return 'Custom';
    if (price === 0) return '$0';
    return `$${price}`;
}

function pricePeriod(plan: PlanTier): string {
    if (plan.monthlyPrice === null) return '';
    return isAnnual.value ? '/year' : '/month';
}

function handleSubscribe(plan: PlanTier) {
    const id = priceId(plan);
    if (!id) return;
    router.post(checkout.url(), { price_id: id });
}
</script>

<template>
    <div class="flex min-h-screen flex-col bg-background text-foreground">
        <Head title="Pricing" />

        <PublicHeader />

        <main class="flex-1">
            <!-- Hero -->
            <section class="py-16 text-center md:py-24">
                <div class="container">
                    <h1
                        class="font-display text-4xl font-semibold tracking-tight sm:text-5xl"
                    >
                        Simple, transparent pricing
                    </h1>
                    <p
                        class="mx-auto mt-4 max-w-2xl text-lg text-muted-foreground"
                    >
                        Start free. Upgrade as you grow. No hidden fees.
                    </p>

                    <!-- Billing toggle -->
                    <div class="mt-8 flex items-center justify-center gap-3">
                        <Label
                            class="cursor-pointer text-sm"
                            :class="
                                !isAnnual
                                    ? 'text-foreground'
                                    : 'text-muted-foreground'
                            "
                        >
                            Monthly
                        </Label>
                        <Switch v-model="isAnnual" />
                        <Label
                            class="cursor-pointer text-sm"
                            :class="
                                isAnnual
                                    ? 'text-foreground'
                                    : 'text-muted-foreground'
                            "
                        >
                            Annual
                            <Badge variant="secondary" class="ml-1"
                                >Save 17%</Badge
                            >
                        </Label>
                    </div>
                </div>
            </section>

            <!-- Pricing cards -->
            <section class="pb-24">
                <div class="container">
                    <div class="grid gap-6 md:grid-cols-2 lg:grid-cols-5">
                        <Card
                            v-for="plan in plans"
                            :key="plan.tier"
                            :class="[
                                'relative flex flex-col',
                                plan.highlighted
                                    ? 'border-[hsl(var(--brand))] shadow-lg'
                                    : 'border-border/50',
                            ]"
                        >
                            <CardHeader>
                                <div class="flex items-center gap-2">
                                    <CardTitle>{{ plan.name }}</CardTitle>
                                    <Badge
                                        v-if="currentTier === plan.tier"
                                        variant="secondary"
                                    >
                                        Current
                                    </Badge>
                                    <Badge
                                        v-else-if="plan.highlighted"
                                        variant="default"
                                    >
                                        Popular
                                    </Badge>
                                </div>
                                <CardDescription>{{
                                    plan.description
                                }}</CardDescription>
                            </CardHeader>

                            <CardContent class="flex-1">
                                <div class="mb-6">
                                    <span class="text-4xl font-bold">{{
                                        displayPrice(plan)
                                    }}</span>
                                    <span
                                        v-if="pricePeriod(plan)"
                                        class="text-sm text-muted-foreground"
                                    >
                                        {{ pricePeriod(plan) }}
                                    </span>
                                </div>

                                <ul class="space-y-2.5">
                                    <li
                                        v-for="feature in plan.features"
                                        :key="feature"
                                        class="flex items-start gap-2 text-sm"
                                    >
                                        <Check
                                            class="mt-0.5 h-4 w-4 shrink-0 text-[hsl(var(--brand))]"
                                        />
                                        <span>{{ feature }}</span>
                                    </li>
                                </ul>
                            </CardContent>

                            <CardFooter>
                                <template v-if="currentTier === plan.tier">
                                    <Button
                                        variant="outline"
                                        class="w-full"
                                        disabled
                                    >
                                        Current Plan
                                    </Button>
                                </template>
                                <template v-else-if="plan.tier === 'free'">
                                    <Button
                                        v-if="isAuthenticated"
                                        variant="outline"
                                        class="w-full"
                                        disabled
                                    >
                                        Free Forever
                                    </Button>
                                    <Button
                                        v-else
                                        variant="outline"
                                        class="w-full"
                                        as-child
                                    >
                                        <Link :href="register()">{{
                                            plan.cta
                                        }}</Link>
                                    </Button>
                                </template>
                                <template
                                    v-else-if="plan.tier === 'enterprise'"
                                >
                                    <Button
                                        variant="secondary"
                                        class="w-full"
                                        as-child
                                    >
                                        <a href="mailto:support@goformx.com">{{
                                            plan.cta
                                        }}</a>
                                    </Button>
                                </template>
                                <template
                                    v-else-if="isAuthenticated && priceId(plan)"
                                >
                                    <Button
                                        :variant="
                                            plan.highlighted
                                                ? 'brand'
                                                : 'default'
                                        "
                                        class="w-full"
                                        @click="handleSubscribe(plan)"
                                    >
                                        {{ plan.cta }}
                                    </Button>
                                </template>
                                <template v-else>
                                    <Button
                                        :variant="
                                            plan.highlighted
                                                ? 'brand'
                                                : 'default'
                                        "
                                        class="w-full"
                                        as-child
                                    >
                                        <Link :href="register()">{{
                                            plan.cta
                                        }}</Link>
                                    </Button>
                                </template>
                            </CardFooter>
                        </Card>
                    </div>
                </div>
            </section>
        </main>
        <PublicFooter />
    </div>
</template>
