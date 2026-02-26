<script setup lang="ts">
import { Head, Link, usePage } from '@inertiajs/vue3';
import { computed } from 'vue';
import PublicFooter from '@/components/PublicFooter.vue';
import PublicHeader from '@/components/PublicHeader.vue';
import { terms } from '@/routes';

const page = usePage();
const seo = computed(() => page.props.seo);

const tocItems = [
    {
        id: 'information-we-collect',
        label: 'Information We Collect',
    },
    {
        id: 'how-we-use-your-information',
        label: 'How We Use Your Information',
    },
    {
        id: 'legal-bases',
        label: 'Legal Bases (GDPR)',
    },
    {
        id: 'sharing-and-third-parties',
        label: 'Sharing & Third Parties',
    },
    {
        id: 'international-data-transfers',
        label: 'International Data Transfers',
    },
    {
        id: 'data-retention',
        label: 'Data Retention',
    },
    {
        id: 'your-rights',
        label: 'Your Rights',
    },
    {
        id: 'cookies-and-tracking',
        label: 'Cookies & Tracking',
    },
    {
        id: 'childrens-privacy',
        label: "Children's Privacy",
    },
    {
        id: 'changes-to-this-policy',
        label: 'Changes to This Policy',
    },
    {
        id: 'contact-us',
        label: 'Contact Us',
    },
];
</script>

