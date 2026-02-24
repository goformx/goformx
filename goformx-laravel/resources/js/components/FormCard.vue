<script setup lang="ts">
import { Link } from '@inertiajs/vue3';
import { computed } from 'vue';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { edit } from '@/routes/forms';

interface Form {
    id?: string;
    ID?: string;
    title?: string;
    description?: string;
    status?: string;
    updated_at?: string;
    [key: string]: unknown;
}

const props = defineProps<{
    form: Form;
}>();

const formId = computed(() => props.form.id ?? props.form.ID ?? '');
const editUrl = computed(() =>
    formId.value ? edit.url({ id: formId.value }) : '',
);
const formattedDate = computed(() => {
    const raw = props.form.updated_at;
    if (!raw) return null;
    try {
        const d = new Date(raw);
        return d.toLocaleDateString(undefined, {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
        });
    } catch {
        return null;
    }
});
</script>

<template>
    <Link
        v-if="editUrl"
        :href="editUrl"
        class="block transition-opacity hover:opacity-90"
    >
        <Card
            class="h-full cursor-pointer border-sidebar-border/70 transition-colors hover:bg-sidebar-accent/50"
        >
            <CardHeader
                class="flex flex-row items-start justify-between gap-2 pb-2"
            >
                <CardTitle class="line-clamp-1 text-base">
                    {{ form.title ?? 'Untitled' }}
                </CardTitle>
                <Badge
                    v-if="form.status"
                    variant="secondary"
                    class="shrink-0 capitalize"
                >
                    {{ form.status }}
                </Badge>
            </CardHeader>
            <CardContent class="pt-0">
                <p
                    v-if="form.description"
                    class="line-clamp-2 text-sm text-muted-foreground"
                >
                    {{ form.description }}
                </p>
                <p
                    v-if="formattedDate"
                    class="mt-2 text-xs text-muted-foreground"
                >
                    Updated {{ formattedDate }}
                </p>
            </CardContent>
        </Card>
    </Link>
</template>
