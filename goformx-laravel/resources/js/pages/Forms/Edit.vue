<script setup lang="ts">
import { Formio } from '@formio/js';
import type { RequestPayload } from '@inertiajs/core';
import { Head, useForm, router, Link } from '@inertiajs/vue3';
import {
    ChevronDown,
    Eye,
    ListChecks,
    Save,
    Code,
    Undo2,
    Redo2,
    Keyboard,
    Pencil,
    Settings2,
} from 'lucide-vue-next';
import { ref, computed, watch, nextTick, onBeforeUnmount } from 'vue';
import { toast } from 'vue-sonner';
import BuilderLayout from '@/components/form-builder/BuilderLayout.vue';
import FieldSettingsPanel from '@/components/form-builder/FieldSettingsPanel.vue';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Collapsible,
    CollapsibleContent,
    CollapsibleTrigger,
} from '@/components/ui/collapsible';
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Separator } from '@/components/ui/separator';
import { useFormBuilder, type FormSchema } from '@/composables/useFormBuilder';
import type { FormComponent } from '@/composables/useFormBuilderState';
import {
    useKeyboardShortcuts,
    formatShortcut,
} from '@/composables/useKeyboardShortcuts';
import AppLayout from '@/layouts/AppLayout.vue';
import { dashboard } from '@/routes';
import { type BreadcrumbItem } from '@/types';

interface Form {
    id?: string;
    ID?: string;
    title?: string;
    description?: string;
    status?: 'draft' | 'published' | 'archived';
    cors_origins?: { origins?: string[] };
    [key: string]: unknown;
}

interface Props {
    form: Form;
    flash?: {
        success?: string;
        error?: string;
    };
}

const props = defineProps<Props>();

const formId = props.form.id ?? props.form.ID ?? '';

const detailsForm = useForm({
    title: props.form.title ?? '',
    description: props.form.description ?? '',
    status: props.form.status ?? 'draft',
    cors_origins: props.form.cors_origins?.origins?.join(', ') ?? '',
});

const showSchemaModal = ref(false);
const showShortcutsModal = ref(false);
const isSavingAll = ref(false);
const showInPlacePreview = ref(false);
const isSettingsOpen = ref(false);
let inPlacePreviewInstance: { destroy?: () => void } | null = null;

const initialSchema = computed((): FormSchema => {
    const s = props.form.schema;
    if (s && typeof s === 'object' && 'components' in s) {
        return s as FormSchema;
    }
    return { display: 'form', components: [] };
});

const {
    isLoading: isBuilderLoading,
    error: builderError,
    isSaving,
    saveSchema,
    getSchema,
    selectedField,
    selectField,
    duplicateField,
    deleteField,
    updateField,
    undo,
    redo,
    canUndo,
    canRedo,
    exportSchema,
} = useFormBuilder({
    containerId: 'form-schema-builder',
    formId: String(formId),
    schema: initialSchema.value,
    autoSave: false,
    onSchemaChange: () => {},
    onSave: async (schema: FormSchema) => {
        await new Promise<void>((resolve, reject) => {
            router.put(
                `/forms/${formId}`,
                {
                    schema: schema as unknown,
                    title: detailsForm.title,
                    description: detailsForm.description,
                    status: detailsForm.status,
                    cors_origins: detailsForm.cors_origins,
                } as RequestPayload,
                {
                    preserveScroll: true,
                    onSuccess: () => resolve(),
                    onError: () => reject(new Error('Failed to save form')),
                },
            );
        });
    },
});

const selectedFieldData = computed<FormComponent | null>(() => {
    if (!selectedField.value) return null;
    const schema = getSchema();
    const findField = (components: unknown[]): FormComponent | null => {
        for (const comp of components) {
            const component = comp as FormComponent;
            if (component.key === selectedField.value) return component;
            if (component.components) {
                const found = findField(component.components as unknown[]);
                if (found) return found;
            }
        }
        return null;
    };
    return findField(schema.components);
});

