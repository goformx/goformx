<script setup lang="ts">
import { h } from 'vue';

const props = defineProps<{
    data: Record<string, unknown>;
}>();
</script>

<template>
    <component
        :is="() => h('script', { type: 'application/ld+json' }, JSON.stringify(props.data))"
    />
</template>
