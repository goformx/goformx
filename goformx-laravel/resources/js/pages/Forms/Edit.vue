<script setup lang="ts">
import { Formio } from '@formio/js';
import goforms from '@goformx/formio';
import { Head, useForm, router, Link } from '@inertiajs/vue3';
import {
    Eye,
    ListChecks,
    Save,
    Code,
    Undo2,
    Redo2,
    Keyboard,
    Pencil,
} from 'lucide-vue-next';
import { ref, computed, watch, nextTick, onBeforeUnmount } from 'vue';
import { toast } from 'vue-sonner';
import BuilderLayout from '@/components/form-builder/BuilderLayout.vue';
import FieldSettingsPanel from '@/components/form-builder/FieldSettingsPanel.vue';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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

Formio.use(goforms);

interface Form {
    id?: string;
    ID?: string;
    title?: string;
    description?: string;
    status?: 'draft' | 'published' | 'archived';
    corsOrigins?: string[];
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
    cors_origins: props.form.corsOrigins?.join(', ') ?? '',
});

const showSchemaModal = ref(false);
const showShortcutsModal = ref(false);
const isSavingAll = ref(false);
const showInPlacePreview = ref(false);
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
                    schema,
                    title: detailsForm.title,
                    description: detailsForm.description,
                    status: detailsForm.status,
                    cors_origins: detailsForm.cors_origins,
                },
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
        toast.success('Form saved successfully');
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
    (flash) => {
        if (flash?.success) toast.success(flash.success);
        if (flash?.error) toast.error(flash.error);
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
        <div class="flex flex-col gap-4 p-4">
            <div class="flex items-center justify-end gap-2">
                <Badge
                    :variant="
                        form.status === 'published' ? 'default' : 'secondary'
                    "
                >
                    {{ form.status ?? 'draft' }}
                </Badge>
                <Separator orientation="vertical" class="h-6" />
                <Button
                    variant="ghost"
                    size="icon"
                    :disabled="!canUndo"
                    title="Undo (Cmd+Z)"
                    @click="undo"
                >
                    <Undo2 class="h-4 w-4" />
                </Button>
                <Button
                    variant="ghost"
                    size="icon"
                    :disabled="!canRedo"
                    title="Redo (Cmd+Shift+Z)"
                    @click="redo"
                >
                    <Redo2 class="h-4 w-4" />
                </Button>
                <Separator orientation="vertical" class="h-6" />
                <Button
                    variant="outline"
                    size="sm"
                    @click="showShortcutsModal = true"
                >
                    <Keyboard class="mr-2 h-4 w-4" />
                    Shortcuts
                </Button>
                <Button variant="outline" size="sm" @click="viewSchema">
                    <Code class="mr-2 h-4 w-4" />
                    Schema
                </Button>
                <Button
                    variant="outline"
                    size="sm"
                    :class="!showInPlacePreview ? 'bg-muted' : ''"
                    @click="showInPlacePreview = false"
                >
                    <Pencil class="mr-2 h-4 w-4" />
                    Builder
                </Button>
                <Button
                    variant="outline"
                    size="sm"
                    :class="showInPlacePreview ? 'bg-muted' : ''"
                    @click="showInPlacePreview = true"
                >
                    <Eye class="mr-2 h-4 w-4" />
                    Preview
                </Button>
                <Button variant="outline" size="sm" as-child>
                    <Link :href="`/forms/${formId}/preview`"
                        >Shareable preview</Link
                    >
                </Button>
                <Button variant="outline" size="sm" as-child>
                    <Link :href="`/forms/${formId}/submissions`">
                        <ListChecks class="mr-2 h-4 w-4" />
                        Submissions
                    </Link>
                </Button>
                <Button variant="outline" size="sm" as-child>
                    <Link :href="`/forms/${formId}/embed`">Embed</Link>
                </Button>
                <Button
                    size="sm"
                    :disabled="isSavingAll || isBuilderLoading"
                    @click="handleSave"
                >
                    <Save class="mr-2 h-4 w-4" />
                    <span v-if="isSavingAll">Saving...</span>
                    <span v-else>Save</span>
                </Button>
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
                class="rounded-lg border bg-background shadow-sm"
                :show-fields-panel="false"
            >
                <template #header>
                    <div class="space-y-4 px-6 py-4">
                        <div class="grid grid-cols-2 gap-4">
                            <div class="space-y-2">
                                <Label for="title" class="text-xs"
                                    >Form Title</Label
                                >
                                <Input
                                    id="title"
                                    v-model="detailsForm.title"
                                    type="text"
                                    placeholder="Enter form title"
                                    required
                                />
                            </div>
                            <div class="space-y-2">
                                <Label for="status" class="text-xs"
                                    >Status</Label
                                >
                                <select
                                    id="status"
                                    v-model="detailsForm.status"
                                    class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                                >
                                    <option value="draft">Draft</option>
                                    <option value="published">Published</option>
                                    <option value="archived">Archived</option>
                                </select>
                            </div>
                        </div>
                        <div class="space-y-2">
                            <Label for="description" class="text-xs"
                                >Description</Label
                            >
                            <Input
                                id="description"
                                v-model="detailsForm.description"
                                type="text"
                                placeholder="Enter form description"
                            />
                        </div>
                        <div class="space-y-2">
                            <Label for="corsOrigins" class="text-xs"
                                >Allowed Origins (CORS)</Label
                            >
                            <Input
                                id="corsOrigins"
                                v-model="detailsForm.cors_origins"
                                type="text"
                                placeholder="e.g. *, https://example.com"
                            />
                            <p class="text-xs text-muted-foreground">
                                Required when publishing. Use * to allow all
                                origins.
                            </p>
                        </div>
                    </div>
                </template>

                <template #canvas>
                    <div class="p-6">
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
                            class="min-h-[500px]"
                            :data-form-id="formId"
                        />
                    </div>
                </template>

                <template #settings-panel>
                    <FieldSettingsPanel
                        :selected-field="selectedFieldData"
                        @update:field="() => {}"
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
