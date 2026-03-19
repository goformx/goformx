<script setup lang="ts">
import { Link } from '@inertiajs/vue3';
import { Monitor, Palette, Shield, User } from 'lucide-vue-next';
import { useCurrentUrl } from '@/composables/useCurrentUrl';

const currentUrl = useCurrentUrl();

const navItems = [
    { title: 'Profile', href: '/settings/profile', icon: User },
    { title: 'Password', href: '/settings/password', icon: Shield },
    { title: 'Two-Factor Auth', href: '/settings/two-factor', icon: Monitor },
    { title: 'Appearance', href: '/settings/appearance', icon: Palette },
];

function isActive(href: string): boolean {
    return currentUrl.value === href;
}
</script>

<template>
    <div class="px-4 py-6">
        <div class="flex flex-col space-y-8 lg:flex-row lg:space-x-12 lg:space-y-0">
            <aside class="lg:w-1/5">
                <nav class="flex space-x-2 lg:flex-col lg:space-x-0 lg:space-y-1">
                    <Link
                        v-for="item in navItems"
                        :key="item.href"
                        :href="item.href"
                        class="inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors"
                        :class="
                            isActive(item.href)
                                ? 'bg-muted text-foreground'
                                : 'text-muted-foreground hover:bg-muted/50 hover:text-foreground'
                        "
                    >
                        <component :is="item.icon" class="h-4 w-4" />
                        {{ item.title }}
                    </Link>
                </nav>
            </aside>
            <div class="flex-1 lg:max-w-2xl">
                <slot />
            </div>
        </div>
    </div>
</template>
