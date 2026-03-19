<script setup lang="ts">
import { Head } from '@inertiajs/vue3';
import { onMounted, ref } from 'vue';

const props = defineProps<{ formId: string }>();
const formContainer = ref<HTMLElement>();
const loaded = ref(false);
const error = ref('');

onMounted(async () => {
    try {
        const { Formio } = await import('@formio/js');
        const goforms = (await import('@goformx/formio')).default;
        Formio.use(goforms);

        const schemaUrl = `/api/forms/${props.formId}/schema`;
        // Fetch schema from Go public API
        const response = await fetch(schemaUrl);
        if (!response.ok) throw new Error('Form not found');
        const data = await response.json();

        if (formContainer.value) {
            await Formio.createForm(formContainer.value, data.data?.schema || {}, {
                readOnly: false,
            });
        }
        loaded.value = true;
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to load form';
    }
});
</script>
<template>
    <Head :title="`Form`" />
    <div class="max-w-3xl mx-auto p-6">
        <div v-if="error" class="text-red-600">{{ error }}</div>
        <div v-else ref="formContainer"></div>
    </div>
</template>
