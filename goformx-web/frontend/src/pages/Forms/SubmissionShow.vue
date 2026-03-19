<script setup lang="ts">
import { Head } from '@inertiajs/vue3';
defineProps<{ form: { id: string; title: string }; submission: Record<string, unknown> }>();
</script>
<template>
    <Head :title="`Submission — ${form.title}`" />
    <div class="p-6">
        <h1 class="text-2xl font-bold">Submission — {{ form.title }}</h1>
        <pre class="mt-4 bg-muted p-4 rounded text-sm overflow-auto">{{ JSON.stringify(submission, null, 2) }}</pre>
    </div>
</template>