const shortcuts = [
    {
        key: 's',
        meta: true,
        handler: () => void handleSave(),
        description: 'Save form',
    },
    {
        key: 'p',
        meta: true,
        handler: () => router.visit(`/forms/${formId}/preview`),
        description: 'Preview form',
    },
    { key: 'z', meta: true, handler: () => undo(), description: 'Undo' },
    {
        key: 'z',
        meta: true,
        shift: true,
        handler: () => redo(),
        description: 'Redo',
    },
    {
        key: 'd',
        meta: true,
        handler: () => {
            if (selectedField.value) duplicateField(selectedField.value);
        },
        description: 'Duplicate selected field',
    },
    {
        key: 'Backspace',
        meta: true,
        handler: () => {
            if (selectedField.value) deleteField(selectedField.value);
        },
        description: 'Delete selected field',
    },
    {
        key: '/',
        meta: true,
        handler: () => {
            showShortcutsModal.value = true;
        },
        description: 'Show shortcuts',
    },
];

useKeyboardShortcuts(shortcuts);

async function handleSave() {
    if (isSavingAll.value || isSaving.value) return;
    if (
        detailsForm.status === 'published' &&
        !detailsForm.cors_origins.trim()
    ) {
        toast.error('CORS origins are required when publishing a form.');
        return;
    }
    isSavingAll.value = true;
    try {
        await saveSchema();
    } catch (err) {
        const message =
            err instanceof Error ? err.message : 'Failed to save form';
        toast.error(message);
    } finally {
        isSavingAll.value = false;
    }
}

function viewSchema() {
    showSchemaModal.value = true;
}

watch(
    () => props.flash,
    (flash, oldFlash) => {
        if (flash?.success && flash.success !== oldFlash?.success) {
            toast.success(flash.success);
        }
        if (flash?.error && flash.error !== oldFlash?.error) {
            toast.error(flash.error);
        }
    },
    { immediate: true },
);

watch(
    builderError,
    (error) => {
        if (error) toast.error(error);
    },
    { immediate: true },
);

watch(showInPlacePreview, async (isPreview) => {
    if (isPreview) {
        await nextTick();
        const container = document.getElementById('edit-inplace-preview');
        if (!container) return;
        const schema = getSchema();
        if (!schema?.components?.length) {
            toast.info('Add fields in the builder to preview.');
            return;
        }
        try {
            inPlacePreviewInstance = await Formio.createForm(
                container,
                schema,
                {
                    readOnly: true,
                    noSubmit: true,
                    noAlerts: true,
                },
            );
        } catch (err) {
            console.error('Preview failed:', err);
            toast.error('Failed to load preview');
        }
    } else {
        if (inPlacePreviewInstance?.destroy) {
            inPlacePreviewInstance.destroy();
        }
        inPlacePreviewInstance = null;
    }
});

onBeforeUnmount(() => {
    if (inPlacePreviewInstance?.destroy) {
        inPlacePreviewInstance.destroy();
    }
    inPlacePreviewInstance = null;
});

const breadcrumbs: BreadcrumbItem[] = [
    { title: 'Dashboard', href: dashboard().url },
    { title: 'Forms', href: '/forms' },
    { title: props.form.title ?? 'Edit Form', href: `/forms/${formId}` },
];
</script>

