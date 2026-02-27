<script setup lang="ts">
import { Head } from '@inertiajs/vue3';

defineProps<{
    title: string;
    content: string;
    slug: string;
    navigation: Array<{
        title: string;
        slug: string;
        order: number;
        active: boolean;
    }>;
}>();
</script>

<template>
    <div>
        <Head :title="title" />
        <h1>{{ title }}</h1>
        <div v-html="content" />
    </div>
</template>
