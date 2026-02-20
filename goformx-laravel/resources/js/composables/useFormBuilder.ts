import { ref, onMounted, onUnmounted, watch, type Ref } from 'vue';
import { Formio } from '@formio/js';
import goforms from '@goformx/formio';
import { Logger } from '@/lib/logger';
import { useFormBuilderState, type FormComponent } from './useFormBuilderState';

Formio.use(goforms);

export interface FormSchema {
    display?: string;
    components: FormComponent[];
}

export interface FormBuilderOptions {
    containerId: string;
    formId: string;
    schema?: FormSchema;
    onSchemaChange?: (schema: FormSchema) => void;
    onSave?: (schema: FormSchema) => Promise<void>;
    autoSave?: boolean;
    autoSaveDelay?: number;
}

export interface UseFormBuilderReturn {
    builder: Ref<unknown | null>;
    schema: Ref<FormSchema>;
    isLoading: Ref<boolean>;
    error: Ref<string | null>;
    isSaving: Ref<boolean>;
    saveSchema: () => Promise<void>;
    getSchema: () => FormSchema;
    setSchema: (newSchema: FormSchema) => void;
    selectedField: Ref<string | null>;
    selectField: (fieldKey: string | null) => void;
    duplicateField: (fieldKey: string) => void;
    deleteField: (fieldKey: string) => void;
    undo: () => void;
    redo: () => void;
    canUndo: Ref<boolean>;
    canRedo: Ref<boolean>;
    exportSchema: () => string;
    importSchema: (json: string) => void;
}

const defaultSchema: FormSchema = {
    display: 'form',
    components: [],
};

