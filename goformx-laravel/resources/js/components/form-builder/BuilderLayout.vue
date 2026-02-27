<script setup lang="ts">
import { ChevronLeft, ChevronRight } from 'lucide-vue-next';
import { ref, onMounted } from 'vue';
import { Button } from '@/components/ui/button';
import { Logger } from '@/lib/logger';

interface Props {
    showFieldsPanel?: boolean;
    showSettingsPanel?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
    showFieldsPanel: true,
    showSettingsPanel: true,
});

// Panel collapse state (persisted in localStorage)
const isFieldsPanelCollapsed = ref(false);
const isSettingsPanelCollapsed = ref(false);

onMounted(() => {
    try {
        const savedFieldsState = localStorage.getItem(
            'builder-fields-panel-collapsed',
        );
        const savedSettingsState = localStorage.getItem(
            'builder-settings-panel-collapsed',
        );

        if (savedFieldsState !== null) {
            isFieldsPanelCollapsed.value = savedFieldsState === 'true';
        }
        if (savedSettingsState !== null) {
            isSettingsPanelCollapsed.value = savedSettingsState === 'true';
        }
    } catch (error) {
        Logger.error('Failed to load panel state:', error);
    }
});

function toggleFieldsPanel() {
    isFieldsPanelCollapsed.value = !isFieldsPanelCollapsed.value;
    try {
        localStorage.setItem(
            'builder-fields-panel-collapsed',
            String(isFieldsPanelCollapsed.value),
        );
    } catch (error) {
        Logger.error('Failed to save fields panel state:', error);
    }
}

function toggleSettingsPanel() {
    isSettingsPanelCollapsed.value = !isSettingsPanelCollapsed.value;
    try {
        localStorage.setItem(
            'builder-settings-panel-collapsed',
            String(isSettingsPanelCollapsed.value),
        );
    } catch (error) {
        Logger.error('Failed to save settings panel state:', error);
    }
}
</script>

<template>
    <div class="builder-layout flex h-full flex-col">
        <div
            v-if="$slots['header']"
            class="builder-header border-b bg-background"
        >
            <slot name="header" />
        </div>

        <div class="builder-content flex flex-1 overflow-hidden">
            <div
                v-if="props.showFieldsPanel"
                :class="[
                    'builder-fields-panel border-r bg-background transition-all duration-200',
                    isFieldsPanelCollapsed ? 'w-0' : 'w-64',
                ]"
            >
                <div
                    v-if="!isFieldsPanelCollapsed"
                    class="flex h-full flex-col"
                >
                    <div
                        class="flex items-center justify-between border-b px-4 py-3"
                    >
                        <h3 class="text-sm font-semibold">Fields</h3>
                        <Button
                            variant="ghost"
                            size="icon"
                            class="h-6 w-6"
                            @click="toggleFieldsPanel"
                        >
                            <ChevronLeft class="h-4 w-4" />
                        </Button>
                    </div>
                    <div class="flex-1 overflow-hidden">
                        <slot name="fields-panel" />
                    </div>
                </div>
            </div>

            <div
                v-if="props.showFieldsPanel && isFieldsPanelCollapsed"
                class="flex items-start border-r px-1 pt-3"
            >
                <Button
                    variant="ghost"
                    size="icon"
                    class="h-6 w-6"
                    @click="toggleFieldsPanel"
                >
                    <ChevronRight class="h-4 w-4" />
                </Button>
            </div>

            <div class="builder-canvas flex-1 overflow-auto bg-muted/10">
                <slot name="canvas" />
            </div>

            <div
                v-if="props.showSettingsPanel && isSettingsPanelCollapsed"
                class="flex items-start border-l px-1 pt-3"
            >
                <Button
                    variant="ghost"
                    size="icon"
                    class="h-6 w-6"
                    @click="toggleSettingsPanel"
                >
                    <ChevronLeft class="h-4 w-4" />
                </Button>
            </div>

            <div
                v-if="props.showSettingsPanel"
                :class="[
                    'builder-settings-panel border-l bg-background transition-all duration-200',
                    isSettingsPanelCollapsed ? 'w-0' : 'w-80',
                ]"
            >
                <div
                    v-if="!isSettingsPanelCollapsed"
                    class="flex h-full flex-col"
                >
                    <div
                        class="flex items-center justify-between border-b px-4 py-3"
                    >
                        <h3 class="text-sm font-semibold">Settings</h3>
                        <Button
                            variant="ghost"
                            size="icon"
                            class="h-6 w-6"
                            @click="toggleSettingsPanel"
                        >
                            <ChevronRight class="h-4 w-4" />
                        </Button>
                    </div>
                    <div class="flex-1 overflow-hidden">
                        <slot name="settings-panel" />
                    </div>
                </div>
            </div>
        </div>

        <div
            v-if="$slots['footer']"
            class="builder-footer border-t bg-background"
        >
            <slot name="footer" />
        </div>
    </div>
</template>

<style scoped>
.builder-layout {
    flex: 1;
    min-height: 0;
}

@media (max-width: 768px) {
    .builder-content {
        flex-direction: column;
    }

    .builder-fields-panel,
    .builder-settings-panel {
        width: 100% !important;
        height: auto;
        max-height: 40vh;
        border-right: none;
        border-left: none;
        border-bottom: 1px solid hsl(var(--border));
    }

    .builder-canvas {
        flex: 1;
    }
}
</style>
