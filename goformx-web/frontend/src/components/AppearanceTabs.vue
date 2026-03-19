<script setup lang="ts">
import { Monitor, Moon, Sun } from 'lucide-vue-next';
import { useAppearance } from '@/composables/useAppearance';

const { appearance, updateAppearance } = useAppearance();

const tabs = [
    { value: 'light', label: 'Light', icon: Sun },
    { value: 'dark', label: 'Dark', icon: Moon },
    { value: 'system', label: 'System', icon: Monitor },
] as const;
</script>

<template>
    <div class="flex flex-wrap gap-4">
        <button
            v-for="tab in tabs"
            :key="tab.value"
            class="flex flex-col items-center gap-2 rounded-lg border-2 p-4 transition-colors"
            :class="
                appearance === tab.value
                    ? 'border-primary bg-muted'
                    : 'border-transparent hover:bg-muted/50'
            "
            @click="updateAppearance(tab.value)"
        >
            <component :is="tab.icon" class="h-6 w-6" />
            <span class="text-sm font-medium">{{ tab.label }}</span>
        </button>
    </div>
</template>