<template>
    <Head :title="(form.title as string) ?? 'Edit Form'" />

    <AppLayout :breadcrumbs="breadcrumbs">
        <div class="flex h-[calc(100vh-4.5rem)] flex-col overflow-hidden">
            <div class="flex items-center justify-between border-b px-4 py-2">
                <div class="flex items-center gap-1">
                    <div class="flex items-center rounded-md border p-0.5">
                        <Button
                            variant="ghost"
                            size="sm"
                            class="h-7 px-3"
                            :class="!showInPlacePreview ? 'bg-muted' : ''"
                            @click="showInPlacePreview = false"
                        >
                            <Pencil class="mr-1.5 h-3.5 w-3.5" />
                            Builder
                        </Button>
                        <Button
                            variant="ghost"
                            size="sm"
                            class="h-7 px-3"
                            :class="showInPlacePreview ? 'bg-muted' : ''"
                            @click="showInPlacePreview = true"
                        >
                            <Eye class="mr-1.5 h-3.5 w-3.5" />
                            Preview
                        </Button>
                    </div>
                    <Separator orientation="vertical" class="mx-1 h-5" />
                    <Button
                        variant="ghost"
                        size="icon"
                        class="h-7 w-7"
                        :disabled="!canUndo"
                        title="Undo (Cmd+Z)"
                        @click="undo"
                    >
                        <Undo2 class="h-3.5 w-3.5" />
                    </Button>
                    <Button
                        variant="ghost"
                        size="icon"
                        class="h-7 w-7"
                        :disabled="!canRedo"
                        title="Redo (Cmd+Shift+Z)"
                        @click="redo"
                    >
                        <Redo2 class="h-3.5 w-3.5" />
                    </Button>
                </div>
                <div class="flex items-center gap-1.5">
                    <Badge
                        :variant="
                            form.status === 'published'
                                ? 'default'
                                : 'secondary'
                        "
                        class="text-xs"
                    >
                        {{ form.status ?? 'draft' }}
                    </Badge>
                    <Button
                        variant="ghost"
                        size="icon"
                        class="h-7 w-7"
                        title="View Schema"
                        @click="viewSchema"
                    >
                        <Code class="h-3.5 w-3.5" />
                    </Button>
                    <Button
                        variant="ghost"
                        size="icon"
                        class="h-7 w-7"
                        title="Keyboard Shortcuts (Cmd+/)"
                        @click="showShortcutsModal = true"
                    >
                        <Keyboard class="h-3.5 w-3.5" />
                    </Button>
                    <Separator orientation="vertical" class="mx-0.5 h-5" />
                    <Button variant="outline" size="sm" class="h-7" as-child>
                        <Link :href="`/forms/${formId}/submissions`">
                            <ListChecks class="mr-1.5 h-3.5 w-3.5" />
                            Submissions
                        </Link>
                    </Button>
                    <Button variant="outline" size="sm" class="h-7" as-child>
                        <Link :href="`/forms/${formId}/embed`">Embed</Link>
                    </Button>
                    <Button
                        size="sm"
                        class="h-7"
                        :disabled="isSavingAll || isBuilderLoading"
                        @click="handleSave"
                    >
                        <Save class="mr-1.5 h-3.5 w-3.5" />
                        <span v-if="isSavingAll">Saving...</span>
                        <span v-else>Save</span>
                    </Button>
                </div>
            </div>

            <div
                v-show="showInPlacePreview"
                class="rounded-lg border bg-background p-6 shadow-sm"
            >
                <p class="mb-4 text-sm text-muted-foreground">
                    In-place preview (read-only). Use “Builder” to edit.
                </p>
                <div id="edit-inplace-preview" class="min-h-[400px]" />
            </div>

            <BuilderLayout
                v-show="!showInPlacePreview"
                class="flex-1 overflow-hidden"
                :show-fields-panel="false"
                :show-settings-panel="!!selectedField"
            >
                <template #header>
                    <Collapsible v-model:open="isSettingsOpen" class="border-b px-4 py-2">
                        <div class="flex items-center gap-2">
                            <Input
                                id="title"
                                v-model="detailsForm.title"
                                type="text"
                                placeholder="Form title"
                                required
                                class="h-8 max-w-xs border-transparent bg-transparent text-sm font-medium hover:border-input focus:border-input"
                            />
                            <CollapsibleTrigger as-child>
                                <Button variant="ghost" size="sm" class="h-7 gap-1 text-xs text-muted-foreground">
                                    <Settings2 class="h-3.5 w-3.5" />
                                    Settings
                                    <ChevronDown
                                        class="h-3 w-3 transition-transform"
                                        :class="{ 'rotate-180': isSettingsOpen }"
                                    />
                                </Button>
                            </CollapsibleTrigger>
                        </div>
                        <CollapsibleContent class="mt-2 space-y-2 pb-1">
                            <div class="grid grid-cols-3 gap-3">
                                <div class="space-y-1">
                                    <Label for="status" class="text-xs text-muted-foreground">Status</Label>
                                    <select
                                        id="status"
                                        v-model="detailsForm.status"
                                        class="flex h-8 w-full rounded-md border border-input bg-background px-2 py-1 text-sm"
                                    >
                                        <option value="draft">Draft</option>
                                        <option value="published">Published</option>
                                        <option value="archived">Archived</option>
                                    </select>
                                </div>
                                <div class="space-y-1">
                                    <Label for="description" class="text-xs text-muted-foreground">Description</Label>
                                    <Input
                                        id="description"
                                        v-model="detailsForm.description"
                                        type="text"
                                        placeholder="Optional description"
                                        class="h-8"
                                    />
                                </div>
                                <div class="space-y-1">
                                    <Label for="corsOrigins" class="text-xs text-muted-foreground">CORS Origins</Label>
                                    <Input
                                        id="corsOrigins"
                                        v-model="detailsForm.cors_origins"
                                        type="text"
                                        placeholder="e.g. *, https://example.com"
                                        class="h-8"
                                    />
                                </div>
                            </div>
                        </CollapsibleContent>
                    </Collapsible>
                </template>

                <template #canvas>
                    <div class="h-full p-4">
                        <div
                            v-if="isBuilderLoading"
                            class="flex items-center justify-center py-12"
                        >
                            <div class="text-muted-foreground">
                                Loading form builder...
                            </div>
                        </div>
                        <div
                            id="form-schema-builder"
                            class="h-full min-h-[400px]"
                            :data-form-id="formId"
                        />
                    </div>
                </template>

                <template #settings-panel>
                    <FieldSettingsPanel
                        :selected-field="selectedFieldData"
                        @update:field="
                            (field: FormComponent) =>
                                updateField(field.key, field)
                        "
                        @duplicate="(key) => duplicateField(key)"
                        @delete="(key) => deleteField(key)"
                        @close="() => selectField(null)"
                    />
                </template>
            </BuilderLayout>

            <Dialog v-model:open="showSchemaModal">
                <DialogContent class="max-h-[80vh] max-w-3xl">
                    <DialogHeader>
                        <DialogTitle>Form Schema (JSON)</DialogTitle>
                    </DialogHeader>
                    <div class="max-h-[60vh] overflow-auto">
                        <pre
                            class="overflow-auto rounded-md bg-muted p-4 text-xs"
                            >{{ exportSchema() }}</pre
                        >
                    </div>
                    <div class="flex justify-end gap-2">
                        <Button
                            variant="outline"
                            @click="showSchemaModal = false"
                        >
                            Close
                        </Button>
                    </div>
                </DialogContent>
            </Dialog>

            <Dialog v-model:open="showShortcutsModal">
                <DialogContent class="max-w-md">
                    <DialogHeader>
                        <DialogTitle>Keyboard Shortcuts</DialogTitle>
                    </DialogHeader>
                    <div class="space-y-3">
                        <div
                            v-for="shortcut in shortcuts"
                            :key="shortcut.description"
                            class="flex items-center justify-between py-2"
                        >
                            <span class="text-sm">{{
                                shortcut.description
                            }}</span>
                            <kbd
                                class="inline-flex items-center gap-1 rounded border border-border bg-muted px-2 py-1 font-mono text-xs"
                            >
                                {{ formatShortcut(shortcut) }}
                            </kbd>
                        </div>
                    </div>
                    <div class="flex justify-end">
                        <Button @click="showShortcutsModal = false"
                            >Close</Button
                        >
                    </div>
                </DialogContent>
            </Dialog>
        </div>
    </AppLayout>
</template>
