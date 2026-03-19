<script setup lang="ts">
import { Head } from '@inertiajs/vue3';
defineProps<{ forms: Array<{ id: string; title: string }> }>();
</script>
<template>
    <Head title="Forms" />
    <div class="p-6">
        <h1 class="text-2xl font-bold">Your Forms</h1>
        <div v-if="forms.length === 0" class="mt-4 text-muted-foreground">No forms yet.</div>
        <ul v-else class="mt-4 space-y-2">
            <li v-for="form in forms" :key="form.id">{{ form.title }}</li>
        </ul>
    </div>
</template>
