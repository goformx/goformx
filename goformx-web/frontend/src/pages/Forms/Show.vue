<script setup lang="ts">
import { Head } from '@inertiajs/vue3';
defineProps<{ form: { id: string; title: string } }>();
</script>
<template>
    <Head :title="form.title" />
    <div class="p-6">
        <h1 class="text-2xl font-bold">{{ form.title }}</h1>
    </div>
</template>
