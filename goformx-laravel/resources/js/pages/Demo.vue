<script setup lang="ts">
import { Formio } from '@formio/js';
import goforms from '@goformx/formio';
import { Head, Link, usePage } from '@inertiajs/vue3';
import { AlertCircle, CheckCircle2 } from 'lucide-vue-next';
import { ref, computed, onMounted } from 'vue';
import PublicFooter from '@/components/PublicFooter.vue';
import PublicHeader from '@/components/PublicHeader.vue';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { register } from '@/routes';
import { index as formsIndex } from '@/routes/forms';

Formio.use(goforms);

const props = defineProps<{
    formId: string;
}>();

const page = usePage();
const seo = computed(() => page.props.seo);

const title = 'Demo – GoFormX';
const ogTitle = 'Demo – GoFormX';
const description =
    'Try the GoFormX demo form. Submit a test response and see how embedded forms work with the GoFormX API.';
const ogDescription =
    'Try the GoFormX demo form. Submit a test response and see how embedded forms work.';
const goFormsPublicUrl = computed(
    () => (page.props.goFormsPublicUrl as string) ?? '',
);

const schemaUrl = computed(() =>
    goFormsPublicUrl.value
        ? `${goFormsPublicUrl.value}/forms/${props.formId}/schema`
        : '',
);
const submitUrl = computed(() =>
    goFormsPublicUrl.value
        ? `${goFormsPublicUrl.value}/forms/${props.formId}/submit`
        : '',
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

        form.on(
            'submit',
            async (submission: { data?: Record<string, unknown> }) => {
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
                        const errors =
                            errData?.errors ?? errData?.data?.errors ?? [];
                        if (Array.isArray(errors)) {
                            form.emit('submitError', { errors });
                        } else if (typeof errors === 'object') {
                            form.emit('submitError', { errors });
                        }
                        return;
                    }
                    if (res.status === 429) {
                        errorMessage.value =
                            'Too many submissions. Please try again later.';
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
            },
        );

        status.value = 'form';
    } catch {
        status.value = 'error';
        errorMessage.value = 'Form temporarily unavailable.';
    }
});
</script>

<template>
    <div class="flex min-h-screen flex-col bg-background text-foreground">
        <Head :title="title">
            <meta name="description" :content="description" />
            <link rel="canonical" :href="seo.currentUrl" />
            <meta property="og:type" content="website" />
            <meta property="og:title" :content="ogTitle" />
            <meta property="og:description" :content="ogDescription" />
            <meta property="og:url" :content="seo.currentUrl" />
            <meta property="og:site_name" :content="page.props.name" />
            <meta
                v-if="seo.defaultOgImage"
                property="og:image"
                :content="seo.defaultOgImage"
            />
            <meta
                name="twitter:card"
                :content="
                    seo.defaultOgImage ? 'summary_large_image' : 'summary'
                "
            />
            <meta name="twitter:title" :content="ogTitle" />
            <meta name="twitter:description" :content="ogDescription" />
            <meta
                v-if="seo.defaultOgImage"
                name="twitter:image"
                :content="seo.defaultOgImage"
            />
        </Head>

        <PublicHeader />

        <main class="relative flex-1 p-6">
            <div
                class="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_70%_50%_at_50%_0%,hsl(var(--brand)/0.04),transparent_60%)]"
            />
            <div class="relative mx-auto max-w-2xl space-y-4">
                <Alert v-if="errorMessage" variant="destructive">
                    <AlertCircle class="h-4 w-4" />
                    <AlertDescription>{{ errorMessage }}</AlertDescription>
                </Alert>

                <div
                    v-if="status === 'success'"
                    class="rounded-xl border border-border bg-card p-8 text-center shadow-sm"
                >
                    <div
                        class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-[hsl(var(--brand)/0.12)] text-[hsl(var(--brand))]"
                    >
                        <CheckCircle2 class="h-8 w-8" />
                    </div>
                    <h2
                        class="font-display text-xl font-semibold tracking-tight"
                    >
                        Thank you
                    </h2>
                    <p class="mt-1 text-muted-foreground">
                        Your response has been submitted.
                    </p>
                </div>

                <div
                    v-show="status === 'loading' || status === 'form'"
                    class="rounded-xl border border-border bg-card p-6 shadow-sm"
                >
                    <div
                        v-if="status === 'loading'"
                        class="flex flex-col items-center justify-center gap-3 py-14"
                    >
                        <div
                            class="h-8 w-8 animate-pulse rounded-full bg-[hsl(var(--brand)/0.2)]"
                        />
                        <p class="text-sm text-muted-foreground">
                            Loading form…
                        </p>
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
        <PublicFooter />
    </div>
</template>
