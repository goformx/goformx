<script setup lang="ts">
import { Head } from '@inertiajs/vue3';
defineProps<{ tier: string; is_paid: boolean; usage: { forms_count: number } }>();
</script>
<template>
    <Head title="Billing" />
    <div class="p-6">
        <h1 class="text-2xl font-bold">Billing</h1>
        <p class="mt-2">Current plan: <strong>{{ tier }}</strong></p>
        <p class="text-muted-foreground">Forms: {{ usage.forms_count }}</p>
    </div>
</template>
