<script setup lang="ts">
import { Head, Link } from '@inertiajs/vue3';
import PublicHeader from '@/components/PublicHeader.vue';
import { dashboard, home } from '@/routes';

defineProps<{
    canRegister?: boolean;
}>();
</script>

<template>
    <div class="flex min-h-screen flex-col bg-background text-foreground">
        <Head title="Demo – Not configured" />

        <PublicHeader :can-register="canRegister" />

        <main
            class="container flex flex-1 flex-col items-center justify-center px-4 py-16 text-center"
        >
            <h1 class="text-xl font-semibold tracking-tight">
                Demo not configured
            </h1>
            <p class="mt-2 max-w-md text-muted-foreground">
                The demo form is not set up. Set
                <code class="rounded bg-muted px-1.5 py-0.5 text-sm"
                    >GOFORMS_DEMO_FORM_ID</code
                >
                in your environment, or create a form in the dashboard and add
                its ID to config.
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
