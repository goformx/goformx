# Form.io Tailwind + shadcn Theme Conversion

**Date:** 2026-02-26
**Status:** Approved
**Scope:** Full conversion of @goformx/formio templates from Bootstrap to Tailwind CSS + shadcn design tokens

## Problem

Form.io ships with Bootstrap 5 styling. GoFormX uses Tailwind CSS v4 + shadcn-vue design tokens everywhere else. This creates:

1. **Visual inconsistency** — forms look like a Bootstrap island in a Tailwind app
2. **Bundle bloat** — shipping bootstrap.min.css + bootstrap-icons.css + formio.full.css just for forms
3. **Theme fragmentation** — dark mode and design token changes don't propagate to forms

## Decision

**Approach A: Convert EJS templates in-place.** Rewrite all 60+ EJS templates in `@goformx/formio` to emit Tailwind utilities + shadcn CSS custom properties. Replace Bootstrap Icons with inline Lucide SVGs. Drop Bootstrap dependencies entirely.

### Alternatives Considered

- **B: CSS override layer** — Keep Bootstrap templates, override with Tailwind `@apply`. Rejected: doesn't remove Bootstrap, fragile across Form.io upgrades, two layers of indirection.
- **C: Runtime class transformer** — Rewrite classes in `transform()`. Rejected: many classes hardcoded in EJS templates, incomplete coverage.

## Architecture

### What Changes

| File | Change |
|------|--------|
| `src/templates/goforms/*/form.ejs` | All 61 template dirs: Bootstrap classes → Tailwind + shadcn tokens |
| `src/templates/goforms/*/html.ejs` | Where present: same conversion |
| `src/templates/goforms/cssClasses.ts` | Map Form.io logical names → Tailwind utilities |
| `src/templates/goforms/iconClass.ts` | Return Lucide inline SVG strings instead of bi-* classes |
| `src/templates/goforms/index.ts` | Updated `transform()` for Tailwind logic |
| `goformx-laravel/resources/css/app.css` | Remove Bootstrap/formio.full.css imports |
| `goformx-laravel/package.json` | Remove `bootstrap`, `bootstrap-icons` deps |

### What Stays the Same

- Package structure, build process (tsc + gulp + webpack), exports
- Template context API (`ctx.component`, `ctx.input`, `ctx.instance`, etc.)
- Accessibility attributes (aria-*, roles, sr-only)
- Registration mechanism (`Formio.use(goforms)`)
- Form.io JS behavior (validation, submission, drag-and-drop)

## CSS Class Mapping

### Buttons

| Bootstrap | Tailwind + shadcn |
|-----------|------------------|
| `btn btn-primary` | `inline-flex items-center justify-center rounded-md bg-primary text-primary-foreground px-4 py-2 text-sm font-medium shadow-sm hover:bg-primary/90` |
| `btn btn-danger` | `inline-flex items-center justify-center rounded-md bg-destructive text-destructive-foreground px-4 py-2 text-sm font-medium shadow-sm` |
| `btn btn-default` | `inline-flex items-center justify-center rounded-md border border-input bg-background px-4 py-2 text-sm font-medium shadow-sm` |

### Alerts

| Bootstrap | Tailwind + shadcn |
|-----------|------------------|
| `alert alert-danger` | `rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive` |
| `alert alert-success` | `rounded-md border border-green-500/50 bg-green-500/10 px-4 py-3 text-sm text-green-700 dark:text-green-400` |
| `alert alert-warning` | `rounded-md border border-yellow-500/50 bg-yellow-500/10 px-4 py-3 text-sm text-yellow-700 dark:text-yellow-400` |

### Form Elements

| Bootstrap | Tailwind + shadcn |
|-----------|------------------|
| `form-control` | `flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring` |
| `form-check` | `flex items-center gap-2` |
| `form-check-input` | `h-4 w-4 rounded border border-primary text-primary focus:ring-2 focus:ring-ring` |
| `col-form-label` | `text-sm font-medium leading-none` |
| `form-text text-muted` | `text-sm text-muted-foreground` |

### Cards (Panels)

| Bootstrap | Tailwind + shadcn |
|-----------|------------------|
| `card` | `rounded-lg border bg-card text-card-foreground shadow-sm` |
| `card-header` | `flex items-center justify-between border-b px-4 py-3` |
| `card-body` | `p-4` |
| `card-title` | `text-sm font-semibold` |

