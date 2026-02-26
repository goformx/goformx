<script setup lang="ts">
import { Head, Link, usePage } from '@inertiajs/vue3';
import { computed } from 'vue';
import PublicFooter from '@/components/PublicFooter.vue';
import PublicHeader from '@/components/PublicHeader.vue';

const page = usePage();
const seo = computed(() => page.props.seo);

const tocItems = [
    {
        id: 'acceptance-of-terms',
        label: 'Acceptance of Terms',
    },
    {
        id: 'description-of-service',
        label: 'Description of Service',
    },
    {
        id: 'your-account',
        label: 'Your Account',
    },
    {
        id: 'acceptable-use',
        label: 'Acceptable Use',
    },
    {
        id: 'intellectual-property',
        label: 'Intellectual Property',
    },
    {
        id: 'payment-and-billing',
        label: 'Payment and Billing',
    },
    {
        id: 'data-and-privacy',
        label: 'Data and Privacy',
    },
    {
        id: 'service-availability',
        label: 'Service Availability',
    },
    {
        id: 'limitation-of-liability',
        label: 'Limitation of Liability',
    },
    {
        id: 'termination',
        label: 'Termination',
    },
    {
        id: 'governing-law-and-disputes',
        label: 'Governing Law & Disputes',
    },
    {
        id: 'changes-to-these-terms',
        label: 'Changes to These Terms',
    },
    {
        id: 'contact-us',
        label: 'Contact Us',
    },
];
</script>

