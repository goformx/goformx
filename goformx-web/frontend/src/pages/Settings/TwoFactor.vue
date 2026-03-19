<script setup lang="ts">
import { Head } from '@inertiajs/vue3';
defineProps<{ twoFactor: { enabled: boolean } }>();
</script>
<template>
    <Head title="Two-Factor Authentication" />
    <div class="p-6">
        <h1 class="text-2xl font-bold">Two-Factor Authentication</h1>
        <p class="mt-2">{{ twoFactor.enabled ? 'Enabled' : 'Not enabled' }}</p>
    </div>
</template>
