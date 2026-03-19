<script setup lang="ts">
import { Head } from '@inertiajs/vue3';
defineProps<{ form: { id: string; title: string } }>();
</script>
<template>
    <Head :title="`Embed: ${form.title}`" />
    <div class="p-6">
        <h1 class="text-2xl font-bold">Embed: {{ form.title }}</h1>
        <p class="text-muted-foreground mt-2">Embed code will be shown here.</p>
    </div>
</template>
