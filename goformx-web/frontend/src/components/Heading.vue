<script setup lang="ts">
defineProps<{
    variant?: 'default' | 'small';
    title: string;
    description?: string;
}>();
</script>

<template>
    <div>
        <h3
            :class="
                variant === 'small'
                    ? 'text-base font-medium'
                    : 'text-lg font-medium'
            "
        >
            {{ title }}
        </h3>
        <p v-if="description" class="mt-0.5 text-sm text-muted-foreground">
            {{ description }}
        </p>
    </div>
</template>
