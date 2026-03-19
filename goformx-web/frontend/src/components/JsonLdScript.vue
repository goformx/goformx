<script setup lang="ts">
/**
 * Renders a JSON-LD `<script>` tag via Vue's `h()` render function.
 * Vue strips `<script>` elements from `<template>` blocks, so a render
 * function is the only way to inject one into the document head.
 */
import { h } from 'vue';

const props = defineProps<{
    data: Record<string, unknown>;
}>();

function safeJsonLd(): string {
    try {
        return JSON.stringify(props.data).replace(/<\/script/gi, '<\\/script');
    } catch {
        return '{}';
    }
}
</script>

<template>
    <component
        :is="
            () =>
                h('script', {
                    type: 'application/ld+json',
                    innerHTML: safeJsonLd(),
                })
        "
    />
</template>
