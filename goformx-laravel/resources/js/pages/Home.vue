<script setup lang="ts">
import { Head, Link, usePage } from '@inertiajs/vue3';
import { FileText, Zap, Shield, Code, Database, Globe } from 'lucide-vue-next';
import { computed } from 'vue';
import JsonLdScript from '@/components/JsonLdScript.vue';
import PublicHeader from '@/components/PublicHeader.vue';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { dashboard, demo, login, register } from '@/routes';

defineProps<{
    canRegister: boolean;
}>();

const page = usePage();
const seo = computed(() => page.props.seo);
const title = 'GoFormX – Your Forms, Our Backend';
const description =
    'Build and host forms with a visual dashboard and form builder. RESTful API and public submit for embeds. Self-hosted, no lock-in.';
const jsonLd = computed(() => ({
    '@context': 'https://schema.org',
    '@type': 'WebApplication',
    name: 'GoFormX',
    url: seo.value.appUrl,
    description,
    applicationCategory: 'DeveloperApplication',
}));

const features = [
    {
        icon: FileText,
        title: 'Visual Form Builder',
        description:
            'Create forms visually with drag-and-drop. No coding required.',
    },
    {
        icon: Zap,
        title: 'Real-time Validation',
        description: 'Instant feedback with validation schemas.',
    },
    {
        icon: Shield,
        title: 'Secure by Default',
        description:
            'Built-in CSRF protection, rate limiting, and input sanitization.',
    },
    {
        icon: Code,
        title: 'API-First Design',
        description: 'RESTful API for embedding forms anywhere on the web.',
    },
    {
        icon: Database,
        title: 'Self-Hosted',
        description:
            'Your data stays on your servers. Full control and privacy.',
    },
    {
        icon: Globe,
        title: 'CORS Support',
        description:
            'Embed forms on any domain with configurable CORS policies.',
    },
];
</script>