<template>
    <div class="flex min-h-screen flex-col bg-background text-foreground">
        <Head title="Terms of Service">
            <meta
                name="description"
                content="GoFormX terms of service covering account usage, acceptable use, intellectual property, billing, liability, and governing law."
            />
            <link rel="canonical" :href="seo.currentUrl" />
        </Head>

        <PublicHeader />

        <main class="flex-1">
            <div class="container py-12 lg:py-16">
                <!-- Page header -->
                <div class="mb-10">
                    <h1
                        class="text-3xl font-semibold tracking-tight sm:text-4xl"
                    >
                        Terms of Service
                    </h1>
                    <p class="mt-2 text-muted-foreground">
                        Last updated: February 26, 2026
                    </p>
                </div>

                <div class="lg:grid lg:grid-cols-[220px_1fr] lg:gap-12">
                    <!-- Sticky TOC sidebar (hidden on mobile) -->
                    <nav class="hidden lg:block" aria-label="Table of contents">
                        <div class="sticky top-8">
                            <ul class="space-y-2 text-sm">
                                <li v-for="item in tocItems" :key="item.id">
                                    <a
                                        :href="`#${item.id}`"
                                        class="text-muted-foreground hover:text-foreground"
                                    >
                                        {{ item.label }}
                                    </a>
                                </li>
                            </ul>
                        </div>
                    </nav>

                    <!-- Content -->
                    <div class="space-y-10">
                        <!-- Intro -->
                        <div class="space-y-4 text-muted-foreground">
                            <p>
                                These Terms of Service ("Terms") govern your
                                access to and use of the GoFormX platform
                                ("Service"), operated by GoFormX ("we", "us", or
                                "our"). Please read these Terms carefully before
                                using the Service.
                            </p>
                        </div>

                        <!-- 1. Acceptance of Terms -->
                        <section id="acceptance-of-terms">
                            <h2 class="text-xl font-semibold">
                                1. Acceptance of Terms
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    By creating an account, accessing, or using
                                    the Service, you agree to be bound by these
                                    Terms and our
                                    <Link
                                        href="/privacy"
                                        class="text-foreground underline underline-offset-4 hover:text-foreground/80"
                                        >Privacy Policy</Link
                                    >. If you do not agree with any part of
                                    these Terms, you must not use the Service.
                                </p>
                                <p>
                                    You must be at least 16 years of age to use
                                    the Service. By using GoFormX, you represent
                                    and warrant that you meet this age
                                    requirement.
                                </p>
                            </div>
                        </section>

                        <!-- 2. Description of Service -->
                        <section id="description-of-service">
                            <h2 class="text-xl font-semibold">
                                2. Description of Service
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    GoFormX is a forms management platform that
                                    enables you to create, host, and collect
                                    form submissions. The Service includes both
                                    free and paid tiers. The Service consists
                                    of:
                                </p>
                                <ul class="list-disc space-y-2 pl-6">
                                    <li>
                                        A web dashboard for building and
                                        managing forms.
                                    </li>
                                    <li>
                                        A REST API for programmatic form
                                        operations.
                                    </li>
                                    <li>
                                        Public endpoints for form submission and
                                        embedding.
                                    </li>
                                </ul>
                                <p>
                                    We reserve the right to modify, suspend, or
                                    discontinue any part of the Service at any
                                    time, with reasonable notice where
                                    practicable.
                                </p>
                            </div>
                        </section>

                        <!-- 3. Your Account -->
                        <section id="your-account">
                            <h2 class="text-xl font-semibold">
                                3. Your Account
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    To use certain features of the Service, you
                                    must create an account. When you create an
                                    account, you agree to:
                                </p>
                                <ul class="list-disc space-y-2 pl-6">
                                    <li>
                                        Provide accurate, current, and complete
                                        information during registration.
                                    </li>
                                    <li>
                                        Maintain and promptly update your
                                        account information to keep it accurate.
                                    </li>
                                    <li>
                                        Maintain only one account per person.
                                    </li>
                                    <li>
                                        Keep your login credentials secure and
                                        confidential.
                                    </li>
                                    <li>
                                        Accept responsibility for all activity
                                        that occurs under your account.
                                    </li>
                                </ul>
                                <p>
                                    We strongly recommend enabling two-factor
                                    authentication (2FA) to help protect your
                                    account. You must notify us immediately at
                                    <a
                                        href="mailto:support@goformx.com"
                                        class="text-foreground underline underline-offset-4 hover:text-foreground/80"
                                        >support@goformx.com</a
                                    >
                                    if you become aware of any unauthorized
                                    access to or use of your account.
                                </p>
                            </div>
                        </section>

                        <!-- 4. Acceptable Use -->
                        <section id="acceptable-use">
                            <h2 class="text-xl font-semibold">
                                4. Acceptable Use
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>You agree not to use the Service to:</p>
                                <ul class="list-disc space-y-2 pl-6">
                                    <li>
                                        Submit illegal content through forms.
                                    </li>
                                    <li>
                                        Use forms for phishing, spam, or malware
                                        distribution.
                                    </li>
                                    <li>
                                        Collect sensitive data (including
                                        health, financial, or children's
                                        information) without appropriate legal
                                        safeguards.
                                    </li>
                                    <li>
                                        Circumvent API rate limits or other
                                        technical restrictions.
                                    </li>
                                    <li>
                                        Reverse engineer, decompile, or
                                        disassemble any part of the platform.
                                    </li>
                                    <li>
                                        Impersonate any person or entity, or
                                        misrepresent your affiliation with any
                                        person or entity.
                                    </li>
                                    <li>
                                        Interfere with or disrupt the Service,
                                        its servers, or networks connected to
                                        the Service.
                                    </li>
                                </ul>
                                <p>
                                    Violation of these restrictions may result
                                    in immediate suspension or termination of
                                    your account.
                                </p>
                            </div>
                        </section>

                        <!-- 5. Intellectual Property -->
                        <section id="intellectual-property">
                            <h2 class="text-xl font-semibold">
                                5. Intellectual Property
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <h3 class="font-medium text-foreground">
                                    Our Property
                                </h3>
                                <p>
                                    GoFormX and its original content, features,
                                    and functionality — including all source
                                    code, design, trademarks, and documentation
                                    — are and will remain the exclusive property
                                    of GoFormX and its licensors. The Service is
                                    protected by copyright, trademark, and other
                                    applicable laws.
                                </p>

                                <h3 class="font-medium text-foreground">
                                    Your Content
                                </h3>
                                <p>
                                    You retain full ownership of the form
                                    content you create and the submission data
                                    your forms collect. By using the Service,
                                    you grant us a limited, non-exclusive,
                                    royalty-free license to host, process,
                                    store, and display your content solely for
                                    the purpose of providing and operating the
                                    Service.
                                </p>
                                <p>
                                    This license terminates when you delete your
                                    content or your account, subject to our data
                                    retention policy described in our
                                    <Link
                                        href="/privacy"
                                        class="text-foreground underline underline-offset-4 hover:text-foreground/80"
                                        >Privacy Policy</Link
                                    >.
                                </p>
                            </div>
                        </section>

                        <!-- 6. Payment and Billing -->
                        <section id="payment-and-billing">
                            <h2 class="text-xl font-semibold">
                                6. Payment and Billing
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    All billing is processed through Stripe. By
                                    subscribing to a paid plan, you authorize us
                                    to charge the applicable fees through
                                    Stripe.
                                </p>
                                <ul class="list-disc space-y-2 pl-6">
                                    <li>
                                        <strong class="text-foreground"
                                            >Subscription plans</strong
                                        >
                                        are billed monthly or annually, as
                                        selected at the time of purchase.
                                    </li>
                                    <li>
                                        A
                                        <strong class="text-foreground"
                                            >free tier</strong
                                        >
                                        is available with no payment required.
                                    </li>
                                    <li>
                                        Paid plans begin at the time of
                                        subscription.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Cancellation</strong
                                        >
                                        takes effect at the end of the current
                                        billing period. You will retain access
                                        to paid features until your billing
                                        period expires.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >No refunds</strong
                                        >
                                        are provided for partial billing
                                        periods.
                                    </li>
                                    <li>
                                        We may change pricing with
                                        <strong class="text-foreground"
                                            >30 days written notice</strong
                                        >. Price changes will apply at your next
                                        renewal date.
                                    </li>
                                </ul>
                            </div>
                        </section>

                        <!-- 7. Data and Privacy -->
                        <section id="data-and-privacy">
                            <h2 class="text-xl font-semibold">
                                7. Data and Privacy
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    Your use of GoFormX is also governed by our
                                    <Link
                                        href="/privacy"
                                        class="text-foreground underline underline-offset-4 hover:text-foreground/80"
                                        >Privacy Policy</Link
                                    >, which describes how we collect, process,
                                    retain, and protect your data, as well as
                                    your rights regarding your personal
                                    information.
                                </p>
                                <p>
                                    By using the Service, you acknowledge that
                                    you have read and understood our Privacy
                                    Policy and agree to its terms.
                                </p>
                            </div>
                        </section>

                        <!-- 8. Service Availability -->
                        <section id="service-availability">
                            <h2 class="text-xl font-semibold">
                                8. Service Availability
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    We strive to maintain high availability of
                                    the Service but do not guarantee
                                    uninterrupted or error-free operation.
                                </p>
                                <ul class="list-disc space-y-2 pl-6">
                                    <li>
                                        We may perform scheduled maintenance
                                        with reasonable advance notice.
                                    </li>
                                    <li>
                                        We are not liable for any downtime,
                                        whether planned or unplanned.
                                    </li>
                                    <li>
                                        No formal Service Level Agreement (SLA)
                                        is provided at this time.
                                    </li>
                                </ul>
                            </div>
                        </section>

                        <!-- 9. Limitation of Liability -->
                        <section id="limitation-of-liability">
                            <h2 class="text-xl font-semibold">
                                9. Limitation of Liability
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    The Service is provided "as is" and "as
                                    available" without warranties of any kind,
                                    whether express or implied, including but
                                    not limited to implied warranties of
                                    merchantability, fitness for a particular
                                    purpose, and non-infringement.
                                </p>
                                <p>
                                    To the maximum extent permitted by
                                    applicable law, our total aggregate
                                    liability arising out of or relating to
                                    these Terms or the use of the Service is
                                    limited to the total fees you paid to us in
                                    the twelve (12) months preceding the event
                                    giving rise to the claim.
                                </p>
                                <p>
                                    In no event shall we be liable for any
                                    indirect, incidental, special,
                                    consequential, or punitive damages,
                                    including but not limited to loss of
                                    profits, data, use, or goodwill, regardless
                                    of the cause of action or the theory of
                                    liability.
                                </p>
                            </div>
                        </section>

                        <!-- 10. Termination -->
                        <section id="termination">
                            <h2 class="text-xl font-semibold">
                                10. Termination
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    You may delete your account at any time
                                    through your account settings.
                                </p>
                                <p>
                                    We may suspend or terminate your account if
                                    you materially violate these Terms. We will
                                    provide reasonable notice before taking such
                                    action, except where immediate action is
                                    required — for example, in cases of illegal
                                    activity, security threats, or actions that
                                    could harm other users or the integrity of
                                    the Service.
                                </p>
                                <p>
                                    Upon termination, your data will be handled
                                    in accordance with our data retention policy
                                    as described in our
                                    <Link
                                        href="/privacy"
                                        class="text-foreground underline underline-offset-4 hover:text-foreground/80"
                                        >Privacy Policy</Link
                                    >.
                                </p>
                            </div>
                        </section>

                        <!-- 11. Governing Law and Disputes -->
                        <section id="governing-law-and-disputes">
                            <h2 class="text-xl font-semibold">
                                11. Governing Law and Disputes
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    These Terms are governed by and construed in
                                    accordance with the laws of the Province of
                                    Ontario and the federal laws of Canada
                                    applicable therein, without regard to
                                    conflict of law provisions.
                                </p>
                                <p>
                                    Any disputes arising out of or relating to
                                    these Terms or the Service shall be resolved
                                    in the courts of Ontario, Canada.
                                </p>
                                <p>
                                    Nothing in these Terms limits your statutory
                                    consumer rights under applicable law,
                                    including the General Data Protection
                                    Regulation (GDPR), the California Consumer
                                    Privacy Act (CCPA), or other consumer
                                    protection legislation.
                                </p>
                            </div>
                        </section>

                        <!-- 12. Changes to These Terms -->
                        <section id="changes-to-these-terms">
                            <h2 class="text-xl font-semibold">
                                12. Changes to These Terms
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    We may update these Terms from time to time.
                                    For material changes, we will notify you by
                                    email at least 30 days before the changes
                                    take effect. We will also update the "Last
                                    updated" date at the top of this page.
                                </p>
                                <p>
                                    Continued use of the Service after the
                                    notice period constitutes acceptance of the
                                    updated Terms. If you disagree with the
                                    changes, you may close your account before
                                    they take effect.
                                </p>
                            </div>
                        </section>

                        <!-- 13. Contact Us -->
                        <section id="contact-us">
                            <h2 class="text-xl font-semibold">
                                13. Contact Us
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    If you have any questions about these Terms
                                    of Service, please contact us at:
                                </p>
                                <p>
                                    <a
                                        href="mailto:support@goformx.com"
                                        class="text-foreground underline underline-offset-4 hover:text-foreground/80"
                                        >support@goformx.com</a
                                    >
                                </p>
                            </div>
                        </section>
                    </div>
                </div>

                <!-- Cross-link -->
                <div class="mt-12 border-t border-border/50 pt-8">
                    <p class="text-sm text-muted-foreground">
                        See also our
                        <Link
                            href="/privacy"
                            class="text-foreground underline underline-offset-4 hover:text-foreground/80"
                            >Privacy Policy</Link
                        >.
                    </p>
                </div>
            </div>
        </main>

        <PublicFooter />
    </div>
</template>
