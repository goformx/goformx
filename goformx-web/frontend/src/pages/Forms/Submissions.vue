<script setup lang="ts">
import { Head } from '@inertiajs/vue3';
defineProps<{ form: { id: string; title: string }; submissions: Array<{ id: string }> }>();
</script>
<template>
    <Head :title="`Submissions: ${form.title}`" />
    <div class="p-6">
        <h1 class="text-2xl font-bold">Submissions: {{ form.title }}</h1>
        <p class="text-muted-foreground mt-2">{{ submissions.length }} submissions</p>
    </div>
</template>
