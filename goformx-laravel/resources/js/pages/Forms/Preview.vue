<script setup lang="ts">
import { Formio } from '@formio/js';
import goforms from '@goformx/formio';
import { Head, Link } from '@inertiajs/vue3';
import { AlertCircle, Pencil } from 'lucide-vue-next';
import { ref, computed, onMounted } from 'vue';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import AppLayout from '@/layouts/AppLayout.vue';
import { dashboard } from '@/routes';
import { index as formsIndex, edit } from '@/routes/forms';
import { type BreadcrumbItem } from '@/types';

Formio.use(goforms);

interface Form {
    id?: string;
    ID?: string;
    title?: string;
    description?: string;
    schema?: { display?: string; components?: unknown[] };
    [key: string]: unknown;
}

const props = defineProps<{
    form: Form;
}>();

const formId = computed(() => props.form.id ?? props.form.ID ?? '');

const breadcrumbs = computed((): BreadcrumbItem[] => [
    { title: 'Dashboard', href: dashboard().url },
    { title: 'Forms', href: formsIndex.url() },
    { title: props.form.title ?? 'Form', href: formId.value ? edit.url({ id: formId.value }) : '#' },
    { title: 'Preview', href: '#' },
]);

const isLoading = ref(true);
const error = ref<string | null>(null);

onMounted(async () => {
    const container = document.getElementById('form-preview-container');
    if (!container) {
        error.value = 'Preview container not found';
        isLoading.value = false;
        return;
    }

    const schema = props.form.schema;
    if (!schema || !schema.components || schema.components.length === 0) {
        error.value = 'This form has no fields yet. Add fields in the form builder.';
        isLoading.value = false;
        return;
    }

    try {
        await Formio.createForm(container, schema, {
            readOnly: true,
            noSubmit: true,
            noAlerts: true,
        });
    } catch (err) {
        console.error('Failed to load form preview:', err);
        error.value = 'Failed to load form preview';
    } finally {
        isLoading.value = false;
    }
});
</script>

<template>
    <Head :title="`Preview: ${form.title ?? 'Form'}`" />

    <AppLayout :breadcrumbs="breadcrumbs">
        <div class="flex h-full flex-1 flex-col gap-4 overflow-x-auto rounded-xl p-4">
            <div class="flex items-center justify-between">
                <h1 class="text-xl font-semibold">Preview</h1>
                <Button v-if="formId" variant="outline" as-child>
                    <Link :href="edit.url({ id: formId })">
                        <Pencil class="mr-2 h-4 w-4" />
                        Edit form
                    </Link>
                </Button>
            </div>

            <Alert v-if="error" variant="destructive">
                <AlertCircle class="h-4 w-4" />
                <AlertDescription>{{ error }}</AlertDescription>
            </Alert>

            <div
                v-if="isLoading"
                class="flex flex-col items-center justify-center gap-3 py-14 text-muted-foreground"
            >
                <div class="h-8 w-8 animate-pulse rounded-full bg-muted" />
                <p class="text-sm">Loading form…</p>
            </div>

            <div
                v-show="!isLoading && !error"
                class="max-w-2xl rounded-xl border border-border bg-card p-6 shadow-sm"
            >
                <div id="form-preview-container" class="min-h-[200px]" />
            </div>
        </div>
    </AppLayout>
</template>
