# Form.io Builder Sidebar Component Visibility Fix

**Date**: 2026-02-27
**Problem**: Form builder sidebar showing headers (Basic, Layout) but no draggable field components
**Affected**: goformx-laravel form builder page (`/forms/:id/edit`)

## Symptoms

- Form builder loaded but the sidebar only showed "Basic" and "Layout" section headers
- The draggable field buttons (Text Field, Email, Number, etc.) were invisible
- Large empty white space between section headers
- No console errors related to templates

## Root Causes

### 1. Package Version Mismatch

The `@goformx/formio` npm package (v0.1.4) contained **Bootstrap-based templates**, while the local source code in `goformx-formio/src/` had been updated to **Tailwind-based templates**.

**Evidence**: Comparing compiled template output:
```bash
# npm package (Bootstrap classes)
cat node_modules/@goformx/formio/lib/mjs/templates/goforms/builderSidebarGroup/form.ejs.js
# Shows: class="card form-builder-panel accordion-item"
# Shows: class="btn btn-outline-primary btn-sm formcomponent"

# Local source (Tailwind classes)
cat goformx-formio/lib/mjs/templates/goforms/builderSidebarGroup/form.ejs.js
# Shows: class="border-b border-border form-builder-panel"
# Shows: class="inline-flex items-center ... formcomponent"
```

### 2. Tailwind CSS Class Generation

Even after linking the local package, the Tailwind utility classes used in the goforms templates weren't being generated. Tailwind v4 uses `@source` directives for content scanning:

```css
@source '../../node_modules/@goformx/formio/lib/**/*.js';
```

However, this wasn't sufficient because:
- The package is symlinked (`node_modules/@goformx/formio -> ../../../goformx-formio`)
- Tailwind may not follow symlinks correctly
- The classes are embedded in JavaScript string templates, not in actual markup files

### 3. CSS Cascade Layer Conflicts

Bootstrap CSS is in `layer(formio)`, making it lower-priority than Tailwind's unlayered styles by design. However, this also means Bootstrap's `!important` rules (which Form.io depends on for display toggling) are beaten by unlayered `!important` — creating a conflict where Tailwind resets override Bootstrap's intended display states.

## Solution

The fix uses a two-pronged approach: CSS for non-color structural properties, and JavaScript CSSOM for color properties that conflict with Bootstrap's layered `!important` rules.

### Step 1: Link Local Package

```bash
cd /home/jones/dev/goformx/goformx-formio
npm link

cd /home/jones/dev/goformx/goformx-laravel
npm link @goformx/formio
```

This creates a symlink so changes to local templates are reflected immediately.

### Step 2: CSS Overrides for Non-Color Properties

In `goformx-laravel/resources/css/formio-overrides.css` (imported into `layer(formio)`), add structural overrides that don't conflict with Bootstrap's `!important` color rules:

```css
/* Drop zone — non-color properties work via CSS */
#form-schema-builder .drag-and-drop-alert {
    border-radius: 0.75rem !important;
    padding: 3rem 2rem !important;
    text-align: center !important;
    font-size: 0.875rem !important;
}

/* Builder submit button — non-color properties work via CSS */
#form-schema-builder .formio-builder-form .btn-primary {
    display: inline-flex !important;
    padding: 0.5rem 1rem !important;
    font-size: 0.875rem !important;
    border-radius: 0.375rem !important;
}
```

### Step 3: JavaScript CSSOM for Color Properties

In `goformx-laravel/resources/js/composables/useFormBuilder.ts`, use `element.style.setProperty(prop, value, 'important')` to apply color properties. Inline `!important` wins the cascade over stylesheet `!important` regardless of layers:

```typescript
// Sidebar buttons — color properties that conflict with Bootstrap
btn.style.setProperty('border', '1px solid var(--border)', 'important');
btn.style.setProperty('color', 'var(--foreground)', 'important');
btn.style.setProperty('background-color', 'var(--background)', 'important');
```

A `MutationObserver` re-applies these styles whenever Form.io mutates the DOM (e.g., toggling sidebar groups).

### Step 4: Add Tailwind Source Scanning (Optional)

```css
@source '../../node_modules/@goformx/formio/lib/**/*.js';
```

This tells Tailwind v4 to scan the goformx/formio templates for utility classes, though the CSS/JS overrides above are more reliable.

## Debugging Techniques Used

### 1. Template Registration Verification

Added debug logging to verify goforms templates were being loaded:

```typescript
Logger.debug('GoForms plugin:', goforms);
Formio.use(goforms);
const FormioExt = Formio as { Templates?: { framework?: string; current?: object } };
Logger.debug('Formio.Templates.framework:', FormioExt.Templates?.framework);
// Output: "goforms" - templates ARE registered
```

### 2. Debug CSS Backgrounds

Used colored backgrounds to verify elements exist in DOM:

```css
#form-schema-builder div[ref="sidebar-container"] {
    background: rgba(255, 0, 0, 0.1);
    min-height: 50px;
}
```

This revealed the elements existed but had no visible content.

### 3. Console Message Analysis

```bash
grep -i "goforms\|template\|framework" console-output.txt
```

Confirmed templates were registered but helped identify the CSS visibility issue.

## Key Learnings

1. **Form.io uses `ref` attributes** for template element references (e.g., `ref="sidebar-component"`). These are valid CSS attribute selectors.

2. **Tailwind v4 `@source` directive** may not follow symlinks reliably. Manual CSS overrides provide a more stable solution during development.

3. **The goforms templates use Tailwind CSS variable classes** like `text-foreground`, `border-primary`, `bg-accent`. These require:
   - The CSS variables to be defined (via `@theme` in app.css)
   - The utility classes to be generated by Tailwind

4. **`Formio.use(plugin)` works by**:
   - Reading `plugin.framework` to set the active framework name
   - Reading `plugin.templates[frameworkName]` to get template definitions
   - Setting `Formio.Templates.current` to the active templates

5. **Bootstrap CSS in layers** can still cause conflicts. The `layer(formio)` approach gives lower specificity, but inline `!important` via JavaScript CSSOM is needed for color properties that Bootstrap also sets with `!important`.

## Files Modified

- `goformx-laravel/resources/css/app.css` — CSS layout rules and `@source` directive
- `goformx-laravel/resources/css/formio-overrides.css` — CSS structural overrides in formio layer (dialog styling, non-color properties)
- `goformx-laravel/resources/js/composables/useFormBuilder.ts` — JavaScript CSSOM workaround for color properties, MutationObserver
- `goformx-laravel/resources/js/pages/Forms/Edit.vue` — Collapsible header, layout adjustments
- `goformx-laravel/resources/js/components/form-builder/BuilderLayout.vue` — Height calculations

## Future Improvements

1. **Publish updated @goformx/formio** — Release a new npm version with Tailwind templates so `npm link` isn't required in production
2. **Remove Bootstrap imports** — Once all Form.io templates use Tailwind, remove Bootstrap CSS entirely
3. **Add Tailwind safelist** — If `@source` scanning remains unreliable, add a safelist for goforms template classes
4. **Convert remaining templates** — Ensure all Form.io templates (not just builder sidebar) use Tailwind classes
