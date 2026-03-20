<script setup lang="ts">
import { Head, Link } from '@inertiajs/vue3';
import { ArrowLeft, Eye, Inbox } from 'lucide-vue-next';
import { computed } from 'vue';
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
    schema?: { components?: Array<{ key: string; label: string; type: string }> };
}

const props = defineProps<{ form: Form; submissions: Submission[] }>();

const breadcrumbs: BreadcrumbItem[] = [
    { title: 'Dashboard', href: '/dashboard' },
    { title: 'Forms', href: '/forms' },
    { title: props.form.title, href: `/forms/${props.form.id}/edit` },
    { title: 'Submissions', href: `/forms/${props.form.id}/submissions` },
];

const columns = computed(() => {
    const components = props.form.schema?.components ?? [];
    return components
        .filter((c) => c.type !== 'button')
        .map((c) => ({ key: c.key, label: c.label }));
});

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
    if (typeof value === 'object') return JSON.stringify(value);
    return String(value);
}
</script>

<template>
    <Head :title="`Submissions: ${form.title}`" />

    <AppLayout :breadcrumbs="breadcrumbs">
        <div class="p-6">
            <div class="mb-6 flex items-center justify-between">
                <div>
                    <h1 class="text-2xl font-bold">{{ form.title }}</h1>
                    <p class="text-sm text-muted-foreground">
                        {{ submissions.length }} submission{{
                            submissions.length !== 1 ? 's' : ''
                        }}
                    </p>
                </div>
                <Button variant="outline" size="sm" as-child>
                    <Link :href="`/forms/${form.id}/edit`">
                        <ArrowLeft class="mr-1.5 h-3.5 w-3.5" />
                        Back to editor
                    </Link>
                </Button>
            </div>

            <div
                v-if="submissions.length === 0"
                class="flex flex-col items-center justify-center rounded-lg border border-dashed py-16"
            >
                <Inbox class="mb-3 h-10 w-10 text-muted-foreground/50" />
                <p class="text-sm text-muted-foreground">
                    No submissions yet
                </p>
            </div>

            <div
                v-else
                class="overflow-hidden rounded-lg border"
            >
                <table class="w-full text-sm">
                    <thead>
                        <tr class="border-b bg-muted/50">
                            <th
                                v-for="col in columns"
                                :key="col.key"
                                class="px-4 py-3 text-left font-medium text-muted-foreground"
                            >
                                {{ col.label }}
                            </th>
                            <th class="px-4 py-3 text-left font-medium text-muted-foreground">
                                Status
                            </th>
                            <th class="px-4 py-3 text-left font-medium text-muted-foreground">
                                Submitted
                            </th>
                            <th class="px-4 py-3 text-right font-medium text-muted-foreground">
                                <span class="sr-only">Actions</span>
                            </th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr
                            v-for="sub in submissions"
                            :key="sub.id"
                            class="border-b last:border-0 hover:bg-muted/30 transition-colors"
                        >
                            <td
                                v-for="col in columns"
                                :key="col.key"
                                class="px-4 py-3 max-w-[200px] truncate"
                                :title="formatValue(sub.data[col.key])"
                            >
                                {{ formatValue(sub.data[col.key]) }}
                            </td>
                            <td class="px-4 py-3">
                                <Badge
                                    :variant="sub.status === 'pending' ? 'secondary' : 'default'"
                                    class="text-xs"
                                >
                                    {{ sub.status }}
                                </Badge>
                            </td>
                            <td class="px-4 py-3 text-muted-foreground whitespace-nowrap">
                                {{ formatDate(sub.submitted_at) }}
                            </td>
                            <td class="px-4 py-3 text-right">
                                <Button variant="ghost" size="icon" class="h-7 w-7" as-child>
                                    <Link :href="`/forms/${form.id}/submissions/${sub.id}`">
                                        <Eye class="h-3.5 w-3.5" />
                                    </Link>
                                </Button>
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    </AppLayout>
</template>
