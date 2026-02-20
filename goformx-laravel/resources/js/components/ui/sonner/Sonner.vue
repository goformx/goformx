<script lang="ts" setup>
import type { ToasterProps } from 'vue-sonner';
import { reactiveOmit } from '@vueuse/core';
import { Toaster as Sonner } from 'vue-sonner';

const props = defineProps<ToasterProps>();
const delegatedProps = reactiveOmit(props, 'toastOptions');
</script>

<template>
    <Sonner
        class="toaster group"
        :toast-options="{
            classes: {
                toast: 'group toast group-[.toaster]:bg-background group-[.toaster]:text-foreground group-[.toaster]:border-border group-[.toaster]:shadow-lg',
                description: 'group-[.toast]:text-muted-foreground',
                actionButton:
                    'group-[.toast]:bg-primary group-[.toast]:text-primary-foreground',
                cancelButton:
                    'group-[.toast]:bg-muted group-[.toast]:text-muted-foreground',
            },
        }"
        v-bind="delegatedProps"
    />
</template>
