# Documentation Pages Design

**Date**: 2026-02-27
**Status**: Approved

## Summary

Replace the default Laravel documentation link with in-app, public documentation pages at `/docs`. Markdown-driven content rendered through a single Inertia page with a dedicated docs layout.

## Requirements

- **Audience**: End users and developers
- **Access**: Public (no auth)
- **Scope**: 5 essential pages, ship fast
- **Stack**: Markdown files + CommonMark parser + Inertia/Vue

## Architecture

Single controller, single Inertia page, markdown content files.

```
Browser → GET /docs/{slug?}
       → DocsController@show
       → reads resources/docs/{slug}.md
       → parses markdown → HTML
       → Inertia::render('Docs/Show', { content, title, nav })
       → DocsLayout.vue renders sidebar nav + HTML content
```

- **Route**: `GET /docs/{slug?}` — public, no auth. Default slug = `getting-started`
- **Controller**: `DocsController` with a single `show()` method
- **Parser**: `league/commonmark` (already in Laravel's dependency tree)
- **Layout**: `DocsLayout.vue` using `AppHeaderLayout` (public pages header) with left sidebar nav and content area
- **Content**: Markdown files in `resources/docs/` with YAML front matter for title and order

## Pages

| Slug | Title | Audience | Content |
|------|-------|----------|---------|
| `getting-started` | Getting Started | All | What GoFormX is, create account, build first form |
| `form-builder` | Form Builder | End users | Drag-and-drop UI, field types, validation, preview |
| `embedding-forms` | Embedding Forms | Developers | Embed snippet, public URL, styling options |
| `api-reference` | API Reference | Developers | Public endpoints (schema, submit, embed), request/response examples |
| `submissions` | Submissions | All | Viewing submissions, data export |

## Component Design

### DocsLayout.vue

- Left sidebar: nav links from markdown front matter (title + order)
- Right content area: rendered HTML with Tailwind Typography `prose` class
- Mobile: sidebar collapses into a shadcn `Sheet`
- Reuses `AppHeaderLayout` for top navbar

### Styling

- `@tailwindcss/typography` `prose` class for markdown content
- Dark mode via existing CSS variables + `prose-invert`
- Code blocks with monospace font and subtle background
- Sidebar mirrors authenticated sidebar design language (hover states, active indicator)

## Data Flow

1. `resources/docs/{slug}.md` contains YAML front matter (`title`, `order`) and markdown body
2. `DocsController::show($slug)` reads the file, parses front matter + markdown
3. Controller scans `resources/docs/*.md` for sidebar navigation (title, order, slug)
4. Inertia response: `{ content (HTML), title, navigation[] }`
5. `DocsLayout.vue` renders sidebar from `navigation[]`, content inside `prose` container

## Files to Create

- `app/Http/Controllers/DocsController.php`
- `resources/js/pages/Docs/Show.vue`
- `resources/js/layouts/DocsLayout.vue`
- `resources/docs/getting-started.md`
- `resources/docs/form-builder.md`
- `resources/docs/embedding-forms.md`
- `resources/docs/api-reference.md`
- `resources/docs/submissions.md`

## Files to Modify

- `routes/web.php` — add `/docs/{slug?}` route
- `resources/js/components/AppSidebar.vue` — update Documentation link href
- `resources/js/components/AppHeader.vue` — update Documentation link href

## Out of Scope

- Search
- Versioning
- Edit-on-GitHub links
- Breadcrumbs
- Table of contents sidebar
