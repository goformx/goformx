<script setup lang="ts">
defineProps<{
    showFieldsPanel?: boolean;
    showSettingsPanel?: boolean;
}>();
</script>

<template>
    <div class="flex h-full">
        <aside
            v-if="showFieldsPanel"
            class="w-64 shrink-0 overflow-y-auto border-r"
        >
            <slot name="fields-panel" />
        </aside>

        <div class="flex flex-1 flex-col overflow-hidden">
            <div v-if="$slots.header">
                <slot name="header" />
            </div>
            <div class="flex-1 overflow-y-auto">
                <slot name="canvas" />
            </div>
        </div>

        <aside
            v-if="showSettingsPanel"
            class="w-72 shrink-0 overflow-y-auto border-l"
        >
            <slot name="settings-panel" />
        </aside>
    </div>
</template>
