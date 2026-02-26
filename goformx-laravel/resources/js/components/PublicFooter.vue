<script setup lang="ts">
import { Link } from '@inertiajs/vue3';
</script>

<template>
    <footer class="border-t border-border/50 bg-background">
        <div
            class="container flex flex-col items-center justify-between gap-4 px-4 py-6 sm:flex-row sm:px-6"
        >
            <p class="text-sm text-muted-foreground">
                &copy; {{ new Date().getFullYear() }} GoFormX. All rights reserved.
            </p>
            <nav class="flex items-center gap-4">
                <Link
                    href="/privacy"
                    class="text-sm text-muted-foreground transition-colors hover:text-foreground"
                >
                    Privacy Policy
                </Link>
                <span class="text-muted-foreground/40">|</span>
                <Link
                    href="/terms"
                    class="text-sm text-muted-foreground transition-colors hover:text-foreground"
                >
                    Terms of Service
                </Link>
                <span class="text-muted-foreground/40">|</span>
                <a
                    href="https://github.com/goformx/goformx"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="text-sm text-muted-foreground transition-colors hover:text-foreground"
                >
                    GitHub
                </a>
            </nav>
        </div>
    </footer>
</template>
