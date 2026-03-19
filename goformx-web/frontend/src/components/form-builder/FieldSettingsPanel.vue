<script setup lang="ts">
import { Copy, Trash2, X } from 'lucide-vue-next';
import { computed } from 'vue';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Separator } from '@/components/ui/separator';
import type { FormComponent } from '@/composables/useFormBuilderState';

const props = defineProps<{
    selectedField: FormComponent | null;
}>();

const emit = defineEmits<{
    'update:field': [field: FormComponent];
    duplicate: [key: string];
    delete: [key: string];
    close: [];
}>();

const localField = computed(() => {
    if (!props.selectedField) return null;
    return { ...props.selectedField };
});

function updateLabel(value: string) {
    if (!localField.value) return;
    emit('update:field', { ...localField.value, label: value });
}

function updateKey(value: string) {
    if (!localField.value) return;
    emit('update:field', { ...localField.value, key: value });
}
</script>

<template>
    <div v-if="selectedField" class="flex h-full flex-col">
        <div class="flex items-center justify-between border-b px-4 py-2">
            <h3 class="text-sm font-medium">Field Settings</h3>
            <Button
                variant="ghost"
                size="icon"
                class="h-6 w-6"
                @click="emit('close')"
            >
                <X class="h-3.5 w-3.5" />
            </Button>
        </div>

        <div class="flex-1 space-y-4 overflow-y-auto p-4">
            <div class="space-y-2">
                <Label class="text-xs">Label</Label>
                <Input
                    :model-value="selectedField.label ?? ''"
                    class="h-8"
                    @update:model-value="updateLabel($event as string)"
                />
            </div>

            <div class="space-y-2">
                <Label class="text-xs">API Key</Label>
                <Input
                    :model-value="selectedField.key"
                    class="h-8 font-mono text-xs"
                    @update:model-value="updateKey($event as string)"
                />
            </div>

            <div class="space-y-2">
                <Label class="text-xs">Type</Label>
                <Input
                    :model-value="selectedField.type"
                    class="h-8"
                    disabled
                />
            </div>
        </div>

        <Separator />

        <div class="flex items-center gap-2 p-4">
            <Button
                variant="outline"
                size="sm"
                class="flex-1"
                @click="emit('duplicate', selectedField.key)"
            >
                <Copy class="mr-1.5 h-3.5 w-3.5" />
                Duplicate
            </Button>
            <Button
                variant="destructive"
                size="sm"
                class="flex-1"
                @click="emit('delete', selectedField.key)"
            >
                <Trash2 class="mr-1.5 h-3.5 w-3.5" />
                Delete
            </Button>
        </div>
    </div>
</template>
