# Documentation Pages Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the default Laravel docs link with in-app, public, markdown-driven documentation pages at `/docs`.

**Architecture:** Single `DocsController` reads markdown files from `resources/docs/`, parses with `league/commonmark`, renders via Inertia into a `Docs/Show.vue` page with a dedicated docs layout featuring sidebar navigation.

**Tech Stack:** Laravel 12, Inertia v2, Vue 3, `league/commonmark` (already installed), `@tailwindcss/typography` (to install), Tailwind CSS v4, shadcn-vue components.

---

### Task 1: Install @tailwindcss/typography

**Files:**
- Modify: `goformx-laravel/package.json`
- Modify: `goformx-laravel/resources/css/app.css`

**Step 1: Install the typography plugin**

Run: `cd goformx-laravel && ddev exec npm install @tailwindcss/typography`

**Step 2: Import the plugin in app.css**

Add after `@import 'tailwindcss';` in `resources/css/app.css`:

```css
@import '@tailwindcss/typography';
```

**Step 3: Verify build works**

Run: `cd goformx-laravel && ddev exec npm run build`
Expected: Build succeeds with no errors.

**Step 4: Commit**

```bash
git add goformx-laravel/package.json goformx-laravel/package-lock.json goformx-laravel/resources/css/app.css
git commit -m "feat(docs): install @tailwindcss/typography for prose styling"
```

---

### Task 2: Create DocsController

**Files:**
- Create: `goformx-laravel/app/Http/Controllers/DocsController.php`
- Modify: `goformx-laravel/routes/web.php`

**Step 1: Write the feature test**

Run: `cd goformx-laravel && ddev exec php artisan make:test DocsPageTest --pest`

Write in `goformx-laravel/tests/Feature/DocsPageTest.php`:

```php
<?php

use function Pest\Laravel\get;

it('renders the getting started docs page', function () {
    get('/docs')
        ->assertOk()
        ->assertInertia(fn ($page) => $page
            ->component('Docs/Show')
            ->has('title')
            ->has('content')
            ->has('navigation')
            ->where('slug', 'getting-started')
        );
});

it('renders a specific docs page by slug', function () {
    get('/docs/form-builder')
        ->assertOk()
        ->assertInertia(fn ($page) => $page
            ->component('Docs/Show')
            ->where('slug', 'form-builder')
        );
});

it('returns 404 for non-existent docs page', function () {
    get('/docs/does-not-exist')
        ->assertNotFound();
});
```

**Step 2: Run test to verify it fails**

