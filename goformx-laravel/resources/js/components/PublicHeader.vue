<script setup lang="ts">
import { Link } from '@inertiajs/vue3';
import AppLogoIcon from '@/components/AppLogoIcon.vue';
import { dashboard, login, register } from '@/routes';

defineProps<{
    canRegister?: boolean;
}>();
</script>

<template>
    <header
        class="w-full border-b border-border/50 bg-background/80 backdrop-blur-sm"
    >
        <nav
            class="container flex items-center justify-between gap-4 px-4 py-4 sm:px-6"
        >
            <Link href="/" class="flex items-center gap-2">
                <AppLogoIcon class="size-7 text-[hsl(var(--brand))]" />
                <span class="text-lg font-semibold text-foreground"
                    >GoFormX</span
                >
            </Link>
            <div class="flex items-center gap-4">
                <Link
                    v-if="$page.props.auth?.user"
                    :href="dashboard()"
                    class="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
                >
                    Dashboard
                </Link>
                <template v-else>
                    <Link
                        :href="login()"
                        class="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
                    >
                        Log in
                    </Link>
                    <Link
                        v-if="canRegister"
                        :href="register()"
                        class="rounded-md border border-border bg-background px-4 py-2 text-sm font-medium shadow-sm transition-colors hover:bg-muted/50"
                    >
                        Register
                    </Link>
                </template>
            </div>
        </nav>
    </header>
</template>
