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

### 3. CSS Specificity Conflicts

Bootstrap CSS was imported in a lower-specificity layer:
```css
@import 'bootstrap/dist/css/bootstrap.min.css' layer(formio);
```

But Tailwind's base styles and resets had higher specificity, causing Bootstrap's display properties to be overridden.

## Solution

### Step 1: Link Local Package

```bash
cd /home/jones/dev/goformx/goformx-formio
npm link

cd /home/jones/dev/goformx/goformx-laravel
npm link @goformx/formio
```

This creates a symlink so changes to local templates are reflected immediately.

### Step 2: Add CSS Overrides

In `goformx-laravel/resources/css/app.css`, add explicit styles for the component buttons:

```css
/* Force visibility of ALL form builder sidebar content */
#form-schema-builder .formcomponent,
#form-schema-builder span[ref="sidebar-component"] {
    display: inline-flex !important;
    visibility: visible !important;
    opacity: 1 !important;
    align-items: center;
    gap: 0.25rem;
    padding: 0.375rem 0.75rem;
    font-size: 0.875rem;
    border-radius: 0.375rem;
    border: 1px solid hsl(var(--primary));
    color: hsl(var(--primary));
    background: transparent;
    cursor: grab;
    transition: background-color 0.15s, color 0.15s;
}

#form-schema-builder .formcomponent:hover,
#form-schema-builder span[ref="sidebar-component"]:hover {
    background: hsl(var(--primary));
    color: hsl(var(--primary-foreground));
}

/* Expanded group content */
#form-schema-builder div[ref="sidebar-group"]:not(.hidden) {
    display: block !important;
}

/* Collapsed group stays hidden */
#form-schema-builder div[ref="sidebar-group"].hidden,
#form-schema-builder .hidden {
    display: none !important;
}
```

### Step 3: Add Tailwind Source Scanning (Optional)

```css
@source '../../node_modules/@goformx/formio/lib/**/*.js';
```

This tells Tailwind v4 to scan the goformx/formio templates for utility classes, though the CSS overrides above are more reliable.

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

5. **Bootstrap CSS in layers** can still cause conflicts. The `layer(formio)` approach gives lower specificity, but explicit `!important` overrides may still be needed.

## Files Modified

- `goformx-laravel/resources/css/app.css` - CSS overrides and `@source` directive
- `goformx-laravel/resources/js/pages/Forms/Edit.vue` - Collapsible header, layout adjustments
- `goformx-laravel/resources/js/components/form-builder/BuilderLayout.vue` - Height calculations

## Future Improvements

1. **Publish updated @goformx/formio** - Release a new npm version with Tailwind templates so `npm link` isn't required in production
2. **Remove Bootstrap imports** - Once all Form.io templates use Tailwind, remove Bootstrap CSS entirely
3. **Add Tailwind safelist** - If `@source` scanning remains unreliable, add a safelist for goforms template classes
4. **Convert remaining templates** - Ensure all Form.io templates (not just builder sidebar) use Tailwind classes
