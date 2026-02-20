<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { Head, Link, usePage } from '@inertiajs/vue3';
import { Formio } from '@formio/js';
import goforms from '@goformx/formio';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { AlertCircle, CheckCircle2 } from 'lucide-vue-next';
import { dashboard, login, register } from '@/routes';
import { index as formsIndex } from '@/routes/forms';

Formio.use(goforms);

const props = defineProps<{
    formId: string;
}>();

const page = usePage();
const goFormsPublicUrl = computed(() => (page.props.goFormsPublicUrl as string) ?? '');

const schemaUrl = computed(
    () => (goFormsPublicUrl ? `${goFormsPublicUrl}/forms/${props.formId}/schema` : ''),
);
const submitUrl = computed(
    () => (goFormsPublicUrl ? `${goFormsPublicUrl}/forms/${props.formId}/submit` : ''),
);

const status = ref<'loading' | 'form' | 'success' | 'error'>('loading');
const errorMessage = ref<string | null>(null);
const formInstance = ref<unknown>(null);

onMounted(async () => {
    const container = document.getElementById('demo-form-container');
    if (!container || !schemaUrl.value || !submitUrl.value) {
        status.value = 'error';
        errorMessage.value = 'Form temporarily unavailable.';
        return;
    }

    try {
        const response = await fetch(schemaUrl.value);
        if (response.status === 404) {
            status.value = 'error';
            errorMessage.value = 'Form not found.';
            return;
        }
        if (!response.ok) {
            status.value = 'error';
            errorMessage.value = 'Form temporarily unavailable.';
            return;
        }

        const data = await response.json();
        const schema = data?.data ?? data;
        if (!schema?.components?.length) {
            status.value = 'error';
            errorMessage.value = 'Form not found.';
            return;
        }

        const form = await Formio.createForm(container, schema, {
            readOnly: false,
            noAlerts: true,
            noSubmit: true,
        });
        formInstance.value = form;

        form.on('submit', async (submission: { data?: Record<string, unknown> }) => {
            errorMessage.value = null;
            try {
                const res = await fetch(submitUrl.value, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(submission?.data ?? submission),
                });

                if (res.ok) {
                    status.value = 'success';
                    return;
                }
                if (res.status === 422) {
                    const errData = await res.json().catch(() => ({}));
                    const errors = errData?.errors ?? errData?.data?.errors ?? [];
                    if (Array.isArray(errors)) {
                        form.emit('submitError', { errors });
                    } else if (typeof errors === 'object') {
                        form.emit('submitError', { errors });
                    }
                    return;
                }
                if (res.status === 429) {
                    errorMessage.value = 'Too many submissions. Please try again later.';
                    return;
                }
                if (res.status === 404) {
                    errorMessage.value = 'Form no longer available.';
                    return;
                }
                errorMessage.value = 'Submission failed. Please try again.';
            } catch {
                errorMessage.value = 'Submission failed. Please try again.';
            }
        });

        status.value = 'form';
    } catch {
        status.value = 'error';
        errorMessage.value = 'Form temporarily unavailable.';
    }
});
</script>

<template>
    <div
        class="flex min-h-screen flex-col bg-background text-foreground"
    >
        <Head title="Demo" />

        <header
            class="w-full border-b border-border/50 bg-background/80 backdrop-blur-sm"
        >
            <nav
                class="container flex items-center justify-end gap-4 px-4 py-4 sm:px-6"
            >
                <Link
                    v-if="$page.props.auth?.user"
                    :href="dashboard()"
                    class="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
                >
                    Dashboard
                </Link>
                <template v-else>
                    <Link
                        :href="login()"
                        class="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
                    >
                        Log in
                    </Link>
                    <Link
                        :href="register()"
                        class="rounded-md border border-border bg-background px-4 py-2 text-sm font-medium shadow-sm transition-colors hover:bg-muted/50"
                    >
                        Register
                    </Link>
                </template>
            </nav>
        </header>

        <main class="container flex flex-1 flex-col px-4 py-8">
            <div class="mx-auto w-full max-w-2xl space-y-4">
                <Alert v-if="errorMessage" variant="destructive">
                    <AlertCircle class="h-4 w-4" />
                    <AlertDescription>{{ errorMessage }}</AlertDescription>
                </Alert>

                <div
                    v-if="status === 'success'"
                    class="rounded-lg border border-border bg-card p-6 text-center"
                >
                    <CheckCircle2 class="mx-auto mb-2 h-12 w-12 text-green-600" />
                    <h2 class="text-lg font-semibold">Thanks, we've received your response.</h2>
                    <p class="mt-1 text-muted-foreground">
                        Your submission has been saved.
                    </p>
                </div>

                <div
                    v-show="status === 'loading' || status === 'form'"
                    class="rounded-lg border border-border bg-card p-6"
                >
                    <div
                        v-if="status === 'loading'"
                        class="py-12 text-center text-muted-foreground"
                    >
                        Loading form…
                    </div>
                    <div id="demo-form-container" class="min-h-[200px]" />
                </div>

                <p class="text-center text-sm text-muted-foreground">
                    <Link
                        v-if="$page.props.auth?.user"
                        :href="formsIndex.url()"
                        class="font-medium text-primary underline-offset-4 hover:underline"
                    >
                        Create your own form
                    </Link>
                    <template v-else>
                        <Link
                            :href="register()"
                            class="font-medium text-primary underline-offset-4 hover:underline"
                        >
                            Create your own form
                        </Link>
                    </template>
                </p>
            </div>
        </main>
    </div>
</template>
