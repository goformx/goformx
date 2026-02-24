<script setup lang="ts">
import { Settings2, Copy, Trash2 } from 'lucide-vue-next';
import { computed } from 'vue';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import type { FormComponent } from '@/composables/useFormBuilderState';

interface Props {
    selectedField: FormComponent | null;
}

interface Emits {
    (e: 'update:field', field: FormComponent): void;
    (e: 'duplicate', fieldKey: string): void;
    (e: 'delete', fieldKey: string): void;
    (e: 'close'): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const hasSelectedField = computed(() => props.selectedField !== null);
const field = computed(() => props.selectedField as FormComponent);

const validate = computed(() => {
    return (
        (field.value?.['validate'] as Record<string, unknown> | undefined) ?? {}
    );
});

function updateField(key: string, value: unknown) {
    if (!props.selectedField) return;
    const updated = {
        ...props.selectedField,
        [key]: value,
    };
    emit('update:field', updated);
}

function duplicateField() {
    if (!props.selectedField) return;
    emit('duplicate', props.selectedField.key);
}

function deleteField() {
    if (!props.selectedField) return;
    emit('delete', props.selectedField.key);
}

function onInputChange(key: string, event: Event) {
    const target = event.target as HTMLInputElement;
    updateField(key, target.value);
}

function onTextareaChange(key: string, event: Event) {
    const target = event.target as HTMLTextAreaElement;
    updateField(key, target.value);
}

function onSwitchChange(key: string, checked: boolean) {
    updateField(key, checked);
}

function onValidateChange(validateKey: string, value: unknown) {
    const newValidate = { ...validate.value, [validateKey]: value };
    updateField('validate', newValidate);
}

function onValidateNumberChange(validateKey: string, event: Event) {
    const target = event.target as HTMLInputElement;
    const newValidate = {
        ...validate.value,
        [validateKey]: parseInt(target.value, 10),
    };
    updateField('validate', newValidate);
}
</script>

<template>
    <div class="settings-panel flex h-full flex-col">
        <div
            v-if="!hasSelectedField"
            class="flex flex-1 flex-col items-center justify-center p-6 text-center"
        >
            <Settings2 class="mb-4 h-12 w-12 text-muted-foreground/50" />
            <h3 class="mb-2 text-sm font-semibold">No Field Selected</h3>
            <p class="max-w-[200px] text-xs text-muted-foreground">
                Select a field in the canvas to view and edit its properties
            </p>
        </div>

        <div v-else class="flex flex-1 flex-col overflow-hidden">
            <div class="border-b px-4 py-3">
                <div class="flex items-start justify-between gap-2">
                    <div class="min-w-0 flex-1">
                        <h3 class="truncate text-sm font-semibold">
                            {{ field.label ?? field.type }}
                        </h3>
                        <p class="truncate text-xs text-muted-foreground">
                            {{ field.type }}
                        </p>
                    </div>
                    <div class="flex gap-1">
                        <Button
                            variant="ghost"
                            size="icon"
                            class="h-7 w-7"
                            title="Duplicate field"
                            @click="duplicateField"
                        >
                            <Copy class="h-3.5 w-3.5" />
                        </Button>
                        <Button
                            variant="ghost"
                            size="icon"
                            class="h-7 w-7 text-destructive hover:text-destructive"
                            title="Delete field"
                            @click="deleteField"
                        >
                            <Trash2 class="h-3.5 w-3.5" />
                        </Button>
                    </div>
                </div>
            </div>

            <Tabs
                default-value="display"
                class="flex flex-1 flex-col overflow-hidden"
            >
                <div class="px-4 pt-3">
                    <TabsList class="grid w-full grid-cols-3">
                        <TabsTrigger value="display" class="text-xs">
                            Display
                        </TabsTrigger>
                        <TabsTrigger value="data" class="text-xs"
                            >Data</TabsTrigger
                        >
                        <TabsTrigger value="validation" class="text-xs">
                            Validation
                        </TabsTrigger>
                    </TabsList>
                </div>

                <ScrollArea class="flex-1">
                    <TabsContent
                        value="display"
                        class="mt-0 space-y-4 px-4 pb-4"
                    >
                        <div class="space-y-4 pt-4">
                            <div class="space-y-2">
                                <Label for="field-label" class="text-xs"
                                    >Label</Label
                                >
                                <Input
                                    id="field-label"
                                    :model-value="field.label ?? ''"
                                    type="text"
                                    placeholder="Field label"
                                    @input="
                                        (e: Event) => onInputChange('label', e)
                                    "
                                />
                            </div>
                            <div class="space-y-2">
                                <Label for="field-placeholder" class="text-xs"
                                    >Placeholder</Label
                                >
                                <Input
                                    id="field-placeholder"
                                    :model-value="
                                        (field['placeholder'] as string) ?? ''
                                    "
                                    type="text"
                                    placeholder="Placeholder text"
                                    @input="
                                        (e: Event) =>
                                            onInputChange('placeholder', e)
                                    "
                                />
                            </div>
                            <div class="space-y-2">
                                <Label for="field-description" class="text-xs"
                                    >Description</Label
                                >
                                <textarea
                                    id="field-description"
                                    :value="
                                        (field['description'] as string) ?? ''
                                    "
                                    class="flex min-h-[60px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                                    placeholder="Field description"
                                    @input="
                                        (e: Event) =>
                                            onTextareaChange('description', e)
                                    "
                                />
                            </div>
                            <div class="flex items-center justify-between">
                                <Label for="field-hidden" class="text-xs"
                                    >Hidden</Label
                                >
                                <Switch
                                    id="field-hidden"
                                    :checked="
                                        (field['hidden'] as boolean) ?? false
                                    "
                                    @update:checked="
                                        (checked: boolean) =>
                                            onSwitchChange('hidden', checked)
                                    "
                                />
                            </div>
                            <div class="flex items-center justify-between">
                                <Label for="field-disabled" class="text-xs"
                                    >Disabled</Label
                                >
                                <Switch
                                    id="field-disabled"
                                    :checked="
                                        (field['disabled'] as boolean) ?? false
                                    "
                                    @update:checked="
                                        (checked: boolean) =>
                                            onSwitchChange('disabled', checked)
                                    "
                                />
                            </div>
                        </div>
                    </TabsContent>