Run: `cd goformx-laravel && ddev exec php artisan test --compact --filter=DocsPageTest`
Expected: FAIL (route not defined, controller doesn't exist)

**Step 3: Create the controller**

Run: `cd goformx-laravel && ddev exec php artisan make:class Http/Controllers/DocsController --no-interaction`

Replace contents of `goformx-laravel/app/Http/Controllers/DocsController.php`:

```php
<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;
use Illuminate\Support\Facades\File;
use Inertia\Inertia;
use Inertia\Response;
use League\CommonMark\CommonMarkConverter;
use League\CommonMark\Extension\FrontMatter\FrontMatterExtension;
use League\CommonMark\Extension\FrontMatter\Output\RenderedContentWithFrontMatter;
use League\CommonMark\Environment\Environment;
use League\CommonMark\MarkdownConverter;
use Symfony\Component\HttpKernel\Exception\NotFoundHttpException;

class DocsController extends Controller
{
    private const DEFAULT_SLUG = 'getting-started';

    public function __invoke(Request $request, ?string $slug = null): Response
    {
        $slug = $slug ?? self::DEFAULT_SLUG;
        $path = resource_path("docs/{$slug}.md");

        if (! File::exists($path)) {
            throw new NotFoundHttpException;
        }

        $environment = new Environment([]);
        $environment->addExtension(new FrontMatterExtension);
        $converter = new MarkdownConverter($environment);

        $result = $converter->convert(File::get($path));
        $frontMatter = $result instanceof RenderedContentWithFrontMatter
            ? $result->getFrontMatter()
            : [];

        return Inertia::render('Docs/Show', [
            'title' => $frontMatter['title'] ?? $slug,
            'content' => $result->getContent(),
            'slug' => $slug,
            'navigation' => $this->buildNavigation($slug),
        ]);
    }

    /** @return array<int, array{title: string, slug: string, active: bool}> */
    private function buildNavigation(string $activeSlug): array
    {
        $docsPath = resource_path('docs');
        $files = File::glob("{$docsPath}/*.md");
        $nav = [];

        $environment = new Environment([]);
        $environment->addExtension(new FrontMatterExtension);
        $converter = new MarkdownConverter($environment);

        foreach ($files as $file) {
            $fileSlug = pathinfo($file, PATHINFO_FILENAME);
            $result = $converter->convert(File::get($file));
            $frontMatter = $result instanceof RenderedContentWithFrontMatter
                ? $result->getFrontMatter()
                : [];

            $nav[] = [
                'title' => $frontMatter['title'] ?? $fileSlug,
                'slug' => $fileSlug,
                'order' => $frontMatter['order'] ?? 99,
                'active' => $fileSlug === $activeSlug,
            ];
        }

        usort($nav, fn ($a, $b) => $a['order'] <=> $b['order']);

        return $nav;
    }
}
```

**Step 4: Add the route**

In `goformx-laravel/routes/web.php`, add after the `terms` route (line 57):

```php
use App\Http\Controllers\DocsController;

Route::get('docs/{slug?}', DocsController::class)->name('docs.show');
```

**Step 5: Create placeholder markdown files so tests pass**

Create `goformx-laravel/resources/docs/getting-started.md`:

```markdown
---
title: Getting Started
order: 1
---

# Getting Started

Welcome to GoFormX.
```

Create `goformx-laravel/resources/docs/form-builder.md`:

```markdown
---
title: Form Builder
order: 2
---

# Form Builder

Build forms with drag and drop.
```

**Step 6: Run tests to verify they pass**

Run: `cd goformx-laravel && ddev exec php artisan test --compact --filter=DocsPageTest`
Expected: 3 tests PASS

**Step 7: Run PHP formatting**

Run: `cd goformx-laravel && ddev exec vendor/bin/pint --dirty --format agent`

**Step 8: Commit**

```bash
git add goformx-laravel/app/Http/Controllers/DocsController.php goformx-laravel/routes/web.php goformx-laravel/tests/Feature/DocsPageTest.php goformx-laravel/resources/docs/
git commit -m "feat(docs): add DocsController with markdown parsing and route"
```

---

### Task 3: Create Docs/Show.vue page and DocsLayout

**Files:**
- Create: `goformx-laravel/resources/js/pages/Docs/Show.vue`

**Step 1: Create the docs page**

Create `goformx-laravel/resources/js/pages/Docs/Show.vue`:

```vue
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
    currentIndex.value > 0
        ? props.navigation[currentIndex.value - 1]
        : null,
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
                    class="prose prose-neutral dark:prose-invert max-w-none prose-headings:font-display prose-headings:tracking-tight prose-a:text-[hsl(var(--brand))] prose-a:no-underline hover:prose-a:underline prose-code:rounded prose-code:bg-muted prose-code:px-1.5 prose-code:py-0.5 prose-code:text-sm prose-code:before:content-none prose-code:after:content-none prose-pre:bg-muted prose-pre:border prose-pre:border-border"
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
```

**Step 2: Generate Wayfinder routes**

Run: `cd goformx-laravel && ddev exec php artisan wayfinder:generate`

Verify `resources/js/routes/docs.ts` was generated with a `show` function.

**Step 3: Verify build**

Run: `cd goformx-laravel && ddev exec npm run build`
Expected: Build succeeds.

**Step 4: Commit**

```bash
git add goformx-laravel/resources/js/pages/Docs/Show.vue goformx-laravel/resources/js/routes/
git commit -m "feat(docs): add Docs/Show.vue page with sidebar and prose styling"
```

---

### Task 4: Update documentation links in sidebar and header

**Files:**
- Modify: `goformx-laravel/resources/js/components/AppSidebar.vue` (lines 46-57)
- Modify: `goformx-laravel/resources/js/components/AppHeader.vue` (lines 64-75)
- Modify: `goformx-laravel/resources/js/components/NavFooter.vue` (lines 31-34, change from external `<a>` to Inertia `<Link>`)

**Step 1: Update AppSidebar.vue**

Change the Documentation footer item `href` from `'https://laravel.com/docs/starter-kits#vue'` to use the Wayfinder route:

```typescript
import { show as docsShow } from '@/routes/docs';

const footerNavItems: NavItem[] = [
    {
        title: 'Github Repo',
        href: 'https://github.com/goformx/goformx',
        icon: Folder,
    },
    {
        title: 'Documentation',
        href: docsShow(),
        icon: BookOpen,
    },
];
```

**Step 2: Update AppHeader.vue**

Same change — replace the hardcoded URL:

```typescript
import { show as docsShow } from '@/routes/docs';

const rightNavItems: NavItem[] = [
    {
        title: 'Repository',
        href: 'https://github.com/goformx/goformx',
        icon: Folder,
    },
    {
        title: 'Documentation',
        href: docsShow(),
        icon: BookOpen,
    },
];
```

**Step 3: Update NavFooter.vue to handle internal links**

The NavFooter currently renders all items as external `<a>` tags with `target="_blank"`. The Documentation link now needs to be an Inertia `<Link>`. Update the template to detect internal vs external links:

```vue
<script setup lang="ts">
import { Link } from '@inertiajs/vue3';
import {
    SidebarGroup,
    SidebarGroupContent,
    SidebarMenu,
    SidebarMenuButton,
    SidebarMenuItem,
} from '@/components/ui/sidebar';
import { toUrl } from '@/lib/utils';
import { type NavItem } from '@/types';

type Props = {
    items: NavItem[];
    class?: string;
};

defineProps<Props>();

function isExternal(href: string | { url: string }): boolean {
    const url = typeof href === 'string' ? href : href.url;
    return url.startsWith('http://') || url.startsWith('https://');
}
</script>

<template>
    <SidebarGroup
        :class="`group-data-[collapsible=icon]:p-0 ${$props.class || ''}`"
    >
        <SidebarGroupContent>
            <SidebarMenu>
                <SidebarMenuItem v-for="item in items" :key="item.title">
                    <SidebarMenuButton
                        class="text-neutral-600 hover:text-neutral-800 dark:text-neutral-300 dark:hover:text-neutral-100"
                        as-child
                    >
                        <a
                            v-if="isExternal(item.href)"
                            :href="toUrl(item.href)"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            <component :is="item.icon" />
                            <span>{{ item.title }}</span>
                        </a>
                        <Link v-else :href="item.href">
                            <component :is="item.icon" />
                            <span>{{ item.title }}</span>
                        </Link>
                    </SidebarMenuButton>
                </SidebarMenuItem>
            </SidebarMenu>
        </SidebarGroupContent>
    </SidebarGroup>
</template>
```

**Step 4: Run lint and format**

Run: `cd goformx-laravel && ddev exec npm run lint && ddev exec npm run format`

**Step 5: Build and verify**

Run: `cd goformx-laravel && ddev exec npm run build`
Expected: Build succeeds.

**Step 6: Commit**

```bash
git add goformx-laravel/resources/js/components/AppSidebar.vue goformx-laravel/resources/js/components/AppHeader.vue goformx-laravel/resources/js/components/NavFooter.vue
git commit -m "feat(docs): update Documentation links to use in-app docs route"
```

---

### Task 5: Write the documentation content

**Files:**
- Create/Replace: `goformx-laravel/resources/docs/getting-started.md`
- Create/Replace: `goformx-laravel/resources/docs/form-builder.md`
- Create: `goformx-laravel/resources/docs/embedding-forms.md`
- Create: `goformx-laravel/resources/docs/api-reference.md`
- Create: `goformx-laravel/resources/docs/submissions.md`

**Step 1: Write getting-started.md**

```markdown
---
title: Getting Started
order: 1
---

# Getting Started

GoFormX is a forms platform that lets you build, embed, and collect submissions — no backend code required.

## Create an account

1. Click **Register** in the top-right corner
2. Enter your name, email, and password
3. Verify your email address

## Build your first form

1. From the **Dashboard**, click **Forms** in the sidebar
2. Click **Create Form**
3. Give your form a name and click **Create**
4. You'll land in the **Form Builder** — drag fields from the left sidebar onto the canvas
5. Click **Save** when you're done

## Share your form

Every form gets a public URL you can share directly:

```
https://goformx.com/forms/{form-id}
```

Or embed it in your own site — see [Embedding Forms](/docs/embedding-forms).

## View submissions

When someone fills out your form, submissions appear under **Forms → [Your Form] → Submissions**. See [Submissions](/docs/submissions) for details.

## What's next

- [Form Builder](/docs/form-builder) — learn about field types and validation
- [Embedding Forms](/docs/embedding-forms) — put forms on your website
- [API Reference](/docs/api-reference) — integrate programmatically
```

**Step 2: Write form-builder.md**

```markdown
---
title: Form Builder
order: 2
---

# Form Builder

The form builder uses a drag-and-drop interface to create forms visually.

## Adding fields

The left sidebar contains field groups:

- **Basic** — text field, text area, number, password, checkbox, select, radio
- **Advanced** — email, URL, phone number, date/time, file upload, signature
- **Layout** — columns, panel, tabs, well, fieldset

Drag any field onto the canvas to add it to your form.

## Editing fields

Click any field on the canvas to open the edit dialog. You can configure:

- **Label** — the field's display name
- **Placeholder** — hint text shown when empty
- **Required** — whether the field must be filled
- **Validation** — min/max length, pattern, custom error messages
- **Conditional** — show/hide based on other field values

## Reordering fields

Drag fields up or down on the canvas to reorder them.

## Previewing

Click **Preview** in the toolbar to see how your form looks to users. This opens a read-only view of the form.

## Saving

Click **Save** to persist your changes. The form schema is stored and used when rendering the public form and processing submissions.
```

**Step 3: Write embedding-forms.md**

```markdown
---
title: Embedding Forms
order: 3
---

# Embedding Forms

You can embed any GoFormX form on your website using an iframe or by linking directly.

## Direct link

Every published form is accessible at:

```
https://goformx.com/forms/{form-id}
```

Share this URL via email, social media, or anywhere else.

## Embed with iframe

Add this HTML to your page:

```html
<iframe
  src="https://goformx.com/forms/{form-id}/embed"
  width="100%"
  height="600"
  frameborder="0"
  style="border: none;"
></iframe>
```

The embed endpoint renders the form without the site header and footer for a clean embedded experience.

## Styling tips

- Set `width="100%"` and a fixed height, or use CSS to make the iframe responsive
- The form inherits its own styling — it won't conflict with your site's CSS
- For dark backgrounds, the form supports dark mode automatically
```

**Step 4: Write api-reference.md**

```markdown
---
title: API Reference
order: 4
---

# API Reference

GoFormX exposes public endpoints for fetching form schemas and submitting responses. These endpoints require no authentication and are rate-limited to 60 requests per minute.

## Get form schema

Retrieve the JSON schema for a form.

```
GET /api/forms/{form-id}/schema
```

**Response** `200 OK`

```json
{
  "id": "abc123",
  "title": "Contact Form",
  "schema": {
    "components": [...]
  }
}
```

## Submit a form

Submit a response to a form.

```
POST /api/forms/{form-id}/submit
Content-Type: application/json
```

**Request body**

```json
{
  "data": {
    "name": "Jane Doe",
    "email": "jane@example.com",
    "message": "Hello!"
  }
}
```

**Response** `201 Created`

```json
{
  "id": "sub_xyz",
  "form_id": "abc123",
  "created_at": "2026-02-27T12:00:00Z"
}
```

**Error responses**

| Status | Meaning |
|--------|---------|
| `404`  | Form not found |
| `422`  | Validation failed — response includes field errors |
| `429`  | Rate limit exceeded — wait and retry |

## Get embed HTML

Returns a standalone HTML page for embedding in an iframe.

```
GET /api/forms/{form-id}/embed
```

Returns `text/html` with the rendered form.
```

**Step 5: Write submissions.md**

```markdown
---
title: Submissions
order: 5
---

# Submissions

Every time someone fills out your form, a submission is recorded.

## Viewing submissions

1. Go to **Forms** from the sidebar
2. Click on the form you want to check
3. Click **Submissions** in the form toolbar

You'll see a list of all submissions with timestamps.

## Submission detail

Click any submission to see the full response data, including all field values and metadata.

## Data handling

- Submissions are stored securely in the GoFormX database
- Each submission records the form schema version at the time of submission
- File uploads (if enabled) are stored separately and linked to the submission
```

**Step 6: Run all tests**

Run: `cd goformx-laravel && ddev exec php artisan test --compact --filter=DocsPageTest`
Expected: All tests pass.

**Step 7: Commit**

```bash
git add goformx-laravel/resources/docs/
git commit -m "feat(docs): add documentation content for all 5 pages"
```

---

### Task 6: Add docs to sitemap and public footer

**Files:**
- Modify: `goformx-laravel/routes/web.php` (sitemap section, ~line 31-37)
- Modify: `goformx-laravel/resources/js/components/PublicFooter.vue`

**Step 1: Add docs to sitemap**

In `routes/web.php`, add to the `$urls` array (after the terms entry):

```php
['loc' => $appUrl.'/docs', 'lastmod' => $lastmod],
```

**Step 2: Add Documentation link to PublicFooter.vue**

Add between the Terms and GitHub links:

```vue
<span class="text-muted-foreground/40">|</span>
<Link
    :href="docsShow().url"
    class="text-sm text-muted-foreground transition-colors hover:text-foreground"
>
    Documentation
</Link>
```

Import `show as docsShow` from `@/routes/docs` at the top.

**Step 3: Build and verify**

Run: `cd goformx-laravel && ddev exec npm run build`
Expected: Build succeeds.

**Step 4: Commit**

```bash
git add goformx-laravel/routes/web.php goformx-laravel/resources/js/components/PublicFooter.vue
git commit -m "feat(docs): add docs to sitemap and public footer"
```

---

### Task 7: Run full test suite and final formatting

**Step 1: Run PHP formatting**

Run: `cd goformx-laravel && ddev exec vendor/bin/pint --dirty --format agent`

**Step 2: Run frontend lint and format**

Run: `cd goformx-laravel && ddev exec npm run lint && ddev exec npm run format`

**Step 3: Run full test suite**

Run: `cd goformx-laravel && ddev exec php artisan test --compact`
Expected: All tests pass.

**Step 4: Build frontend**

Run: `cd goformx-laravel && ddev exec npm run build`
Expected: Build succeeds.

**Step 5: Commit any formatting changes**

```bash
git add -A goformx-laravel/
git commit -m "chore: run formatting and lint fixes"
```

(Skip if no changes.)
