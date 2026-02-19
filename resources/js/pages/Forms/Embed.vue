<script setup lang="ts">
import { computed, ref } from 'vue';
import { Head, Link } from '@inertiajs/vue3';
import AppLayout from '@/layouts/AppLayout.vue';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { type BreadcrumbItem } from '@/types';
import { dashboard } from '@/routes';
import { index as formsIndex, edit } from '@/routes/forms';
import { Check, Copy } from 'lucide-vue-next';

interface Form {
    id?: string;
    ID?: string;
    title?: string;
    [key: string]: unknown;
}

const props = defineProps<{
    form: Form;
    embed_base_url: string;
}>();

const formId = computed(() => props.form.id ?? props.form.ID ?? '');

const embedUrl = computed(() => {
    const base = (props.embed_base_url ?? '').replace(/\/$/, '');
    return formId.value ? `${base}/forms/${formId.value}/embed` : '';
});

const iframeSnippet = computed(() => {
    if (!embedUrl.value) return '';
    return `<iframe src="${embedUrl.value}" width="100%" height="500" frameborder="0" title="${(props.form.title ?? 'Form').replace(/"/g, '&quot;')}"></iframe>`;
});

const breadcrumbs = computed((): BreadcrumbItem[] => [
    { title: 'Dashboard', href: dashboard().url },
    { title: 'Forms', href: formsIndex.url() },
    { title: props.form.title ?? 'Form', href: formId.value ? edit.url({ id: formId.value }) : '#' },
    { title: 'Embed', href: '#' },
]);

const copied = ref<'url' | 'iframe' | null>(null);

async function copyToClipboard(text: string, type: 'url' | 'iframe') {
    try {
        await navigator.clipboard.writeText(text);
        copied.value = type;
        setTimeout(() => { copied.value = null; }, 2000);
    } catch {
        // ignore
    }
}
</script>

<template>
    <Head :title="`Embed: ${form.title ?? 'Form'}`" />

    <AppLayout :breadcrumbs="breadcrumbs">
        <div class="flex h-full flex-1 flex-col gap-4 overflow-x-auto rounded-xl p-4">
            <div class="flex items-center justify-between">
                <h1 class="text-xl font-semibold">Embed</h1>
                <Button v-if="formId" variant="outline" as-child>
                    <Link :href="edit.url({ id: formId })">Edit form</Link>
                </Button>
            </div>

            <Card class="border-sidebar-border/70">
                <CardHeader>
                    <CardTitle class="text-base">Embed URL</CardTitle>
                    <CardDescription>
                        Use this URL to load the form in an iframe on your site.
                    </CardDescription>
                </CardHeader>
                <CardContent class="flex gap-2">
                    <Input
                        :model-value="embedUrl"
                        readonly
                        class="font-mono text-sm"
                    />
                    <Button
                        variant="outline"
                        size="icon"
                        :title="copied === 'url' ? 'Copied' : 'Copy URL'"
                        @click="copyToClipboard(embedUrl, 'url')"
                    >
                        <Check v-if="copied === 'url'" class="h-4 w-4" />
                        <Copy v-else class="h-4 w-4" />
                    </Button>
                </CardContent>
            </Card>

            <Card class="border-sidebar-border/70">
                <CardHeader>
                    <CardTitle class="text-base">Iframe snippet</CardTitle>
                    <CardDescription>
                        Paste this HTML where you want the form to appear.
                    </CardDescription>
                </CardHeader>
                <CardContent class="flex flex-col gap-2">
                    <pre class="overflow-x-auto rounded-md border border-border bg-muted/50 p-3 text-xs font-mono">{{ iframeSnippet }}</pre>
                    <Button
                        variant="outline"
                        size="sm"
                        class="w-fit"
                        @click="copyToClipboard(iframeSnippet, 'iframe')"
                    >
                        <Check v-if="copied === 'iframe'" class="mr-2 h-4 w-4" />
                        <Copy v-else class="mr-2 h-4 w-4" />
                        {{ copied === 'iframe' ? 'Copied' : 'Copy snippet' }}
                    </Button>
                </CardContent>
            </Card>
        </div>
    </AppLayout>
</template>
