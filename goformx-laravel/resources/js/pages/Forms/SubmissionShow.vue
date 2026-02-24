<script setup lang="ts">
import { Head, Link } from '@inertiajs/vue3';
import { computed } from 'vue';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import AppLayout from '@/layouts/AppLayout.vue';
import { dashboard } from '@/routes';
import { index as formsIndex, edit } from '@/routes/forms';
import { type BreadcrumbItem } from '@/types';

interface Form {
    id?: string;
    ID?: string;
    title?: string;
    [key: string]: unknown;
}

interface Submission {
    id?: string;
    form_id?: string;
    status?: string;
    submitted_at?: string;
    data?: Record<string, unknown>;
    [key: string]: unknown;
}

const props = defineProps<{
    form: Form;
    submission: Submission;
}>();

const formId = computed(() => props.form.id ?? props.form.ID ?? '');

const breadcrumbs = computed((): BreadcrumbItem[] => [
    { title: 'Dashboard', href: dashboard().url },
    { title: 'Forms', href: formsIndex.url() },
    {
        title: props.form.title ?? 'Form',
        href: formId.value ? edit.url({ id: formId.value }) : '#',
    },
    {
        title: 'Submissions',
        href: formId.value ? `/forms/${formId.value}/submissions` : '#',
    },
    { title: 'Submission', href: '#' },
]);

const dataEntries = computed((): [string, unknown][] => {
    const data = props.submission.data;
    if (data && typeof data === 'object' && !Array.isArray(data)) {
        return Object.entries(data);
    }
    return [];
});

function formatDate(value: string | undefined): string {
    if (!value) return '—';
    try {
        return new Date(value).toLocaleString(undefined, {
            dateStyle: 'medium',
            timeStyle: 'short',
        });
    } catch {
        return value;
    }
}

function formatValue(value: unknown): string {
    if (value === null || value === undefined) return '—';
    if (typeof value === 'object') return JSON.stringify(value);
    return String(value);
}
</script>

<template>
    <Head :title="`Submission: ${form.title ?? 'Form'}`" />

    <AppLayout :breadcrumbs="breadcrumbs">
        <div
            class="flex h-full flex-1 flex-col gap-4 overflow-x-auto rounded-xl p-4"
        >
            <div class="flex items-center justify-between">
                <h1 class="text-xl font-semibold">Submission</h1>
                <Button v-if="formId" variant="outline" as-child>
                    <Link :href="`/forms/${formId}/submissions`"
                        >Back to submissions</Link
                    >
                </Button>
            </div>

            <Card class="border-sidebar-border/70">
                <CardHeader class="pb-2">
                    <p class="text-sm text-muted-foreground">
                        Submitted {{ formatDate(submission.submitted_at) }}
                    </p>
                    <p
                        v-if="submission.status"
                        class="text-xs text-muted-foreground capitalize"
                    >
                        Status: {{ submission.status }}
                    </p>
                </CardHeader>
                <CardContent class="space-y-3 pt-0">
                    <template v-if="dataEntries.length > 0">
                        <div
                            v-for="[key, value] in dataEntries"
                            :key="key"
                            class="flex flex-col gap-0.5 border-b border-border/50 pb-2 last:border-0 last:pb-0"
                        >
                            <span
                                class="text-xs font-medium text-muted-foreground"
                                >{{ key }}</span
                            >
                            <span class="text-sm">{{
                                formatValue(value)
                            }}</span>
                        </div>
                    </template>
                    <p v-else class="text-sm text-muted-foreground">
                        No submission data.
                    </p>
                </CardContent>
            </Card>
        </div>
    </AppLayout>
</template>
