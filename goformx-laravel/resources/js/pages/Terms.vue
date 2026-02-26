<script setup lang="ts">
import { Head } from '@inertiajs/vue3';
import PublicFooter from '@/components/PublicFooter.vue';
import PublicHeader from '@/components/PublicHeader.vue';
</script>

<template>
    <div class="flex min-h-screen flex-col bg-background text-foreground">
        <Head title="Terms of Service" />
        <PublicHeader />
        <main class="flex-1">
            <div class="container py-16">
                <h1 class="text-3xl font-semibold">Terms of Service</h1>
                <p class="mt-4 text-muted-foreground">Coming soon.</p>
            </div>
        </main>
        <PublicFooter />
    </div>
</template>
