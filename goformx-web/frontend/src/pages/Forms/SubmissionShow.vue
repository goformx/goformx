<script setup lang="ts">
import { Head, Link } from '@inertiajs/vue3';
import { ArrowLeft } from 'lucide-vue-next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import AppLayout from '@/layouts/AppLayout.vue';
import { type BreadcrumbItem } from '@/types';

interface Submission {
    id: string;
    data: Record<string, unknown>;
    status: string;
    submitted_at: string;
}

interface Form {
    id: string;
    title: string;
}

const props = defineProps<{ form: Form; submission: Submission }>();

const breadcrumbs: BreadcrumbItem[] = [
    { title: 'Dashboard', href: '/dashboard' },
    { title: 'Forms', href: '/forms' },
    { title: props.form.title, href: `/forms/${props.form.id}/edit` },
    { title: 'Submissions', href: `/forms/${props.form.id}/submissions` },
    { title: props.submission.id.slice(0, 8), href: '#' },
];

function formatDate(iso: string): string {
    return new Date(iso).toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
    });
}

function formatValue(value: unknown): string {
    if (value === null || value === undefined || value === '') return '—';
    if (typeof value === 'object') return JSON.stringify(value, null, 2);
    return String(value);
}
</script>

<template>
    <Head :title="`Submission — ${form.title}`" />

    <AppLayout :breadcrumbs="breadcrumbs">
        <div class="p-6 max-w-2xl">
            <div class="mb-6 flex items-center justify-between">
                <div>
                    <h1 class="text-2xl font-bold">Submission</h1>
                    <p class="text-sm text-muted-foreground mt-1">
                        {{ form.title }} &middot;
                        <Badge
                            :variant="submission.status === 'pending' ? 'secondary' : 'default'"
                            class="text-xs"
                        >
                            {{ submission.status }}
                        </Badge>
                        &middot; {{ formatDate(submission.submitted_at) }}
                    </p>
                </div>
                <Button variant="outline" size="sm" as-child>
                    <Link :href="`/forms/${form.id}/submissions`">
                        <ArrowLeft class="mr-1.5 h-3.5 w-3.5" />
                        All submissions
                    </Link>
                </Button>
            </div>

            <div class="rounded-lg border divide-y">
                <div
                    v-for="(value, key) in submission.data"
                    :key="String(key)"
                    class="flex px-4 py-3 gap-4"
                >
                    <dt class="w-1/3 text-sm font-medium text-muted-foreground shrink-0">
                        {{ key }}
                    </dt>
                    <dd class="text-sm break-words">
                        {{ formatValue(value) }}
                    </dd>
                </div>
            </div>

            <p class="mt-4 text-xs text-muted-foreground">
                ID: {{ submission.id }}
            </p>
        </div>
    </AppLayout>
</template>
