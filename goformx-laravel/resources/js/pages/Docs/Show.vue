<script setup lang="ts">
import { Head, Link } from '@inertiajs/vue3';
import { BookOpen, ChevronRight, Menu } from 'lucide-vue-next';
import { computed, ref } from 'vue';
import PublicFooter from '@/components/PublicFooter.vue';
import PublicHeader from '@/components/PublicHeader.vue';
import { Button } from '@/components/ui/button';
import {
    Sheet,
    SheetContent,
    SheetHeader,
    SheetTitle,
    SheetTrigger,
} from '@/components/ui/sheet';
import { show } from '@/routes/docs';

type NavEntry = {
    title: string;
    slug: string;
    order: number;
    active: boolean;
};

const props = defineProps<{
    title: string;
    content: string;
    slug: string;
    navigation: NavEntry[];
}>();

const mobileOpen = ref(false);

const currentIndex = computed(() =>
    props.navigation.findIndex((n) => n.active),
);
const prevPage = computed(() =>
    currentIndex.value > 0 ? props.navigation[currentIndex.value - 1] : null,
);
const nextPage = computed(() =>
    currentIndex.value < props.navigation.length - 1
        ? props.navigation[currentIndex.value + 1]
        : null,
);
</script>

<template>
    <div class="flex min-h-screen flex-col bg-background text-foreground">
        <Head :title="`${title} - Docs`" />

        <PublicHeader />

        <div class="container flex flex-1 gap-0 px-4 py-8 sm:px-6 lg:gap-10">
            <!-- Desktop sidebar -->
            <aside class="hidden w-56 shrink-0 lg:block">
                <nav class="sticky top-8 space-y-1">
                    <p
                        class="mb-3 flex items-center gap-2 text-xs font-semibold tracking-wider text-muted-foreground uppercase"
                    >
                        <BookOpen class="size-3.5" />
                        Documentation
                    </p>
                    <Link
                        v-for="item in navigation"
                        :key="item.slug"
                        :href="show({ slug: item.slug })"
                        class="block rounded-md px-3 py-2 text-sm font-medium transition-colors"
                        :class="
                            item.active
                                ? 'bg-accent text-accent-foreground'
                                : 'text-muted-foreground hover:bg-accent/50 hover:text-foreground'
                        "
                    >
                        {{ item.title }}
                    </Link>
                </nav>
            </aside>

            <!-- Mobile sidebar trigger -->
            <div class="mb-4 lg:hidden">
                <Sheet v-model:open="mobileOpen">
                    <SheetTrigger :as-child="true">
                        <Button variant="outline" size="sm" class="gap-2">
                            <Menu class="size-4" />
                            Docs Menu
                        </Button>
                    </SheetTrigger>
                    <SheetContent side="left" class="w-[260px] p-6">
                        <SheetTitle class="sr-only">
                            Documentation Menu
                        </SheetTitle>
                        <SheetHeader
                            class="mb-4 flex items-center gap-2 text-sm font-semibold"
                        >
                            <BookOpen class="size-4" />
                            Documentation
                        </SheetHeader>
                        <nav class="space-y-1">
                            <Link
                                v-for="item in navigation"
                                :key="item.slug"
                                :href="show({ slug: item.slug })"
                                class="block rounded-md px-3 py-2 text-sm font-medium transition-colors"
                                :class="
                                    item.active
                                        ? 'bg-accent text-accent-foreground'
                                        : 'text-muted-foreground hover:bg-accent/50 hover:text-foreground'
                                "
                                @click="mobileOpen = false"
                            >
                                {{ item.title }}
                            </Link>
                        </nav>
                    </SheetContent>
                </Sheet>
            </div>

            <!-- Content area -->
            <main class="min-w-0 flex-1">
                <article
                    class="prose max-w-none prose-neutral dark:prose-invert prose-headings:font-display prose-headings:tracking-tight prose-a:text-[hsl(var(--brand))] prose-a:no-underline hover:prose-a:underline prose-code:rounded prose-code:bg-muted prose-code:px-1.5 prose-code:py-0.5 prose-code:text-sm prose-code:before:content-none prose-code:after:content-none prose-pre:border prose-pre:border-border prose-pre:bg-muted"
                    v-html="content"
                />

                <!-- Prev / Next navigation -->
                <nav
                    v-if="prevPage || nextPage"
                    class="mt-12 flex items-center justify-between border-t border-border pt-6"
                >
                    <Link
                        v-if="prevPage"
                        :href="show({ slug: prevPage.slug })"
                        class="group flex items-center gap-1 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
                    >
                        <ChevronRight
                            class="size-4 rotate-180 transition-transform group-hover:-translate-x-0.5"
                        />
                        {{ prevPage.title }}
                    </Link>
                    <span v-else />
                    <Link
                        v-if="nextPage"
                        :href="show({ slug: nextPage.slug })"
                        class="group flex items-center gap-1 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
                    >
                        {{ nextPage.title }}
                        <ChevronRight
                            class="size-4 transition-transform group-hover:translate-x-0.5"
                        />
                    </Link>
                </nav>
            </main>
        </div>

        <PublicFooter />
    </div>
</template>
