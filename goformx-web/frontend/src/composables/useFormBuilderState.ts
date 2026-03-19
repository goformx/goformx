import { ref, computed, type Ref } from 'vue';
import { Logger } from '@/lib/logger';

export interface FormComponent {
    key: string;
    type: string;
    label?: string;
    [key: string]: unknown;
}

export interface FormSchema {
    components: FormComponent[];
    display?: string;
    [key: string]: unknown;
}

export interface BuilderState {
    selectedField: Ref<string | null>;
    isDirty: Ref<boolean>;
    history: Ref<FormSchema[]>;
    historyIndex: Ref<number>;
    canUndo: Ref<boolean>;
    canRedo: Ref<boolean>;
}

export interface UseFormBuilderStateReturn extends BuilderState {
    selectField: (fieldKey: string | null) => void;
    markDirty: () => void;
    markClean: () => void;
    pushHistory: (schema: FormSchema) => void;
    undo: () => FormSchema | null;
    redo: () => FormSchema | null;
    clearHistory: () => void;
    reset: () => void;
}

const MAX_HISTORY_SIZE = 50;

export function useFormBuilderState(
    formId?: string,
): UseFormBuilderStateReturn {
    const selectedField = ref<string | null>(null);
    const isDirty = ref(false);
    const history = ref<FormSchema[]>([]);
    const historyIndex = ref(-1);

    const canUndo = computed(() => historyIndex.value > 0);
    const canRedo = computed(
        () => historyIndex.value < history.value.length - 1,
    );

    if (formId) {
        try {
            const savedState = localStorage.getItem(
                `form-builder-state-${formId}`,
            );
            if (savedState) {
                const parsed = JSON.parse(savedState) as {
                    selectedField: string | null;
                };
                selectedField.value = parsed.selectedField;
            }
        } catch (error) {
            Logger.error('Failed to load form builder state:', error);
        }
    }

    const saveState = (): void => {
        if (!formId) return;
        try {
            const state = {
                selectedField: selectedField.value,
            };
            localStorage.setItem(
                `form-builder-state-${formId}`,
                JSON.stringify(state),
            );
        } catch (error) {
            Logger.error('Failed to save form builder state:', error);
        }
    };

    const selectField = (fieldKey: string | null): void => {
        selectedField.value = fieldKey;
        saveState();
    };

    const markDirty = (): void => {
        isDirty.value = true;
    };

    const markClean = (): void => {
        isDirty.value = false;
    };

    const pushHistory = (schema: FormSchema): void => {
        if (historyIndex.value < history.value.length - 1) {
            history.value = history.value.slice(0, historyIndex.value + 1);
        }
        history.value.push(JSON.parse(JSON.stringify(schema)) as FormSchema);
        if (history.value.length > MAX_HISTORY_SIZE) {
            history.value.shift();
            // historyIndex stays the same: the removed entry was before the current position
        } else {
            historyIndex.value++;
        }
        markDirty();
    };

    const undo = (): FormSchema | null => {
        if (!canUndo.value) return null;
        historyIndex.value--;
        return JSON.parse(
            JSON.stringify(history.value[historyIndex.value]),
        ) as FormSchema;
    };

    const redo = (): FormSchema | null => {
        if (!canRedo.value) return null;
        historyIndex.value++;
        return JSON.parse(
            JSON.stringify(history.value[historyIndex.value]),
        ) as FormSchema;
    };

    const clearHistory = (): void => {
        history.value = [];
        historyIndex.value = -1;
    };

    const reset = (): void => {
        selectedField.value = null;
        isDirty.value = false;
        clearHistory();
        saveState();
    };

    return {
        selectedField,
        isDirty,
        history,
        historyIndex,
        canUndo,
        canRedo,
        selectField,
        markDirty,
        markClean,
        pushHistory,
        undo,
        redo,
        clearHistory,
        reset,
    };
}
