<script setup lang="ts">
import { Head } from '@inertiajs/vue3';
import AuthLayout from '@/layouts/auth/AuthSimpleLayout.vue';

defineProps<{
    title?: string;
    description?: string;
}>();
</script>

<template>
    <!-- Auth pages must not be indexed -->
    <Head>
        <meta name="robots" content="noindex, nofollow" />
    </Head>
    <AuthLayout :title="title" :description="description">
        <slot />
    </AuthLayout>
</template>
