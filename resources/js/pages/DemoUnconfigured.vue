<script setup lang="ts">
import { Head, Link } from '@inertiajs/vue3';
import { dashboard, home, login, register } from '@/routes';

defineProps<{
    canRegister?: boolean;
}>();
</script>

<template>
    <div
        class="flex min-h-screen flex-col bg-background text-foreground"
    >
        <Head title="Demo – Not configured" />

        <header
            class="w-full border-b border-border/50 bg-background/80 backdrop-blur-sm"
        >
            <nav
                class="container flex items-center justify-end gap-4 px-4 py-4 sm:px-6"
            >
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
            </nav>
        </header>

        <main class="container flex flex-1 flex-col items-center justify-center px-4 py-16 text-center">
            <h1 class="text-xl font-semibold tracking-tight">
                Demo not configured
            </h1>
            <p class="mt-2 max-w-md text-muted-foreground">
                The demo form is not set up. Set
                <code class="rounded bg-muted px-1.5 py-0.5 text-sm">GOFORMS_DEMO_FORM_ID</code>
                in your environment, or create a form in the dashboard and add its ID to config.
            </p>
            <div class="mt-6 flex gap-4">
                <Link
                    :href="home()"
                    class="text-sm font-medium text-primary underline-offset-4 hover:underline"
                >
                    Home
                </Link>
                <Link
                    v-if="$page.props.auth?.user"
                    :href="dashboard()"
                    class="text-sm font-medium text-primary underline-offset-4 hover:underline"
                >
                    Dashboard
                </Link>
            </div>
        </main>
    </div>
</template>
