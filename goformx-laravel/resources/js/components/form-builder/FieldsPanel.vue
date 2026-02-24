<script setup lang="ts">
import {
    Type,
    AlignLeft,
    Hash,
    Mail,
    Phone,
    Calendar,
    CheckSquare,
    Circle,
    List,
    FileText,
    Layout,
    Columns,
    Square,
    Star,
} from 'lucide-vue-next';
import { ref, computed } from 'vue';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';

interface FieldType {
    type: string;
    label: string;
    icon: typeof Type;
    category: 'basic' | 'layout' | 'advanced';
    keywords?: string[];
}

const searchQuery = ref('');

const fieldTypes: FieldType[] = [
    {
        type: 'textfield',
        label: 'Text',
        icon: Type,
        category: 'basic',
        keywords: ['input', 'text', 'string'],
    },
    {
        type: 'textarea',
        label: 'Text Area',
        icon: AlignLeft,
        category: 'basic',
        keywords: ['paragraph', 'multiline'],
    },
    {
        type: 'number',
        label: 'Number',
        icon: Hash,
        category: 'basic',
        keywords: ['integer', 'decimal', 'numeric'],
    },
    {
        type: 'email',
        label: 'Email',
        icon: Mail,
        category: 'basic',
        keywords: ['email address', 'contact'],
    },
    {
        type: 'phoneNumber',
        label: 'Phone',
        icon: Phone,
        category: 'basic',
        keywords: ['telephone', 'mobile'],
    },
    {
        type: 'datetime',
        label: 'Date/Time',
        icon: Calendar,
        category: 'basic',
        keywords: ['date', 'time', 'calendar'],
    },
    {
        type: 'checkbox',
        label: 'Checkbox',
        icon: CheckSquare,
        category: 'basic',
        keywords: ['check', 'toggle', 'boolean'],
    },
    {
        type: 'radio',
        label: 'Radio',
        icon: Circle,
        category: 'basic',
        keywords: ['option', 'choice'],
    },
    {
        type: 'select',
        label: 'Select',
        icon: List,
        category: 'basic',
        keywords: ['dropdown', 'picker', 'options'],
    },
    {
        type: 'file',
        label: 'File Upload',
        icon: FileText,
        category: 'basic',
        keywords: ['attachment', 'upload', 'document'],
    },
    {
        type: 'panel',
        label: 'Panel',
        icon: Layout,
        category: 'layout',
        keywords: ['container', 'group', 'section'],
    },
    {
        type: 'columns',
        label: 'Columns',
        icon: Columns,
        category: 'layout',
        keywords: ['grid', 'layout', 'flex'],
    },
    {
        type: 'fieldset',
        label: 'Field Set',
        icon: Square,
        category: 'layout',
        keywords: ['group', 'section'],
    },
    {
        type: 'button',
        label: 'Button',
        icon: Star,
        category: 'advanced',
        keywords: ['submit', 'action', 'click'],
    },
];

const filteredFields = computed(() => {
    if (!searchQuery.value.trim()) {
        return fieldTypes;
    }
    const query = searchQuery.value.toLowerCase();
    return fieldTypes.filter((field) => {
        if (field.label.toLowerCase().includes(query)) return true;
        if (field.type.toLowerCase().includes(query)) return true;
        if (field.keywords?.some((keyword) => keyword.includes(query)))
            return true;
        return false;
    });
});

const basicFields = computed(() =>
    filteredFields.value.filter((f) => f.category === 'basic'),
);
const layoutFields = computed(() =>
    filteredFields.value.filter((f) => f.category === 'layout'),
);
const advancedFields = computed(() =>
    filteredFields.value.filter((f) => f.category === 'advanced'),
);
</script>

<template>
    <div class="fields-panel flex h-full flex-col">
        <div class="px-4 py-3">
            <Input
                v-model="searchQuery"
                type="search"
                placeholder="Search fields..."
                class="h-9"
            />
        </div>

        <ScrollArea class="flex-1">
            <div class="space-y-4 px-2 pb-4">
                <div v-if="basicFields.length > 0">
                    <div class="mb-2 px-2">
                        <h4
                            class="text-xs font-semibold tracking-wide text-muted-foreground uppercase"
                        >
                            Basic
                        </h4>
                    </div>
                    <div class="space-y-1">
                        <button
                            v-for="field in basicFields"
                            :key="field.type"
                            class="field-item flex w-full cursor-move items-center gap-3 rounded-md px-3 py-2 transition-colors hover:bg-accent hover:text-accent-foreground"
                            :data-type="field.type"
                            :title="field.label"
                        >
                            <component
                                :is="field.icon"
                                class="h-4 w-4 flex-shrink-0 text-muted-foreground"
                            />
                            <span
                                class="flex-1 truncate text-left text-sm font-medium"
                                >{{ field.label }}</span
                            >
                        </button>
                    </div>
                </div>

                <Separator
                    v-if="
                        basicFields.length > 0 &&
                        (layoutFields.length > 0 || advancedFields.length > 0)
                    "
                />

                <div v-if="layoutFields.length > 0">
                    <div class="mb-2 px-2">
                        <h4
                            class="text-xs font-semibold tracking-wide text-muted-foreground uppercase"
                        >
                            Layout
                        </h4>
                    </div>
                    <div class="space-y-1">
                        <button
                            v-for="field in layoutFields"
                            :key="field.type"
                            class="field-item flex w-full cursor-move items-center gap-3 rounded-md px-3 py-2 transition-colors hover:bg-accent hover:text-accent-foreground"
                            :data-type="field.type"
                            :title="field.label"
                        >
                            <component
                                :is="field.icon"
                                class="h-4 w-4 flex-shrink-0 text-muted-foreground"
                            />
                            <span
                                class="flex-1 truncate text-left text-sm font-medium"
                                >{{ field.label }}</span
                            >
                        </button>
                    </div>
                </div>

                <Separator
                    v-if="layoutFields.length > 0 && advancedFields.length > 0"
                />

                <div v-if="advancedFields.length > 0">
                    <div class="mb-2 px-2">
                        <h4
                            class="text-xs font-semibold tracking-wide text-muted-foreground uppercase"
                        >
                            Advanced
                        </h4>
                    </div>
                    <div class="space-y-1">
                        <button
                            v-for="field in advancedFields"
                            :key="field.type"
                            class="field-item flex w-full cursor-move items-center gap-3 rounded-md px-3 py-2 transition-colors hover:bg-accent hover:text-accent-foreground"
                            :data-type="field.type"
                            :title="field.label"
                        >
                            <component
                                :is="field.icon"
                                class="h-4 w-4 flex-shrink-0 text-muted-foreground"
                            />
                            <span
                                class="flex-1 truncate text-left text-sm font-medium"
                                >{{ field.label }}</span
                            >
                        </button>
                    </div>
                </div>

                <div
                    v-if="filteredFields.length === 0"
                    class="px-3 py-8 text-center"
                >
                    <p class="text-sm text-muted-foreground">No fields found</p>
                    <p class="mt-1 text-xs text-muted-foreground">
                        Try a different search term
                    </p>
                </div>
            </div>
        </ScrollArea>
    </div>
</template>

<style scoped>
.field-item {
    user-select: none;
}

.field-item:active {
    cursor: grabbing;
}
</style>
