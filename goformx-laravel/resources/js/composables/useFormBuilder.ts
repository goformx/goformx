import { Formio } from '@formio/js';
import goforms from '@goformx/formio';
import { ref, onMounted, onUnmounted, watch, type Ref } from 'vue';
import { Logger } from '@/lib/logger';
import { useFormBuilderState, type FormComponent } from './useFormBuilderState';

Formio.use(goforms);

export interface FormSchema {
    display?: string;
    components: FormComponent[];
    [key: string]: unknown;
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
    updateField: (fieldKey: string, updates: Partial<FormComponent>) => void;
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
    options: FormBuilderOptions,
): UseFormBuilderReturn {
    const builder = ref<unknown | null>(null);
    const schema = ref<FormSchema>(options.schema ?? { ...defaultSchema });
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

    // 1a: Expanded builder instance type with form setter, on/off for events
    let builderInstance: {
        schema: FormSchema;
        form: FormSchema;
        on: (event: string, callback: (...args: unknown[]) => void) => void;
        off: (event: string, callback: (...args: unknown[]) => void) => void;
        destroy?: () => void;
    } | null = null;

    // Re-entrancy guard: prevents circular updates when we set builder.form
    let isSettingSchema = false;

    let autoSaveTimeout: ReturnType<typeof setTimeout> | null = null;
    let sidebarObserver: MutationObserver | null = null;
    let styleDebounceTimer: ReturnType<typeof setTimeout> | null = null;
    let mouseEnterHandler: ((e: Event) => void) | null = null;
    let mouseLeaveHandler: ((e: Event) => void) | null = null;
    let observedContainer: HTMLElement | null = null;

    /**
     * Apply inline !important styles to Form.io elements via element.style.setProperty().
     * Bootstrap's stylesheet rules use !important within the formio cascade layer,
     * which defeats normal CSS class overrides. Inline !important (set via JS)
     * wins the cascade over stylesheet !important.
     */
    function styleFormioElements(root: HTMLElement) {
        root.querySelectorAll<HTMLElement>('.gfx-sidebar-btn').forEach((btn) => {
            btn.style.setProperty('display', 'inline-flex', 'important');
            btn.style.setProperty('align-items', 'center', 'important');
            btn.style.setProperty('gap', '0.375rem', 'important');
            btn.style.setProperty('padding', '0.375rem 0.75rem', 'important');
            btn.style.setProperty('margin', '0', 'important');
            btn.style.setProperty('font-size', '0.8125rem', 'important');
            btn.style.setProperty('font-weight', '500', 'important');
            btn.style.setProperty('line-height', '1.25', 'important');
            btn.style.setProperty('border-radius', '0.375rem', 'important');
            btn.style.setProperty('border', '1px solid var(--border)', 'important');
            btn.style.setProperty('color', 'var(--foreground)', 'important');
            btn.style.setProperty('background-color', 'var(--background)', 'important');
            btn.style.setProperty('cursor', 'grab', 'important');
        });

        root.querySelectorAll<HTMLElement>('.drag-and-drop-alert').forEach((zone) => {
            zone.style.setProperty('border', '2px dashed var(--border)', 'important');
            zone.style.setProperty('color', 'var(--muted-foreground)', 'important');
            zone.style.setProperty('background', 'var(--muted)', 'important');
        });

        root.querySelectorAll<HTMLElement>('.btn-primary').forEach((btn) => {
            btn.style.setProperty('border', '1px solid var(--primary)', 'important');
            btn.style.setProperty('color', 'var(--primary-foreground)', 'important');
            btn.style.setProperty('background-color', 'var(--primary)', 'important');
        });
    }

    /**
     * Watch for Form.io DOM mutations and re-apply custom styles.
     * Also adds hover effects via event delegation since CSS :hover
     * is overridden by the JS-applied inline styles.
     */
    function observeSidebar(container: HTMLElement) {
        observedContainer = container;

        mouseEnterHandler = (e: Event) => {
            const btn = (e.target as HTMLElement).closest?.('.gfx-sidebar-btn') as HTMLElement | null;
            if (btn) {
                btn.style.setProperty('border-color', 'var(--foreground)', 'important');
                btn.style.setProperty('background-color', 'var(--accent)', 'important');
            }
        };
        mouseLeaveHandler = (e: Event) => {
            const btn = (e.target as HTMLElement).closest?.('.gfx-sidebar-btn') as HTMLElement | null;
            if (btn) {
                btn.style.setProperty('border', '1px solid var(--border)', 'important');
                btn.style.setProperty('background-color', 'var(--background)', 'important');
            }
        };

        container.addEventListener('mouseenter', mouseEnterHandler, true);
        container.addEventListener('mouseleave', mouseLeaveHandler, true);

        sidebarObserver = new MutationObserver(() => {
            if (styleDebounceTimer) clearTimeout(styleDebounceTimer);
            styleDebounceTimer = setTimeout(() => {
                styleFormioElements(container);
            }, 16);
        });
        sidebarObserver.observe(container, { childList: true, subtree: true });

        // Style elements already in the DOM before the observer starts watching
        styleFormioElements(container);
    }

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

            // 1b: Removed editForm override — restores native Form.io edit dialogs
            builderInstance = (await Formio.builder(container, schema.value, {
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
            })) as typeof builderInstance;

            builder.value = builderInstance;

            // Fix: Strip inline styles from Dragula mirror so gu-hide can work.
            // Sidebar buttons have JS-applied inline display:inline-flex!important
            // (to beat Bootstrap layer). The mirror is a clone that inherits these
            // inline styles. Dragula hides the mirror via .gu-hide{display:none!important}
            // (in the formio CSS layer), but inline !important beats layered !important
            // per CSS Cascade Level 5, so the mirror stays visible and blocks
            // elementFromPoint() from finding the actual drop target.
            const bi = builderInstance as { dragula?: {
                on: (event: string, callback: (...args: unknown[]) => void) => void;
            } };
            if (bi.dragula) {
                bi.dragula.on('cloned', (mirror: unknown) => {
                    const el = mirror as HTMLElement;
                    if (el.classList.contains('gfx-sidebar-btn')) {
                        el.style.removeProperty('display');
                    }
                });
            }

            // 1c: Rewrite builder event listeners
            if (builderInstance) {
                builderInstance.on(
                    'change',
                    (newSchema: unknown, flags: unknown) => {
                        // Ignore programmatic changes (from setSchema/builder.form assignment)
                        if (flags || isSettingSchema) return;
                        const s = newSchema as FormSchema;
                        schema.value = s;
                        pushHistory(s);
                        markDirty();
                        options.onSchemaChange?.(s);
                    },
                );

                builderInstance.on('editComponent', (component: unknown) => {
                    const comp = component as FormComponent;
                    if (comp.key) {
                        selectField(comp.key);
                    }
                });

                builderInstance.on('saveComponent', () => {
                    // Re-select to refresh sidebar data after native dialog save
                    if (selectedField.value) {
                        selectField(selectedField.value);
                    }
                });

                builderInstance.on('removeComponent', (component: unknown) => {
                    const comp = component as FormComponent;
                    if (
                        selectedField.value &&
                        comp.key === selectedField.value
                    ) {
                        selectField(null);
                    }
                });
            }

            // 1i: Push initial schema to history as baseline for undo
            pushHistory(schema.value);

            // Style sidebar buttons via JS to bypass Bootstrap layer !important
            observeSidebar(container);

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

    // 1d: Rewrite setSchema to sync to builder DOM
    function setSchema(newSchema: FormSchema) {
        schema.value = newSchema;
        if (builderInstance) {
            isSettingSchema = true;
            builderInstance.form = newSchema;
            // Reset guard after microtask so the resulting 'change' event is ignored
            void Promise.resolve().then(() => {
                isSettingSchema = false;
            });
        }
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
        if (styleDebounceTimer) clearTimeout(styleDebounceTimer);
        sidebarObserver?.disconnect();
        if (observedContainer) {
            if (mouseEnterHandler) observedContainer.removeEventListener('mouseenter', mouseEnterHandler, true);
            if (mouseLeaveHandler) observedContainer.removeEventListener('mouseleave', mouseLeaveHandler, true);
        }
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
            { deep: true },
        );
    }

    // 1e: Removed the deep watch(schema) that caused undo/redo infinite loop.
    // All callers that mutate schema now explicitly call pushHistory/markDirty/onSchemaChange.

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
        key: string,
    ): FormComponent | null {
        for (const component of components) {
            const comp = component as FormComponent;
            if (comp.key === key) {
                return comp;
            }
            if (comp['components']) {
                const found = findComponent(
                    comp['components'] as unknown[],
                    key,
                );
                if (found) return found;
            }
        }
        return null;
    }

    // 1g: Generate unique key that doesn't collide with existing keys
    function generateUniqueKey(
        baseKey: string,
        existingComponents: FormComponent[],
    ): string {
        const allKeys = new Set<string>();
        const collectKeys = (components: unknown[]): void => {
            for (const component of components) {
                const comp = component as FormComponent;
                allKeys.add(comp.key);
                if (comp['components']) {
                    collectKeys(comp['components'] as unknown[]);
                }
            }
        };
        collectKeys(existingComponents);

        const copyKey = `${baseKey}_copy`;
        if (!allKeys.has(copyKey)) return copyKey;

        let counter = 2;
        while (allKeys.has(`${copyKey}_${counter}`)) {
            counter++;
        }
        return `${copyKey}_${counter}`;
    }

    // 1f: Explicit history/dirty/callback calls in mutation functions
    function duplicateField(fieldKey: string) {
        const currentSchema = getSchema();
        const component = findComponent(currentSchema.components, fieldKey);
        if (!component) {
            Logger.warn(`Component with key "${fieldKey}" not found`);
            return;
        }
        const duplicate = JSON.parse(
            JSON.stringify(component),
        ) as FormComponent;
        duplicate.key = generateUniqueKey(
            component.key,
            currentSchema.components,
        );
        duplicate.label = `${component.label ?? component.type} (Copy)`;
        currentSchema.components.push(duplicate);
        setSchema(currentSchema);
        pushHistory(currentSchema);
        markDirty();
        options.onSchemaChange?.(currentSchema);
        Logger.debug(`Duplicated component: ${fieldKey}`);
    }

    function deleteField(fieldKey: string) {
        const currentSchema = getSchema();
        const filterComponents = (
            components: FormComponent[],
        ): FormComponent[] => {
            return components.filter((comp) => {
                if (comp.key === fieldKey) return false;
                if (comp['components']) {
                    comp['components'] = filterComponents(
                        comp['components'] as FormComponent[],
                    );
                }
                return true;
            });
        };
        currentSchema.components = filterComponents(currentSchema.components);
        // Deselect if the deleted field was selected
        if (selectedField.value === fieldKey) {
            selectField(null);
        }
        setSchema(currentSchema);
        pushHistory(currentSchema);
        markDirty();
        options.onSchemaChange?.(currentSchema);
        Logger.debug(`Deleted component: ${fieldKey}`);
    }

    // 1h: Update a field's properties by key (used by FieldSettingsPanel)
    function updateField(
        fieldKey: string,
        updates: Partial<FormComponent>,
    ): void {
        const currentSchema = JSON.parse(
            JSON.stringify(getSchema()),
        ) as FormSchema;
        const component = findComponent(currentSchema.components, fieldKey);
        if (!component) {
            Logger.warn(
                `updateField: component with key "${fieldKey}" not found`,
            );
            return;
        }
        Object.assign(component, updates);
        setSchema(currentSchema);
        pushHistory(currentSchema);
        markDirty();
        options.onSchemaChange?.(currentSchema);
        Logger.debug(`Updated component: ${fieldKey}`);
    }

    function exportSchema(): string {
        const currentSchema = getSchema();
        return JSON.stringify(currentSchema, null, 2);
    }

    function importSchema(json: string) {
        try {
            const imported = JSON.parse(json) as FormSchema;
            setSchema(imported);
            pushHistory(imported);
            markDirty();
            options.onSchemaChange?.(imported);
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
        updateField,
        undo,
        redo,
        canUndo,
        canRedo,
        exportSchema,
        importSchema,
    };
}
