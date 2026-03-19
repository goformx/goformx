<script setup lang="ts">
import type { PageProps } from '@/types';
</script>
<template>
    <div class="min-h-screen bg-background">
        <nav class="border-b border-border bg-card px-6 py-3 flex items-center justify-between">
            <a href="/dashboard" class="font-bold text-lg">GoFormX</a>
            <div class="flex items-center gap-4 text-sm">
                <a href="/forms" class="text-muted-foreground hover:text-foreground">Forms</a>
                <a href="/billing" class="text-muted-foreground hover:text-foreground">Billing</a>
                <a href="/settings/profile" class="text-muted-foreground hover:text-foreground">Settings</a>
            </div>
        </nav>
        <slot />
    </div>
</template>
