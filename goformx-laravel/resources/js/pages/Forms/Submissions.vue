<script setup lang="ts">
import { computed } from 'vue';
import { Head, Link } from '@inertiajs/vue3';
import AppLayout from '@/layouts/AppLayout.vue';
import { Button } from '@/components/ui/button';
import { Card, CardHeader } from '@/components/ui/card';
import { type BreadcrumbItem } from '@/types';
import { dashboard } from '@/routes';
import { index as formsIndex, edit } from '@/routes/forms';

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
    submissions: Submission[] | { submissions?: Submission[]; count?: number };
}>();

const formId = computed(() => props.form.id ?? props.form.ID ?? '');

const submissionList = computed((): Submission[] => {
    const s = props.submissions;
    if (Array.isArray(s)) return s;
    return s?.submissions ?? [];
});

const breadcrumbs = computed((): BreadcrumbItem[] => [
    { title: 'Dashboard', href: dashboard().url },
    { title: 'Forms', href: formsIndex.url() },
    { title: props.form.title ?? 'Form', href: formId.value ? edit.url({ id: formId.value }) : '#' },
    { title: 'Submissions', href: '#' },
]);

function submissionDetailUrl(submission: Submission): string {
    const sid = submission.id ?? submission.ID ?? '';
    return formId.value && sid ? `/forms/${formId.value}/submissions/${sid}` : '#';
}

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
</script>

<template>
    <Head :title="`Submissions: ${form.title ?? 'Form'}`" />

    <AppLayout :breadcrumbs="breadcrumbs">
        <div class="flex h-full flex-1 flex-col gap-4 overflow-x-auto rounded-xl p-4">
            <div class="flex items-center justify-between">
                <h1 class="text-xl font-semibold">Submissions</h1>
                <Button v-if="formId" variant="outline" as-child>
                    <Link :href="edit.url({ id: formId })">Edit form</Link>
                </Button>
            </div>

            <p v-if="submissionList.length === 0" class="text-muted-foreground">
                No submissions yet.
            </p>

            <div v-else class="grid gap-3">
                <Card
                    v-for="sub in submissionList"
                    :key="sub.id ?? sub.ID"
                    class="border-sidebar-border/70"
                >
                    <CardHeader class="flex flex-row items-center justify-between gap-2 py-3">
                        <div class="space-y-0.5">
                            <p class="text-sm font-medium">
                                {{ formatDate(sub.submitted_at) }}
                            </p>
                            <p class="text-xs text-muted-foreground capitalize">
                                {{ sub.status ?? '—' }}
                            </p>
                        </div>
                        <Button
                            v-if="submissionDetailUrl(sub) !== '#'"
                            variant="outline"
                            size="sm"
                            as-child
                        >
                            <Link :href="submissionDetailUrl(sub)">View</Link>
                        </Button>
                    </CardHeader>
                </Card>
            </div>
        </div>
    </AppLayout>
</template>