                    <TabsContent value="data" class="mt-0 space-y-4 px-4 pb-4">
                        <div class="space-y-4 pt-4">
                            <div class="space-y-2">
                                <Label for="field-key" class="text-xs"
                                    >Field Key (API Name)</Label
                                >
                                <Input
                                    id="field-key"
                                    :model-value="field.key"
                                    type="text"
                                    placeholder="fieldKey"
                                    @input="
                                        (e: Event) => onInputChange('key', e)
                                    "
                                />
                                <p class="text-xs text-muted-foreground">
                                    Used to identify this field in the API
                                </p>
                            </div>
                            <div class="space-y-2">
                                <Label for="field-default" class="text-xs"
                                    >Default Value</Label
                                >
                                <Input
                                    id="field-default"
                                    :model-value="
                                        (field['defaultValue'] as string) ?? ''
                                    "
                                    type="text"
                                    placeholder="Default value"
                                    @input="
                                        (e: Event) =>
                                            onInputChange('defaultValue', e)
                                    "
                                />
                            </div>
                            <Separator />
                            <div class="flex items-center justify-between">
                                <div>
                                    <Label
                                        for="field-persistent"
                                        class="text-xs"
                                        >Persistent</Label
                                    >
                                    <p class="text-xs text-muted-foreground">
                                        Save to database
                                    </p>
                                </div>
                                <Switch
                                    id="field-persistent"
                                    :checked="
                                        (field['persistent'] as boolean) ?? true
                                    "
                                    @update:checked="
                                        (checked: boolean) =>
                                            onSwitchChange(
                                                'persistent',
                                                checked,
                                            )
                                    "
                                />
                            </div>
                        </div>
                    </TabsContent>

                    <TabsContent
                        value="validation"
                        class="mt-0 space-y-4 px-4 pb-4"
                    >
                        <div class="space-y-4 pt-4">
                            <div class="flex items-center justify-between">
                                <div>
                                    <Label for="field-required" class="text-xs"
                                        >Required</Label
                                    >
                                    <p class="text-xs text-muted-foreground">
                                        Field must have a value
                                    </p>
                                </div>
                                <Switch
                                    id="field-required"
                                    :checked="
                                        (validate['required'] as boolean) ??
                                        false
                                    "
                                    @update:checked="
                                        (checked: boolean) =>
                                            onValidateChange(
                                                'required',
                                                checked,
                                            )
                                    "
                                />
                            </div>
                            <div class="space-y-2">
                                <Label for="field-error-label" class="text-xs"
                                    >Custom Error Message</Label
                                >
                                <Input
                                    id="field-error-label"
                                    :model-value="
                                        (field['errorLabel'] as string) ?? ''
                                    "
                                    type="text"
                                    placeholder="This field is required"
                                    @input="
                                        (e: Event) =>
                                            onInputChange('errorLabel', e)
                                    "
                                />
                            </div>
                            <template
                                v-if="
                                    ['textfield', 'textarea', 'email'].includes(
                                        field.type,
                                    )
                                "
                            >
                                <Separator />
                                <div class="space-y-2">
                                    <Label for="field-minlength" class="text-xs"
                                        >Minimum Length</Label
                                    >
                                    <Input
                                        id="field-minlength"
                                        :model-value="
                                            (validate['minLength'] as number) ??
                                            ''
                                        "
                                        type="number"
                                        placeholder="0"
                                        @input="
                                            (e: Event) =>
                                                onValidateNumberChange(
                                                    'minLength',
                                                    e,
                                                )
                                        "
                                    />
                                </div>
                                <div class="space-y-2">
                                    <Label for="field-maxlength" class="text-xs"
                                        >Maximum Length</Label
                                    >
                                    <Input
                                        id="field-maxlength"
                                        :model-value="
                                            (validate['maxLength'] as number) ??
                                            ''
                                        "
                                        type="number"
                                        placeholder="Unlimited"
                                        @input="
                                            (e: Event) =>
                                                onValidateNumberChange(
                                                    'maxLength',
                                                    e,
                                                )
                                        "
                                    />
                                </div>
                            </template>
                        </div>
                    </TabsContent>
                </ScrollArea>
            </Tabs>
        </div>
    </div>
</template>