export function useFormBuilder(
    options: FormBuilderOptions
): UseFormBuilderReturn {
    const builder = ref<unknown | null>(null);
    const schema = ref<FormSchema>(
        options.schema ?? { ...defaultSchema }
    );
    const isLoading = ref(true);
    const error = ref<string | null>(null);
    const isSaving = ref(false);

    const {
        selectedField,
        selectField,
        pushHistory,
        undo: undoHistory,
        redo: redoHistory,
        canUndo,
        canRedo,
        markDirty,
    } = useFormBuilderState(options.formId);

    let builderInstance: { schema: FormSchema; destroy?: () => void } | null =
        null;
    let autoSaveTimeout: ReturnType<typeof setTimeout> | null = null;

    async function initializeBuilder() {
        const container = document.getElementById(options.containerId);
        if (!container) {
            error.value = `Container element #${options.containerId} not found`;
            isLoading.value = false;
            return;
        }

        try {
            Logger.debug('Initializing Form.io builder...');

            // Use provided schema from Inertia (Laravel passes form.schema from Go)
            if (options.schema && options.schema.components) {
                schema.value = options.schema;
            }

            builderInstance = await Formio.builder(container, schema.value, {
                builder: {
                    basic: {
                        default: true,
                        weight: 0,
                        title: 'Basic',
                        components: {
                            textfield: true,
                            textarea: true,
                            number: true,
                            checkbox: true,
                            select: true,
                            radio: true,
                            email: true,
                            phoneNumber: true,
                            datetime: true,
                            button: true,
                        },
                    },
                    layout: {
                        default: false,
                        weight: 10,
                        title: 'Layout',
                        components: {
                            panel: true,
                            columns: true,
                            fieldset: true,
                        },
                    },
                    advanced: false,
                    data: false,
                    premium: false,
                },
                noDefaultSubmitButton: false,
                i18n: {
                    en: {
                        searchFields: 'Search fields...',
                        dragAndDropComponent: 'Drag and drop fields here',
                        basic: 'Basic',
                        advanced: 'Advanced',
                        layout: 'Layout',
                        data: 'Data',
                        premium: 'Premium',
                    },
                },
                editForm: {
                    textfield: [
                        { key: 'display', components: [] },
                        { key: 'data', components: [] },
                        { key: 'validation', components: [] },
                        { key: 'api', components: [] },
                        { key: 'conditional', components: [] },
                        { key: 'logic', components: [] },
                    ],
                },
            });

            builder.value = builderInstance;

            const builderWithEvents = builderInstance as unknown as {
                on?: (event: string, callback: (s: FormSchema) => void) => void;
            };

            if (builderInstance && typeof builderWithEvents.on === 'function') {
                builderWithEvents.on('change', (newSchema: FormSchema) => {
                    schema.value = newSchema;
                    options.onSchemaChange?.(newSchema);
                });
            }

            Logger.debug('Form.io builder initialized successfully');
        } catch (err) {
            Logger.error('Failed to initialize Form.io builder:', err);
            error.value = 'Failed to initialize form builder';
        } finally {
            isLoading.value = false;
        }
    }

    function getSchema(): FormSchema {
        if (builderInstance) {
            return builderInstance.schema;
        }
        return schema.value;
    }

    function setSchema(newSchema: FormSchema) {
        schema.value = newSchema;
    }

    async function saveSchema() {
        if (!options.formId) {
            error.value = 'No form ID provided';
            return;
        }

        isSaving.value = true;
        error.value = null;

        try {
            const currentSchema = getSchema();
            if (options.onSave) {
                await options.onSave(currentSchema);
            } else {
                error.value = 'No save handler configured';
                throw new Error('No save handler configured');
            }
            Logger.debug('Schema saved successfully');
        } catch (err) {
            Logger.error('Failed to save schema:', err);
            error.value = 'Failed to save form schema';
            throw err;
        } finally {
            isSaving.value = false;
        }
    }

    onMounted(() => {
        void initializeBuilder();
    });

    onUnmounted(() => {
        if (builderInstance && typeof builderInstance.destroy === 'function') {
            builderInstance.destroy();
        }
        if (autoSaveTimeout) {
            clearTimeout(autoSaveTimeout);
        }
    });

    if (options.autoSave) {
        watch(
            schema,
            () => {
                if (autoSaveTimeout) {
                    clearTimeout(autoSaveTimeout);
                }
                const delay = options.autoSaveDelay ?? 2000;
                autoSaveTimeout = setTimeout(() => {
                    void saveSchema();
                }, delay);
            },
            { deep: true }
        );
    }

    watch(
        schema,
        (newSchema) => {
            pushHistory(newSchema);
            markDirty();
            options.onSchemaChange?.(newSchema);
        },
        { deep: true }
    );

    function undo() {
        const previousSchema = undoHistory();
        if (previousSchema) {
            setSchema(previousSchema);
        }
    }

    function redo() {
        const nextSchema = redoHistory();
        if (nextSchema) {
            setSchema(nextSchema);
        }
    }

    function findComponent(
        components: unknown[],
        key: string
    ): FormComponent | null {
        for (const component of components) {
            const comp = component as FormComponent;
            if (comp.key === key) {
                return comp;
            }
            if (comp['components']) {
                const found = findComponent(
                    comp['components'] as unknown[],
                    key
                );
                if (found) return found;
            }
        }
        return null;
    }

    function duplicateField(fieldKey: string) {
        const currentSchema = getSchema();
        const component = findComponent(currentSchema.components, fieldKey);
        if (!component) {
            Logger.warn(`Component with key "${fieldKey}" not found`);
            return;
        }
        const duplicate = JSON.parse(
            JSON.stringify(component)
        ) as FormComponent;
        duplicate.key = `${component.key}_copy`;
        duplicate.label = `${component.label ?? component.type} (Copy)`;
        currentSchema.components.push(duplicate);
        setSchema(currentSchema);
        Logger.debug(`Duplicated component: ${fieldKey}`);
    }

    function deleteField(fieldKey: string) {
        const currentSchema = getSchema();
        const filterComponents = (
            components: FormComponent[]
        ): FormComponent[] => {
            return components.filter((comp) => {
                if (comp.key === fieldKey) return false;
                if (comp['components']) {
                    comp['components'] = filterComponents(
                        comp['components'] as FormComponent[]
                    );
                }
                return true;
            });
        };
        currentSchema.components = filterComponents(currentSchema.components);
        setSchema(currentSchema);
        Logger.debug(`Deleted component: ${fieldKey}`);
    }

    function exportSchema(): string {
        const currentSchema = getSchema();
        return JSON.stringify(currentSchema, null, 2);
    }

    function importSchema(json: string) {
        try {
            const imported = JSON.parse(json) as FormSchema;
            setSchema(imported);
            Logger.debug('Schema imported successfully');
        } catch (err) {
            Logger.error('Failed to import schema:', err);
            error.value = 'Invalid schema JSON';
        }
    }

    return {
        builder,
        schema,
        isLoading,
        error,
        isSaving,
        saveSchema,
        getSchema,
        setSchema,
        selectedField,
        selectField,
        duplicateField,
        deleteField,
        undo,
        redo,
        canUndo,
        canRedo,
        exportSchema,
        importSchema,
    };
}