<template>
    <div class="flex min-h-screen flex-col bg-background text-foreground">
        <Head :title="title">
            <meta name="description" :content="description" />
            <link rel="canonical" :href="seo.currentUrl" />
            <meta property="og:type" content="website" />
            <meta property="og:title" :content="title" />
            <meta property="og:description" :content="description" />
            <meta property="og:url" :content="seo.currentUrl" />
            <meta property="og:site_name" :content="page.props.name" />
            <meta
                v-if="seo.defaultOgImage"
                property="og:image"
                :content="seo.defaultOgImage"
            />
            <meta
                name="twitter:card"
                :content="
                    seo.defaultOgImage ? 'summary_large_image' : 'summary'
                "
            />
            <meta name="twitter:title" :content="title" />
            <meta name="twitter:description" :content="description" />
            <meta
                v-if="seo.defaultOgImage"
                name="twitter:image"
                :content="seo.defaultOgImage"
            />
        </Head>
        <Teleport to="head">
            <JsonLdScript :data="jsonLd" />
        </Teleport>

        <PublicHeader :can-register="canRegister" />

        <main class="flex-1">
            <!-- Hero -->
            <section
                class="relative flex min-h-[calc(100vh-4rem)] items-center overflow-hidden py-20 md:py-32"
            >
                <div
                    class="absolute inset-0 bg-[linear-gradient(to_bottom,hsl(var(--background)),hsl(var(--muted)/0.4))]"
                />
                <div
                    class="absolute inset-0 bg-[radial-gradient(ellipse_80%_60%_at_50%_0%,hsl(var(--brand)/0.08),transparent_50%)]"
                />
                <div
                    class="absolute right-0 bottom-0 left-0 h-px bg-gradient-to-r from-transparent via-border to-transparent"
                />
                <div class="relative z-10 container">
                    <div class="flex flex-col items-center text-center">
                        <h1
                            class="[animation:hero-in_0.6s_ease-out_both] font-display text-4xl font-semibold tracking-tight sm:text-5xl md:text-6xl lg:text-7xl"
                        >
                            <span class="text-foreground">Your Forms,</span>
                            <br />
                            <span
                                class="[animation:hero-in_0.6s_ease-out_0.08s_both] text-[hsl(var(--brand))]"
                            >
                                Our Backend
                            </span>
                        </h1>
                        <p
                            class="mt-6 max-w-[42rem] [animation:hero-in_0.5s_ease-out_0.15s_both] text-lg text-muted-foreground sm:text-xl"
                        >
                            Build and host forms. Visual dashboard and form
                            builder; form API and public submit for embeds.
                        </p>
                        <p
                            class="mt-2 [animation:hero-in_0.5s_ease-out_0.2s_both] text-sm text-muted-foreground/70"
                        >
                            No lock-in. Self-host or use the API.
                        </p>
                        <div
                            class="mt-10 flex [animation:hero-in_0.5s_ease-out_0.25s_both] flex-col gap-4 sm:flex-row"
                        >
                            <Button
                                v-if="canRegister"
                                size="lg"
                                variant="brand"
                                as-child
                            >
                                <Link :href="register()">Get started</Link>
                            </Button>
                            <Button v-else size="lg" variant="brand" as-child>
                                <Link :href="login()">Log in</Link>
                            </Button>
                            <Button
                                size="lg"
                                variant="outline"
                                class="border-border/50 bg-background/50 backdrop-blur hover:bg-background/80"
                                as-child
                            >
                                <Link :href="demo().url">Try the demo</Link>
                            </Button>
                            <Button
                                size="lg"
                                variant="outline"
                                class="border-border/50 bg-background/50 backdrop-blur hover:bg-background/80"
                                as-child
                            >
                                <a
                                    href="https://github.com/goformx/goformx"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                >
                                    View on GitHub
                                </a>
                            </Button>
                        </div>
                    </div>
                </div>
            </section>

            <!-- Features -->
            <section class="relative py-24">
                <div
                    class="absolute inset-0 bg-gradient-to-b from-transparent via-muted/20 to-transparent"
                />
                <div class="relative z-10 container">
                    <div class="mb-14 text-center">
                        <h2
                            class="font-display text-3xl font-semibold tracking-tight sm:text-4xl"
                        >
                            Everything You Need
                        </h2>
                        <p class="mt-4 text-lg text-muted-foreground">
                            Powerful features to build and manage forms at
                            scale.
                        </p>
                    </div>
                    <div class="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
                        <Card
                            v-for="(feature, i) in features"
                            :key="feature.title"
                            class="border-border/50 bg-card/80 backdrop-blur-sm transition-all duration-300 hover:border-border hover:bg-card hover:shadow-md"
                            :style="{
                                animation: `card-in 0.5s ease-out ${0.05 * i}s both`,
                            }"
                        >
                            <CardHeader>
                                <div
                                    class="mb-2 flex h-12 w-12 items-center justify-center rounded-lg bg-[hsl(var(--brand)/0.12)] text-[hsl(var(--brand))]"
                                >
                                    <component
                                        :is="feature.icon"
                                        class="h-6 w-6"
                                    />
                                </div>
                                <CardTitle>{{ feature.title }}</CardTitle>
                                <CardDescription>
                                    {{ feature.description }}
                                </CardDescription>
                            </CardHeader>
                        </Card>
                    </div>
                </div>
            </section>

            <!-- CTA -->
            <section class="relative py-24">
                <div
                    class="absolute inset-0 bg-[radial-gradient(ellipse_70%_50%_at_50%_100%,hsl(var(--brand)/0.06),transparent)]"
                />
                <div class="relative z-10 container">
                    <div class="flex flex-col items-center text-center">
                        <h2
                            class="font-display text-3xl font-semibold tracking-tight sm:text-4xl"
                        >
                            Ready to Get Started?
                        </h2>
                        <p
                            class="mt-4 max-w-[42rem] text-lg text-muted-foreground"
                        >
                            Create your first form in minutes. No credit card
                            required.
                        </p>
                        <div class="mt-10">
                            <Button
                                v-if="$page.props.auth?.user"
                                size="lg"
                                variant="brand"
                                as-child
                            >
                                <Link :href="dashboard()">Go to Dashboard</Link>
                            </Button>
                            <Button
                                v-else-if="canRegister"
                                size="lg"
                                variant="brand"
                                as-child
                            >
                                <Link :href="register()"
                                    >Create Your Account</Link
                                >
                            </Button>
                            <Button v-else size="lg" variant="brand" as-child>
                                <Link :href="login()">Log in</Link>
                            </Button>
                        </div>
                    </div>
                </div>
            </section>
        </main>
    </div>
</template>

<style scoped>
@keyframes hero-in {
    from {
        opacity: 0;
        transform: translateY(0.75rem);
    }
    to {
        opacity: 1;
        transform: translateY(0);
    }
}
@keyframes card-in {
    from {
        opacity: 0;
        transform: translateY(0.5rem);
    }
    to {
        opacity: 1;
        transform: translateY(0);
    }
}
</style>
