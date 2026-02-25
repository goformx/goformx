<script setup lang="ts">
import { Head, Link } from '@inertiajs/vue3';
import { CreditCard } from 'lucide-vue-next';
import { computed } from 'vue';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import AppLayout from '@/layouts/AppLayout.vue';
import { pricing } from '@/routes';
import { portal } from '@/routes/billing';
import type { BreadcrumbItem } from '@/types';
import type { BillingUsage, SubscriptionInfo } from '@/types/billing';

const props = defineProps<{
    currentTier: string;
    subscription: SubscriptionInfo | null;
    usage: BillingUsage;
    prices: Record<string, string | null>;
}>();

const breadcrumbs: BreadcrumbItem[] = [
    { title: 'Billing', href: '/billing' },
];

const tierLabels: Record<string, string> = {
    free: 'Free',
    pro: 'Pro',
    business: 'Business',
    enterprise: 'Enterprise',
};

const tierLimits: Record<string, { forms: number; submissions: number }> = {
    free: { forms: 3, submissions: 100 },
    pro: { forms: 25, submissions: 2500 },
    business: { forms: 100, submissions: 25000 },
    enterprise: { forms: -1, submissions: -1 },
};

const limits = computed(() => tierLimits[props.currentTier] ?? tierLimits.free);
const isUnlimited = computed(() => limits.value.forms === -1);

const statusLabel = computed(() => {
    if (!props.subscription?.stripe_status) return 'No subscription';
    const labels: Record<string, string> = {
        active: 'Active',
        trialing: 'Trial',
        past_due: 'Past Due',
        canceled: 'Canceled',
        incomplete: 'Incomplete',
    };
    return labels[props.subscription.stripe_status] ?? props.subscription.stripe_status;
});

const statusVariant = computed<'default' | 'secondary' | 'destructive' | 'outline'>(() => {
    if (!props.subscription?.stripe_status) return 'secondary';
    const variants: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
        active: 'default',
        trialing: 'secondary',
        past_due: 'destructive',
        canceled: 'outline',
        incomplete: 'destructive',
    };
    return variants[props.subscription.stripe_status] ?? 'secondary';
});

function formatNumber(n: number): string {
    return n.toLocaleString();
}

function usagePercent(current: number, max: number): number {
    if (max <= 0) return 0;
    return Math.min(100, Math.round((current / max) * 100));
}
</script>

<template>
    <Head title="Billing" />

    <AppLayout :breadcrumbs="breadcrumbs">
        <div class="flex flex-col gap-6 p-4 md:p-6">
            <!-- Plan overview -->
            <Card>
                <CardHeader>
                    <div class="flex items-center justify-between">
                        <div class="flex items-center gap-3">
                            <CreditCard class="h-5 w-5 text-muted-foreground" />
                            <div>
                                <CardTitle>{{ tierLabels[currentTier] ?? currentTier }} Plan</CardTitle>
                                <CardDescription>Your current subscription plan</CardDescription>
                            </div>
                        </div>
                        <Badge :variant="statusVariant">{{ statusLabel }}</Badge>
                    </div>
                </CardHeader>
                <CardContent>
                    <div class="flex flex-wrap gap-3">
                        <Button variant="outline" as-child>
                            <Link :href="pricing()">Change Plan</Link>
                        </Button>
                        <Button
                            v-if="subscription"
                            variant="outline"
                            as-child
                        >
                            <Link :href="portal()">Manage Billing</Link>
                        </Button>
                    </div>
                </CardContent>
            </Card>

            <!-- Usage -->
            <div class="grid gap-6 md:grid-cols-2">
                <!-- Forms usage -->
                <Card>
                    <CardHeader>
                        <CardTitle class="text-base">Forms</CardTitle>
                        <CardDescription>
                            <template v-if="isUnlimited">
                                {{ formatNumber(usage.forms) }} created (unlimited)
                            </template>
                            <template v-else>
                                {{ formatNumber(usage.forms) }} of {{ formatNumber(limits.forms) }} used
                            </template>
                        </CardDescription>
                    </CardHeader>
                    <CardContent v-if="!isUnlimited">
                        <div class="h-2 w-full overflow-hidden rounded-full bg-muted">
                            <div
                                class="h-full rounded-full bg-[hsl(var(--brand))] transition-all"
                                :style="{ width: `${usagePercent(usage.forms, limits.forms)}%` }"
                            />
                        </div>
                    </CardContent>
                </Card>

                <!-- Submissions usage -->
                <Card>
                    <CardHeader>
                        <CardTitle class="text-base">Submissions</CardTitle>
                        <CardDescription>
                            <template v-if="isUnlimited">
                                {{ formatNumber(usage.submissions) }} this month (unlimited)
                            </template>
                            <template v-else>
                                {{ formatNumber(usage.submissions) }} of {{ formatNumber(limits.submissions) }} this month
                            </template>
                        </CardDescription>
                    </CardHeader>
                    <CardContent v-if="!isUnlimited">
                        <div class="h-2 w-full overflow-hidden rounded-full bg-muted">
                            <div
                                class="h-full rounded-full bg-[hsl(var(--brand))] transition-all"
                                :style="{ width: `${usagePercent(usage.submissions, limits.submissions)}%` }"
                            />
                        </div>
                    </CardContent>
                </Card>
            </div>
        </div>
    </AppLayout>
</template>
