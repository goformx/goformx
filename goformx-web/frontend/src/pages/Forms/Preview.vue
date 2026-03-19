<script setup lang="ts">
import { Head } from '@inertiajs/vue3';
defineProps<{ form: { id: string; title: string; schema?: object } }>();
</script>
<template>
    <Head :title="`Preview: ${form.title}`" />
    <div class="p-6">
        <h1 class="text-2xl font-bold">Preview: {{ form.title }}</h1>
        <p class="text-muted-foreground mt-2">Form preview — read-only rendering of the form schema.</p>
    </div>
</template>