<template>
    <div class="flex min-h-screen flex-col bg-background text-foreground">
        <Head title="Privacy Policy">
            <meta
                name="description"
                content="GoFormX privacy policy covering data collection, usage, retention, and your rights under PIPEDA, Quebec Law 25, GDPR, and CCPA."
            />
            <link rel="canonical" :href="seo.currentUrl" />
        </Head>

        <PublicHeader />

        <main class="flex-1">
            <div class="container px-4 py-12 sm:px-6 lg:px-8 lg:py-16">
                <!-- Page header -->
                <div class="mb-10">
                    <h1
                        class="text-3xl font-semibold tracking-tight sm:text-4xl"
                    >
                        Privacy Policy
                    </h1>
                    <p class="mt-2 text-muted-foreground">
                        Last updated: February 26, 2026
                    </p>
                </div>

                <!-- Mobile TOC -->
                <nav
                    class="mb-8 rounded-lg border border-border/50 p-4 lg:hidden"
                    aria-label="Table of contents"
                >
                    <p class="mb-3 text-sm font-medium">On this page</p>
                    <ul
                        class="grid grid-cols-1 gap-x-4 gap-y-1.5 text-sm sm:grid-cols-2"
                    >
                        <li v-for="item in tocItems" :key="item.id">
                            <a
                                :href="`#${item.id}`"
                                class="text-muted-foreground hover:text-foreground"
                            >
                                {{ item.label }}
                            </a>
                        </li>
                    </ul>
                </nav>

                <div class="lg:grid lg:grid-cols-[220px_1fr] lg:gap-12">
                    <!-- Sticky TOC sidebar (desktop only) -->
                    <nav class="max-lg:hidden" aria-label="Table of contents">
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
                                GoFormX ("we", "us", or "our") operates the
                                GoFormX platform, a forms management service
                                available at goformx.com. This Privacy Policy
                                explains how we collect, use, disclose, and
                                protect your personal information when you use
                                our service.
                            </p>
                            <p>
                                GoFormX is operated from Canada. By using our
                                service, you agree to the collection and use of
                                information in accordance with this policy.
                            </p>
                        </div>

                        <!-- 1. Information We Collect -->
                        <section id="information-we-collect">
                            <h2 class="text-xl font-semibold">
                                1. Information We Collect
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <h3 class="font-medium text-foreground">
                                    Account Data
                                </h3>
                                <p>
                                    When you create an account, we collect your
                                    name and email address. If you enable
                                    two-factor authentication, we store the
                                    associated recovery codes in encrypted form.
                                </p>

                                <h3 class="font-medium text-foreground">
                                    Billing Data
                                </h3>
                                <p>
                                    Payment processing is handled entirely by
                                    Stripe. We receive and store a Stripe
                                    customer token and subscription metadata. We
                                    never receive, process, or store your credit
                                    card number, CVV, or full billing card
                                    details.
                                </p>

                                <h3 class="font-medium text-foreground">
                                    Form Data
                                </h3>
                                <p>
                                    We store the form schemas you create and the
                                    submissions your forms receive. This data is
                                    stored in our PostgreSQL database and is
                                    associated with your account.
                                </p>

                                <h3 class="font-medium text-foreground">
                                    Usage Data
                                </h3>
                                <p>
                                    We automatically collect certain information
                                    when you access our service, including your
                                    IP address, browser type and version, device
                                    type, pages visited, and the date and time
                                    of your visit. This data helps us maintain
                                    and improve the service.
                                </p>

                                <h3 class="font-medium text-foreground">
                                    Cookies
                                </h3>
                                <p>
                                    We use session cookies to maintain your
                                    authenticated session and may use analytics
                                    cookies to understand how the service is
                                    used. See
                                    <a
                                        href="#cookies-and-tracking"
                                        class="text-foreground underline underline-offset-4 hover:text-foreground/80"
                                        >Section 8</a
                                    >
                                    for details.
                                </p>
                            </div>
                        </section>

                        <!-- 2. How We Use Your Information -->
                        <section id="how-we-use-your-information">
                            <h2 class="text-xl font-semibold">
                                2. How We Use Your Information
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    We use the information we collect for the
                                    following purposes:
                                </p>
                                <ul class="list-disc space-y-2 pl-6">
                                    <li>
                                        <strong class="text-foreground"
                                            >Provide and maintain the
                                            service</strong
                                        >
                                        — operating your account, storing your
                                        forms, and processing submissions.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Process payments</strong
                                        >
                                        — managing your subscription through
                                        Stripe and maintaining billing records.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Send transactional emails</strong
                                        >
                                        — account verification, password resets,
                                        security alerts, and subscription
                                        changes.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Improve the product</strong
                                        >
                                        — analyzing usage patterns to improve
                                        features, performance, and reliability.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Ensure security</strong
                                        >
                                        — detecting and preventing fraud, abuse,
                                        and unauthorized access.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Comply with legal
                                            obligations</strong
                                        >
                                        — meeting tax, regulatory, and
                                        law-enforcement requirements.
                                    </li>
                                </ul>
                                <p>
                                    We do not use your information for automated
                                    decision-making or profiling.
                                </p>
                            </div>
                        </section>

                        <!-- 3. Legal Bases for Processing (GDPR) -->
                        <section id="legal-bases">
                            <h2 class="text-xl font-semibold">
                                3. Legal Bases for Processing (GDPR)
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    If you are located in the European Economic
                                    Area (EEA), we process your personal data
                                    under the following legal bases:
                                </p>
                                <div class="overflow-x-auto">
                                    <table class="w-full text-sm">
                                        <thead>
                                            <tr
                                                class="border-b border-border/50"
                                            >
                                                <th
                                                    class="py-3 pr-4 text-left font-medium text-foreground"
                                                >
                                                    Legal Basis
                                                </th>
                                                <th
                                                    class="py-3 text-left font-medium text-foreground"
                                                >
                                                    Processing Activity
                                                </th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            <tr
                                                class="border-b border-border/50"
                                            >
                                                <td
                                                    class="py-3 pr-4 font-medium text-foreground"
                                                >
                                                    Contract Performance
                                                </td>
                                                <td class="py-3">
                                                    Account creation, form
                                                    storage, submission
                                                    processing, and payment
                                                    handling necessary to
                                                    deliver the service you
                                                    subscribed to.
                                                </td>
                                            </tr>
                                            <tr
                                                class="border-b border-border/50"
                                            >
                                                <td
                                                    class="py-3 pr-4 font-medium text-foreground"
                                                >
                                                    Legitimate Interests
                                                </td>
                                                <td class="py-3">
                                                    Service improvement, usage
                                                    analytics, and security
                                                    monitoring, balanced against
                                                    your privacy rights.
                                                </td>
                                            </tr>
                                            <tr
                                                class="border-b border-border/50"
                                            >
                                                <td
                                                    class="py-3 pr-4 font-medium text-foreground"
                                                >
                                                    Consent
                                                </td>
                                                <td class="py-3">
                                                    Optional analytics cookies,
                                                    where applicable. You may
                                                    withdraw consent at any
                                                    time.
                                                </td>
                                            </tr>
                                            <tr>
                                                <td
                                                    class="py-3 pr-4 font-medium text-foreground"
                                                >
                                                    Legal Obligation
                                                </td>
                                                <td class="py-3">
                                                    Retaining billing records
                                                    for tax compliance and
                                                    responding to lawful
                                                    requests from authorities.
                                                </td>
                                            </tr>
                                        </tbody>
                                    </table>
                                </div>
                            </div>
                        </section>

                        <!-- 4. Sharing and Third Parties -->
                        <section id="sharing-and-third-parties">
                            <h2 class="text-xl font-semibold">
                                4. Sharing and Third Parties
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    We do not sell, rent, or trade your personal
                                    data. We share data only with the following
                                    third-party processors, each bound by data
                                    processing agreements:
                                </p>
                                <div class="overflow-x-auto">
                                    <table class="w-full text-sm">
                                        <thead>
                                            <tr
                                                class="border-b border-border/50"
                                            >
                                                <th
                                                    class="py-3 pr-4 text-left font-medium text-foreground"
                                                >
                                                    Processor
                                                </th>
                                                <th
                                                    class="py-3 pr-4 text-left font-medium text-foreground"
                                                >
                                                    Purpose
                                                </th>
                                                <th
                                                    class="py-3 text-left font-medium text-foreground"
                                                >
                                                    Country
                                                </th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            <tr
                                                class="border-b border-border/50"
                                            >
                                                <td
                                                    class="py-3 pr-4 font-medium text-foreground"
                                                >
                                                    Stripe
                                                </td>
                                                <td class="py-3 pr-4">
                                                    Payment processing and
                                                    subscription management
                                                </td>
                                                <td class="py-3">
                                                    United States
                                                </td>
                                            </tr>
                                            <tr
                                                class="border-b border-border/50"
                                            >
                                                <td
                                                    class="py-3 pr-4 font-medium text-foreground"
                                                >
                                                    Hosting Provider
                                                </td>
                                                <td class="py-3 pr-4">
                                                    Infrastructure, servers, and
                                                    database hosting
                                                </td>
                                                <td class="py-3">Canada</td>
                                            </tr>
                                            <tr>
                                                <td
                                                    class="py-3 pr-4 font-medium text-foreground"
                                                >
                                                    Email Service
                                                </td>
                                                <td class="py-3 pr-4">
                                                    Transactional email delivery
                                                    (verification, alerts,
                                                    receipts)
                                                </td>
                                                <td class="py-3">
                                                    United States
                                                </td>
                                            </tr>
                                        </tbody>
                                    </table>
                                </div>
                                <p>
                                    We may also disclose your information if
                                    required by law, court order, or government
                                    request, or if necessary to protect our
                                    rights, safety, or property.
                                </p>
                            </div>
                        </section>

                        <!-- 5. International Data Transfers -->
                        <section id="international-data-transfers">
                            <h2 class="text-xl font-semibold">
                                5. International Data Transfers
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    GoFormX infrastructure is primarily hosted
                                    in Canada. However, some third-party
                                    processors (Stripe, email service) operate
                                    in the United States, which means your data
                                    may be transferred to and processed in the
                                    US.
                                </p>
                                <h3 class="font-medium text-foreground">
                                    For EU/EEA Users
                                </h3>
                                <p>
                                    Where data is transferred outside the EEA,
                                    we rely on Standard Contractual Clauses
                                    (SCCs) approved by the European Commission
                                    to ensure an adequate level of protection
                                    for your personal data.
                                </p>
                                <h3 class="font-medium text-foreground">
                                    For Canadian Users
                                </h3>
                                <p>
                                    In accordance with PIPEDA and the Office of
                                    the Privacy Commissioner's guidance on
                                    cross-border transfers, we are transparent
                                    about the countries where your data may be
                                    processed. Our third-party processors are
                                    contractually bound to protect your
                                    information to a standard comparable to
                                    Canadian privacy law.
                                </p>
                            </div>
                        </section>

                        <!-- 6. Data Retention -->
                        <section id="data-retention">
                            <h2 class="text-xl font-semibold">
                                6. Data Retention
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    We retain your data only as long as
                                    necessary for the purposes described in this
                                    policy:
                                </p>
                                <ul class="list-disc space-y-2 pl-6">
                                    <li>
                                        <strong class="text-foreground"
                                            >Account data</strong
                                        >
                                        — retained while your account is active,
                                        plus 30 days after deletion to allow for
                                        account recovery.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Billing records</strong
                                        >
                                        — retained for 7 years after the
                                        transaction date, as required by
                                        Canadian and applicable tax law.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Form schemas and
                                            submissions</strong
                                        >
                                        — deleted when you delete the associated
                                        form, or when your account is deleted.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Server logs</strong
                                        >
                                        — automatically purged after 90 days.
                                    </li>
                                </ul>
                            </div>
                        </section>

                        <!-- 7. Your Rights -->
                        <section id="your-rights">
                            <h2 class="text-xl font-semibold">
                                7. Your Rights
                            </h2>
                            <div class="mt-4 space-y-6 text-muted-foreground">
                                <h3 class="font-medium text-foreground">
                                    All Users
                                </h3>
                                <p>
                                    Regardless of your location, you have the
                                    right to:
                                </p>
                                <ul class="list-disc space-y-2 pl-6">
                                    <li>
                                        <strong class="text-foreground"
                                            >Access</strong
                                        >
                                        your personal data we hold.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Correct</strong
                                        >
                                        inaccurate or incomplete data.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Delete</strong
                                        >
                                        your account and associated data.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Export</strong
                                        >
                                        your data in a portable format.
                                    </li>
                                </ul>

                                <h3 class="font-medium text-foreground">
                                    Canada (PIPEDA)
                                </h3>
                                <p>
                                    Under the Personal Information Protection
                                    and Electronic Documents Act, you
                                    additionally have the right to:
                                </p>
                                <ul class="list-disc space-y-2 pl-6">
                                    <li>
                                        Withdraw consent for data processing at
                                        any time, subject to legal or
                                        contractual restrictions.
                                    </li>
                                    <li>
                                        File a complaint with the
                                        <strong class="text-foreground"
                                            >Office of the Privacy Commissioner
                                            of Canada (OPC)</strong
                                        >
                                        if you believe your privacy rights have
                                        been violated.
                                    </li>
                                </ul>

                                <h3 class="font-medium text-foreground">
                                    Quebec (Law 25)
                                </h3>
                                <p>
                                    If you are a resident of Quebec, you have
                                    additional rights under the Act Respecting
                                    the Protection of Personal Information in
                                    the Private Sector (Law 25), including:
                                </p>
                                <ul class="list-disc space-y-2 pl-6">
                                    <li>
                                        The right to
                                        <strong class="text-foreground"
                                            >de-indexation</strong
                                        >
                                        — you may request that we cease
                                        disseminating your personal information
                                        or that any hyperlink attached to your
                                        name be de-indexed, where the
                                        dissemination contravenes the law or a
                                        court order.
                                    </li>
                                    <li>
                                        The right to be
                                        <strong class="text-foreground"
                                            >notified of privacy
                                            incidents</strong
                                        >
                                        — in the event of a confidentiality
                                        incident involving your personal
                                        information that presents a risk of
                                        serious injury, we will notify you and
                                        the Commission d'acces a l'information
                                        du Quebec.
                                    </li>
                                </ul>

                                <h3 class="font-medium text-foreground">
                                    EU/EEA (GDPR)
                                </h3>
                                <p>
                                    If you are located in the European Economic
                                    Area, you additionally have the right to:
                                </p>
                                <ul class="list-disc space-y-2 pl-6">
                                    <li>
                                        <strong class="text-foreground"
                                            >Restrict processing</strong
                                        >
                                        of your personal data in certain
                                        circumstances.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Object to processing</strong
                                        >
                                        based on legitimate interests.
                                    </li>
                                    <li>
                                        Lodge a complaint with your local
                                        <strong class="text-foreground"
                                            >supervisory authority</strong
                                        >.
                                    </li>
                                </ul>

                                <h3 class="font-medium text-foreground">
                                    California (CCPA)
                                </h3>
                                <p>
                                    If you are a California resident, you have
                                    the following rights under the California
                                    Consumer Privacy Act:
                                </p>
                                <ul class="list-disc space-y-2 pl-6">
                                    <li>
                                        <strong class="text-foreground"
                                            >Right to know</strong
                                        >
                                        what personal information we collect,
                                        use, disclose, and sell.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Right to delete</strong
                                        >
                                        personal information we hold about you.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Right to opt-out of sale</strong
                                        >
                                        — we do not sell your personal
                                        information. We have never sold personal
                                        information and have no plans to do so.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >Right to non-discrimination</strong
                                        >
                                        — we will not discriminate against you
                                        for exercising any of your CCPA rights.
                                    </li>
                                </ul>

                                <p>
                                    To exercise any of these rights, contact us
                                    at
                                    <a
                                        href="mailto:privacy@goformx.com"
                                        class="text-foreground underline underline-offset-4 hover:text-foreground/80"
                                        >privacy@goformx.com</a
                                    >.
                                </p>
                            </div>
                        </section>

                        <!-- 8. Cookies and Tracking -->
                        <section id="cookies-and-tracking">
                            <h2 class="text-xl font-semibold">
                                8. Cookies and Tracking
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <h3 class="font-medium text-foreground">
                                    Session Cookies (Essential)
                                </h3>
                                <p>
                                    We use session cookies to authenticate your
                                    login, maintain your session state, and
                                    protect against cross-site request forgery
                                    (CSRF). These cookies are strictly necessary
                                    for the service to function and cannot be
                                    disabled.
                                </p>

                                <h3 class="font-medium text-foreground">
                                    Analytics Cookies
                                </h3>
                                <p>
                                    We may use analytics cookies to understand
                                    how visitors interact with our service. If
                                    analytics cookies are used, they will be
                                    clearly disclosed here and consent will be
                                    obtained where required by law.
                                </p>

                                <h3 class="font-medium text-foreground">
                                    Third-Party Marketing Trackers
                                </h3>
                                <p>
                                    We do not use any third-party marketing or
                                    advertising trackers. No data is shared with
                                    advertising networks.
                                </p>
                            </div>
                        </section>

                        <!-- 9. Children's Privacy -->
                        <section id="childrens-privacy">
                            <h2 class="text-xl font-semibold">
                                9. Children's Privacy
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    GoFormX is not directed at individuals under
                                    the age of 16. We do not knowingly collect
                                    personal information from children under 16.
                                    If we become aware that we have collected
                                    personal data from a child under 16 without
                                    verified parental consent, we will take
                                    steps to delete that information promptly.
                                </p>
                                <p>
                                    If you believe we have inadvertently
                                    collected information from a child under 16,
                                    please contact us at
                                    <a
                                        href="mailto:privacy@goformx.com"
                                        class="text-foreground underline underline-offset-4 hover:text-foreground/80"
                                        >privacy@goformx.com</a
                                    >.
                                </p>
                            </div>
                        </section>

                        <!-- 10. Changes to This Policy -->
                        <section id="changes-to-this-policy">
                            <h2 class="text-xl font-semibold">
                                10. Changes to This Policy
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    We may update this Privacy Policy from time
                                    to time. When we make material changes, we
                                    will notify you by email at least 30 days
                                    before the changes take effect. We will also
                                    update the "Last updated" date at the top of
                                    this page.
                                </p>
                                <p>
                                    Non-material changes (such as formatting or
                                    clarifications that do not affect your
                                    rights) may be made without advance notice.
                                    We encourage you to review this page
                                    periodically.
                                </p>
                            </div>
                        </section>

                        <!-- 11. Contact Us -->
                        <section id="contact-us">
                            <h2 class="text-xl font-semibold">
                                11. Contact Us
                            </h2>
                            <div class="mt-4 space-y-4 text-muted-foreground">
                                <p>
                                    If you have questions about this Privacy
                                    Policy or wish to exercise your privacy
                                    rights, contact us at:
                                </p>
                                <p>
                                    <a
                                        href="mailto:privacy@goformx.com"
                                        class="text-foreground underline underline-offset-4 hover:text-foreground/80"
                                        >privacy@goformx.com</a
                                    >
                                </p>
                                <p>We aim to respond within:</p>
                                <ul class="list-disc space-y-2 pl-6">
                                    <li>
                                        <strong class="text-foreground"
                                            >30 days</strong
                                        >
                                        for requests under PIPEDA or GDPR.
                                    </li>
                                    <li>
                                        <strong class="text-foreground"
                                            >45 days</strong
                                        >
                                        for requests under the CCPA.
                                    </li>
                                </ul>
                                <p>
                                    If we need additional time, we will notify
                                    you of the extension and the reason for it.
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
                            :href="terms().url"
                            class="text-foreground underline underline-offset-4 hover:text-foreground/80"
                            >Terms of Service</Link
                        >.
                    </p>
                </div>
            </div>
        </main>

        <PublicFooter />
    </div>
</template>