### Layout

| Bootstrap | Tailwind |
|-----------|----------|
| `row` | `grid grid-cols-12 gap-4` |
| `col-sm-6` | `col-span-6` |
| `col-md-4` | `col-span-4` |
| `col-xs-*` | `col-span-*` (no breakpoint) |

### Utilities

| Bootstrap | Tailwind |
|-----------|----------|
| `mb-2` | `mb-2` |
| `text-muted` | `text-muted-foreground` |
| `float-end` | `ml-auto` |
| `visually-hidden` | `sr-only` |
| `d-grid` | `grid` |

## Icon System

Replace Bootstrap Icons (font-based, `bi bi-*`) with inline Lucide SVGs.

Since EJS templates compile to strings (not Vue components), we create an `iconSvg.ts` utility that returns raw SVG strings for the ~20 icons Form.io uses.

### Icon Mapping

| Bootstrap Icons | Lucide |
|----------------|--------|
| `bi-gear` | Settings |
| `bi-trash` | Trash2 |
| `bi-plus-lg` | Plus |
| `bi-question-circle` | HelpCircle |
| `bi-arrows-move` | GripVertical |
| `bi-pencil` | Pencil |
| `bi-back` (copy) | Copy |
| `bi-clipboard` (paste) | ClipboardPaste |
| `bi-plus-square` | ChevronDown |
| `bi-dash-square` | ChevronUp |
| `bi-spinner-border` | Loader2 (animated) |

The `icon/form.ejs` template changes from `<i class="{{iconClass}}"></i>` to rendering the inline SVG directly.

## formio.full.css Handling

`@formio/js/dist/formio.full.css` bundles Bootstrap + Form.io structural styles. Strategy:

1. **Remove** the `formio.full.css` import from `app.css`
2. **Create** a minimal `formio-structural.css` in `@goformx/formio` containing only JS-driven structural styles:
   - `.formio-dialog-overlay` — modal overlays
   - `.formio-component-hidden` — JS visibility toggles
   - `.drag-container`, `.drag-copy` — drag-and-drop positioning
   - `.formio-disabled-input` — disabled state
   - `.choices__*` — Choices.js select widget (if used)
3. **Import** this structural CSS in `app.css` instead

## Template Conversion Tiers

### Tier 1 — Form fields (user-facing, highest visual impact)

`input`, `button`, `checkbox`, `radio`, `select`, `selectOption`, `field`, `label`, `errorsList`, `alert`, `message`

### Tier 2 — Layout (structural)

`panel`, `columns`, `fieldset`, `container`, `well`, `tab`, `table`, `webform`, `components`, `tableComponents`

### Tier 3 — Builder chrome (admin-facing)

`builder`, `builderComponent`, `builderComponents`, `builderEditForm`, `builderPlaceholder`, `builderSidebar`, `builderSidebarGroup`, `builderWizard`

### Tier 4 — Modals & specialized

`modaldialog`, `modaledit`, `modalPreview`, `componentModal`, `dialog`, `signature`, `file`, `datagrid`, `editgrid`, `editgridTable`, `day`, `address`, `survey`, `tree`, `pdf`, `pdfBuilder`, `pdfBuilderUpload`, `wizard`, `wizardHeader*`, `wizardNav`, `html`, `icon`, `loader`, `loading`, `map`, `multiValueRow`, `multiValueTable`, `multipleMasksInput`, `resourceAdd`

## Testing Strategy

1. **Unit tests**: Existing Mocha tests in wrapper validate template registration (stay valid)
2. **Visual testing**: Render each component type in GoFormX and verify appearance
3. **Functional testing**: Builder drag-and-drop, field editing, form preview, public fill+submit
4. **Dark mode**: Verify all templates work in light/dark via shadcn tokens
5. **Responsive**: Verify grid columns and builder layout at mobile/tablet/desktop breakpoints

## Dependencies Removed

- `bootstrap` (npm)
- `bootstrap-icons` (npm)
- `@formio/js/dist/formio.full.css` import (replaced by structural-only CSS)

## Dependencies Added

- `lucide-static` or inline SVG strings (no runtime dependency needed)
