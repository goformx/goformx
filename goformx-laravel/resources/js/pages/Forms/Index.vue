<script setup lang="ts">
import { computed, ref } from 'vue';
import { Head, router } from '@inertiajs/vue3';
import AppLayout from '@/layouts/AppLayout.vue';
import FormCard from '@/components/FormCard.vue';
import { Button } from '@/components/ui/button';
import { Plus } from 'lucide-vue-next';
import { type BreadcrumbItem } from '@/types';
import { dashboard } from '@/routes';
import { index as formsIndex, store as formsStore } from '@/routes/forms';

interface Form {
    id?: string;
    ID?: string;
    title?: string;
    description?: string;
    status?: string;
    updated_at?: string;
    [key: string]: unknown;
}

interface Props {
    forms: Form[] | { forms: Form[] };
}

const props = defineProps<Props>();

const formList = computed(() => {
    const f = props.forms;
    return Array.isArray(f) ? f : (f?.forms ?? []);
});

const breadcrumbs: BreadcrumbItem[] = [
    { title: 'Dashboard', href: dashboard().url },
    { title: 'Forms', href: formsIndex.url() },
];

const isCreating = ref(false);

function createForm() {
    if (isCreating.value) return;
    isCreating.value = true;
    router.post(formsStore.url(), { title: 'Untitled Form' }, {
        preserveScroll: true,
        onFinish: () => { isCreating.value = false; },
    });
}
</script>

<template>
    <Head title="Forms" />

    <AppLayout :breadcrumbs="breadcrumbs">
        <div class="flex h-full flex-1 flex-col gap-4 overflow-x-auto rounded-xl p-4">
            <div class="flex items-center justify-between">
                <h1 class="text-xl font-semibold">Forms</h1>
                <Button
                    :disabled="isCreating"
                    @click="createForm"
                >
                    <Plus class="size-4" />
                    New form
                </Button>
            </div>
            <div
                v-if="formList.length === 0"
                class="rounded-xl border border-sidebar-border/70 p-8 text-center text-muted-foreground"
            >
                No forms yet.
            </div>
            <div v-else class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                <FormCard
                    v-for="form in formList"
                    :key="form.id ?? form.ID"
                    :form="form"
                />
            </div>
        </div>
    </AppLayout>
</template>
