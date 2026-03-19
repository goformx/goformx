<script setup lang="ts">
import { Head } from '@inertiajs/vue3';
defineProps<{ user: { name: string; email: string } }>();
</script>
<template>
    <Head title="Profile Settings" />
    <div class="p-6">
        <h1 class="text-2xl font-bold">Profile</h1>
        <p class="mt-2">{{ user.name }} ({{ user.email }})</p>
    </div>
</template>
