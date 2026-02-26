# Form.io Tailwind + shadcn Theme Conversion — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Convert all 60+ @goformx/formio EJS templates from Bootstrap 5 to Tailwind CSS + shadcn design tokens, replace Bootstrap Icons with inline Lucide SVGs, and remove Bootstrap dependencies entirely.

**Architecture:** Rewrite the template output layer of an existing Form.io module. The `@goformx/formio` package already provides 67 custom templates registered via `Formio.use()`. We change what those templates emit (Tailwind classes instead of Bootstrap classes), update the CSS class mapping and icon system, then remove Bootstrap from the Laravel app's CSS imports and npm dependencies.

**Tech Stack:** EJS templates, TypeScript, Tailwind CSS v4, shadcn-vue CSS custom properties, Lucide icons (inline SVG strings), Form.io JS v5.1.1

**Reference Design:** `docs/plans/2026-02-26-formio-tailwind-theming-design.md`

---

## Class Reference Table

Use this table everywhere a Bootstrap class appears in a template. Every template conversion in this plan follows these mappings.

```
BOOTSTRAP CLASS                          TAILWIND + SHADCN REPLACEMENT
───────────────────────────────────────────────────────────────────────

// BUTTONS
btn btn-primary                        → inline-flex items-center justify-center gap-2 rounded-md bg-primary text-primary-foreground px-4 py-2 text-sm font-medium shadow-sm hover:bg-primary/90 transition-colors
btn btn-secondary                      → inline-flex items-center justify-center gap-2 rounded-md bg-secondary text-secondary-foreground px-4 py-2 text-sm font-medium shadow-sm hover:bg-secondary/80 transition-colors
btn btn-danger                         → inline-flex items-center justify-center gap-2 rounded-md bg-destructive text-destructive-foreground px-4 py-2 text-sm font-medium shadow-sm hover:bg-destructive/90 transition-colors
btn btn-success                        → inline-flex items-center justify-center gap-2 rounded-md bg-green-600 text-white px-4 py-2 text-sm font-medium shadow-sm hover:bg-green-700 transition-colors
btn btn-warning                        → inline-flex items-center justify-center gap-2 rounded-md bg-yellow-500 text-white px-4 py-2 text-sm font-medium shadow-sm hover:bg-yellow-600 transition-colors
btn btn-default                        → inline-flex items-center justify-center gap-2 rounded-md border border-input bg-background text-foreground px-4 py-2 text-sm font-medium shadow-sm hover:bg-accent transition-colors
btn btn-light                          → inline-flex items-center justify-center gap-2 rounded-md border border-input bg-background text-foreground px-4 py-2 text-sm font-medium shadow-sm hover:bg-accent transition-colors
btn btn-outline-primary                → inline-flex items-center justify-center gap-2 rounded-md border border-primary text-primary bg-transparent px-4 py-2 text-sm font-medium hover:bg-primary hover:text-primary-foreground transition-colors

// BUTTON SIZES
btn-xxs                                → px-1.5 py-0.5 text-xs (replace btn-xxs in the compound class)
btn-xs                                 → px-2 py-1 text-xs
btn-sm                                 → px-3 py-1.5 text-sm
btn-md                                 → px-4 py-2 text-sm
btn-lg                                 → px-6 py-3 text-base

// ALERTS
alert alert-danger                     → rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive
alert alert-success                    → rounded-md border border-green-500/50 bg-green-500/10 px-4 py-3 text-sm text-green-700 dark:text-green-400
alert alert-warning                    → rounded-md border border-yellow-500/50 bg-yellow-500/10 px-4 py-3 text-sm text-yellow-700 dark:text-yellow-400
alert alert-info                       → rounded-md border border-blue-500/50 bg-blue-500/10 px-4 py-3 text-sm text-blue-700 dark:text-blue-400

// FORM ELEMENTS
form-control                           → flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring
form-check                             → flex items-start gap-2
form-check-input                       → h-4 w-4 shrink-0 rounded border border-primary text-primary accent-primary focus:ring-2 focus:ring-ring
form-check-label                       → text-sm font-normal leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70
form-radio                             → space-y-2
form-check-inline                      → inline-flex items-center gap-2
col-form-label                         → text-sm font-medium leading-none text-foreground
form-text text-muted                   → text-sm text-muted-foreground mt-1.5
form-text                              → text-sm text-muted-foreground
form-floating                          → relative
input-group                            → flex items-stretch
input-group-text                       → inline-flex items-center rounded-md border border-input bg-muted px-3 text-sm text-muted-foreground

// CARDS (PANELS)
card                                   → rounded-lg border border-border bg-card text-card-foreground shadow-sm
card-header                            → flex items-center justify-between border-b border-border px-4 py-3
card-body                              → p-4
card-title                             → text-sm font-semibold
card-vertical                          → flex flex-row (for vertical tabs)

// LAYOUT GRID
row                                    → grid grid-cols-12 gap-4
col-xs-{n} / col-sm-{n} / col-md-{n}  → col-span-{n}  (just map the number)
col col-xs-3                           → col-span-3
col col-xs-4                           → col-span-4
col col-xs-5                           → col-span-5
col col-sm-2                           → col-span-2 (or sm:col-span-2 if responsive needed)
col col-sm-3                           → col-span-3
col col-sm-6                           → col-span-6
col col-sm-9                           → col-span-9
col col-sm-10                          → col-span-10
col col-md-2                           → col-span-2
col col-md-10                          → col-span-10
offset-{size}-{n}                      → col-start-{n+1} (CSS grid offset)

// TABLES
table                                  → w-full text-sm
table-bordered                         → border border-border [&_th]:border [&_th]:border-border [&_td]:border [&_td]:border-border
table-striped                          → [&_tbody_tr:nth-child(odd)]:bg-muted/50
table-hover                            → [&_tbody_tr]:hover:bg-muted/50
table-sm                               → [&_th]:px-2 [&_th]:py-1 [&_td]:px-2 [&_td]:py-1

// LIST GROUPS
list-group                             → divide-y divide-border rounded-lg border border-border
list-group-item                        → px-4 py-3
list-group-header                      → font-semibold bg-muted/50

// PAGINATION / WIZARD TABS
pagination                             → flex gap-1
page-item                              → (no class needed, style on child)
page-item active                       → (set child button/link to active style)
page-link                              → inline-flex items-center justify-center rounded-md px-3 py-1.5 text-sm font-medium border border-input hover:bg-accent transition-colors

// NAV / TABS
nav nav-tabs                           → flex border-b border-border
nav-item                               → (no class needed)
nav-link                               → inline-flex items-center px-4 py-2 text-sm font-medium text-muted-foreground hover:text-foreground border-b-2 border-transparent transition-colors
nav-link active                        → border-b-2 border-primary text-foreground

// ACCORDION
accordion                              → divide-y divide-border rounded-lg border border-border
accordion-item                         → (use divide-y from parent)
accordion-header                       → (no class needed)
accordion-collapse collapse            → overflow-hidden transition-all
accordion-collapse show                → (remove hidden)

// BREADCRUMBS (wizard pages)
breadcrumb                             → flex flex-wrap items-center gap-2
badge bg-primary                       → inline-flex items-center rounded-full bg-primary text-primary-foreground px-2.5 py-0.5 text-xs font-semibold
badge bg-info                          → inline-flex items-center rounded-full bg-blue-500/10 text-blue-700 dark:text-blue-400 px-2.5 py-0.5 text-xs font-semibold
badge bg-success                       → inline-flex items-center rounded-full bg-green-500/10 text-green-700 dark:text-green-400 px-2.5 py-0.5 text-xs font-semibold

// PROGRESS BARS
progress                               → h-2 w-full overflow-hidden rounded-full bg-muted
progress-bar                           → h-full bg-primary transition-all

// UTILITIES
mb-0                                   → mb-0
mb-2                                   → mb-2
mb-3                                   → mb-3
mt-0                                   → mt-0
me-2                                   → mr-2
ms-2                                   → ml-2
ps-0                                   → pl-0
p-0                                    → p-0
p-2                                    → p-2
d-grid                                 → grid
gap-1                                  → gap-1
w-100                                  → w-full
text-muted                             → text-muted-foreground
text-end                               → text-right
text-center                            → text-center
text-light                             → text-primary-foreground
text-danger                            → text-destructive
float-end                              → float-right
float-right                            → float-right
visually-hidden                        → sr-only
lead                                   → text-lg font-medium
help-block                             → text-sm text-destructive
has-error                              → text-destructive
has-message                            → (keep or drop — used as JS hook)
invalid-feedback                       → text-sm text-destructive
no-drag                                → (keep — JS hook)
no-drop                                → (keep — JS hook)
drag-copy                              → (keep — JS hook)
bg-light                               → bg-muted
bg-{theme}                             → bg-primary (map dynamically)
control-label                          → text-sm font-medium
field-required                         → after:content-['*'] after:ml-0.5 after:text-destructive
```

---

### Task 1: Create Lucide Icon SVG Utility

**Files:**
- Create: `goformx-formio/src/templates/goforms/iconSvg.ts`
- Modify: `goformx-formio/src/templates/goforms/iconClass.ts`

**Context:** Form.io renders icons via `ctx.iconClass('name')` which returns a CSS class string like `bi bi-trash`. The `icon/form.ejs` template renders `<i class="{{ctx.className}}">`. We need to change the icon system to return inline SVG markup instead of CSS class names. However, `iconClass` is called from many templates (not just the icon template), so we need both an SVG lookup and a backward-compatible class string for templates that embed icon classes directly in `<i>` elements.

**Strategy:** Create an `iconSvg.ts` that maps icon names to inline SVG strings. Then modify `iconClass.ts` to return Lucide-compatible CSS classes. The `icon/form.ejs` template will be updated in Task 2 to use SVG directly.

**Step 1: Create iconSvg.ts with inline Lucide SVGs**

The SVG strings are from Lucide's icon set. Each is 16x16 with `currentColor` for theming. Only include icons actually used by Form.io templates (check all `ctx.iconClass('...')` calls across templates).

Icons used across all templates:
- `remove` / `trash` — Trash2
- `plus` — Plus
- `question-sign` — CircleHelp
- `move` — GripVertical
- `copy` / `back` — Copy
- `save` / `clipboard` — ClipboardPaste
- `cog` / `gear` — Settings
- `edit` / `pencil` — Pencil
- `wrench` — Wrench
- `plus-square-o` — ChevronDown (expand)
- `minus-square-o` — ChevronUp (collapse)
- `remove-circle` / `x-circle` — XCircle
- `new-window` — ExternalLink
- `refresh` / `arrow-repeat` — RotateCcw
- `cloud-upload` — CloudUpload
- `folder-open` — FolderOpen
- `camera` — Camera
- `video` — Video
- `microphone` — Mic
- `zoom-in` / `search-plus` — ZoomIn
- `zoom-out` / `search-minus` — ZoomOut
- `ban` — Ban
- `book` — BookOpen
- `asterisk` — Asterisk
- `check-circle` — CheckCircle
- `times-circle` — XCircle
- `minus` — Minus
- `circle` — Circle
- `list` / `bars` — List
- `arrow-counterclockwise` / `undo` — Undo2
- `arrow-clockwise` / `repeat` — Redo2

```typescript
// src/templates/goforms/iconSvg.ts
const svgAttrs = 'xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"';

const icons: Record<string, string> = {
  trash: `<svg ${svgAttrs}><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/><line x1="10" x2="10" y1="11" y2="17"/><line x1="14" x2="14" y1="11" y2="17"/></svg>`,
  plus: `<svg ${svgAttrs}><path d="M5 12h14"/><path d="M12 5v14"/></svg>`,
  'circle-help': `<svg ${svgAttrs}><circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/></svg>`,
  'grip-vertical': `<svg ${svgAttrs}><circle cx="9" cy="12" r="1"/><circle cx="9" cy="5" r="1"/><circle cx="9" cy="19" r="1"/><circle cx="15" cy="12" r="1"/><circle cx="15" cy="5" r="1"/><circle cx="15" cy="19" r="1"/></svg>`,
  copy: `<svg ${svgAttrs}><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>`,
  'clipboard-paste': `<svg ${svgAttrs}><path d="M15 2H9a1 1 0 0 0-1 1v2c0 .6.4 1 1 1h6c.6 0 1-.4 1-1V3c0-.6-.4-1-1-1Z"/><path d="M8 4H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2M16 4h2a2 2 0 0 1 2 2v2M11 14h10"/><path d="m17 10 4 4-4 4"/></svg>`,
  settings: `<svg ${svgAttrs}><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>`,
  pencil: `<svg ${svgAttrs}><path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/></svg>`,
  wrench: `<svg ${svgAttrs}><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg>`,
  'chevron-down': `<svg ${svgAttrs}><path d="m6 9 6 6 6-6"/></svg>`,
  'chevron-up': `<svg ${svgAttrs}><path d="m18 15-6-6-6 6"/></svg>`,
  'x-circle': `<svg ${svgAttrs}><circle cx="12" cy="12" r="10"/><path d="m15 9-6 6"/><path d="m9 9 6 6"/></svg>`,
  'external-link': `<svg ${svgAttrs}><path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/></svg>`,
  'rotate-ccw': `<svg ${svgAttrs}><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/></svg>`,
  'cloud-upload': `<svg ${svgAttrs}><path d="M12 13v8"/><path d="M4 14.899A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.5 8.242"/><path d="m8 17 4-4 4 4"/></svg>`,
  'folder-open': `<svg ${svgAttrs}><path d="m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2"/></svg>`,
  camera: `<svg ${svgAttrs}><path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z"/><circle cx="12" cy="13" r="3"/></svg>`,
  video: `<svg ${svgAttrs}><path d="m16 13 5.223 3.482a.5.5 0 0 0 .777-.416V7.87a.5.5 0 0 0-.752-.432L16 10.5"/><rect x="2" y="6" width="14" height="12" rx="2"/></svg>`,
  mic: `<svg ${svgAttrs}><path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"/><path d="M19 10v2a7 7 0 0 1-14 0v-2"/><line x1="12" x2="12" y1="19" y2="22"/></svg>`,
  'zoom-in': `<svg ${svgAttrs}><circle cx="11" cy="11" r="8"/><line x1="21" x2="16.65" y1="21" y2="16.65"/><line x1="11" x2="11" y1="8" y2="14"/><line x1="8" x2="14" y1="11" y2="11"/></svg>`,
  'zoom-out': `<svg ${svgAttrs}><circle cx="11" cy="11" r="8"/><line x1="21" x2="16.65" y1="21" y2="16.65"/><line x1="8" x2="14" y1="11" y2="11"/></svg>`,
  ban: `<svg ${svgAttrs}><circle cx="12" cy="12" r="10"/><path d="m4.9 4.9 14.2 14.2"/></svg>`,
  'book-open': `<svg ${svgAttrs}><path d="M12 7v14"/><path d="M3 18a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h5a4 4 0 0 1 4 4 4 4 0 0 1 4-4h5a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1h-6a3 3 0 0 0-3 3 3 3 0 0 0-3-3z"/></svg>`,
  asterisk: `<svg ${svgAttrs}><path d="M12 2v20"/><path d="m4.93 4.93 14.14 14.14"/><path d="m19.07 4.93-14.14 14.14"/></svg>`,
  'check-circle': `<svg ${svgAttrs}><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><path d="m9 11 3 3L22 4"/></svg>`,
  x: `<svg ${svgAttrs}><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>`,
  minus: `<svg ${svgAttrs}><path d="M5 12h14"/></svg>`,
  circle: `<svg ${svgAttrs}><circle cx="12" cy="12" r="10"/></svg>`,
  list: `<svg ${svgAttrs}><line x1="8" x2="21" y1="6" y2="6"/><line x1="8" x2="21" y1="12" y2="12"/><line x1="8" x2="21" y1="18" y2="18"/><line x1="3" x2="3.01" y1="6" y2="6"/><line x1="3" x2="3.01" y1="12" y2="12"/><line x1="3" x2="3.01" y1="18" y2="18"/></svg>`,
  'undo-2': `<svg ${svgAttrs}><path d="M9 14 4 9l5-5"/><path d="M4 9h10.5a5.5 5.5 0 0 1 5.5 5.5a5.5 5.5 0 0 1-5.5 5.5H11"/></svg>`,
  'redo-2': `<svg ${svgAttrs}><path d="m15 14 5-5-5-5"/><path d="M20 9H9.5A5.5 5.5 0 0 0 4 14.5 5.5 5.5 0 0 0 9.5 20H13"/></svg>`,
  'loader-2': `<svg ${svgAttrs} class="animate-spin"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>`,
  'hand-metal': `<svg ${svgAttrs}><path d="M18 12.5V10a2 2 0 0 0-2-2a2 2 0 0 0-2 2v1.4"/><path d="M14 11V9a2 2 0 1 0-4 0v2"/><path d="M10 10.5V5a2 2 0 1 0-4 0v9"/><path d="m7 15-1.76-1.76a2 2 0 0 0-2.83 2.82l3.6 3.6C7.5 21.14 9.2 22 12 22h2a8 8 0 0 0 8-8V7a2 2 0 1 0-4 0v5"/></svg>`,
};

// Map Form.io icon names to our Lucide icon keys
const nameMap: Record<string, string> = {
  remove: 'trash',
  cog: 'settings',
  gear: 'settings',
  copy: 'copy',
  back: 'copy',
  save: 'clipboard-paste',
  clipboard: 'clipboard-paste',
  edit: 'pencil',
  move: 'grip-vertical',
  arrows: 'grip-vertical',
  'arrows-move': 'grip-vertical',
  'question-sign': 'circle-help',
  'question-circle': 'circle-help',
  'plus-square-o': 'chevron-down',
  'minus-square-o': 'chevron-up',
  'plus-square': 'chevron-down',
  'dash-square': 'chevron-up',
  plus: 'plus',
  'plus-lg': 'plus',
  'remove-circle': 'x-circle',
  'x-circle': 'x-circle',
  'new-window': 'external-link',
  'window-plus': 'external-link',
  refresh: 'rotate-ccw',
  'arrow-repeat': 'rotate-ccw',
  'arrow-counterclockwise': 'undo-2',
  undo: 'undo-2',
  repeat: 'redo-2',
  'arrow-clockwise': 'redo-2',
  'cloud-upload': 'cloud-upload',
  'folder-open': 'folder-open',
  'folder2-open': 'folder-open',
  camera: 'camera',
  'camera-video': 'video',
  video: 'video',
  microphone: 'mic',
  mic: 'mic',
  'zoom-in': 'zoom-in',
  'search-plus': 'zoom-in',
  'zoom-out': 'zoom-out',
  'search-minus': 'zoom-out',
  ban: 'ban',
  book: 'book-open',
  asterisk: 'asterisk',
  'check-circle': 'check-circle',
  'check-circle-fill': 'check-circle',
  'times-circle': 'x-circle',
  'x-circle-fill': 'x-circle',
  pencil: 'pencil',
  'pencil-fill': 'pencil',
  minus: 'minus',
  dash: 'minus',
  circle: 'circle',
  'circle-fill': 'circle',
  'hand-paper-o': 'hand-metal',
  'hand-index': 'hand-metal',
  wrench: 'wrench',
  x: 'x',
  list: 'list',
  bars: 'list',
  trash: 'trash',
};

export function getIconSvg(name: string): string {
  const mappedName = nameMap[name] || name;
  return icons[mappedName] || `<span class="text-muted-foreground">[${name}]</span>`;
}

export function getSpinnerSvg(): string {
  return icons['loader-2'];
}

export default { getIconSvg, getSpinnerSvg, icons, nameMap };
```

**Step 2: Update iconClass.ts to use Lucide CSS classes (backward compat)**

Some templates use `ctx.iconClass('name')` as a class on `<i>` elements. We keep this function but have it return a utility class that we can style. Since we're moving to inline SVGs, this function now returns a data attribute class we use as a hook.

```typescript
// src/templates/goforms/iconClass.ts
type iconset = 'lucide' | 'bi' | 'fa';

export default (_iconset: iconset, name: string, spinning: boolean): string => {
  if (spinning) {
    return 'formio-icon formio-icon-spinner animate-spin';
  }
  return `formio-icon formio-icon-${name}`;
};
```

**Step 3: Run existing tests**

Run: `cd goformx-formio && npm test`
Expected: All existing Mocha tests pass (they test template registration, not CSS classes)

**Step 4: Commit**

```bash
cd goformx-formio
git add src/templates/goforms/iconSvg.ts src/templates/goforms/iconClass.ts
git commit -m "feat: add Lucide SVG icon utility, update iconClass for Tailwind"
```

---

### Task 2: Update CSS Classes Map and Transform Function

**Files:**
- Modify: `goformx-formio/src/templates/goforms/cssClasses.ts`
- Modify: `goformx-formio/src/templates/goforms/index.ts`

**Step 1: Rewrite cssClasses.ts**

```typescript
// src/templates/goforms/cssClasses.ts
export default {
  'border-default': '',
  'formio-tab-panel-active': 'active',
  'formio-tab-link-active': 'border-b-2 border-primary text-foreground',
  'formio-tab-link-container-active': 'active',
  'formio-form-error': 'formio-error-wrapper has-message',
  'formio-form-alert': 'rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive',
  'formio-label-error': '',
  'formio-input-error': '',
  'formio-alert-danger': 'rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive',
  'formio-alert-success': 'rounded-md border border-green-500/50 bg-green-500/10 px-4 py-3 text-sm text-green-700 dark:text-green-400',
  'formio-alert-warning': 'rounded-md border border-yellow-500/50 bg-yellow-500/10 px-4 py-3 text-sm text-yellow-700 dark:text-yellow-400',
  'formio-modal-cancel-button': 'inline-flex items-center justify-center gap-2 rounded-md bg-destructive text-destructive-foreground px-4 py-2 text-sm font-medium shadow-sm hover:bg-destructive/90 transition-colors formio-dialog-button',
  'formio-modal-confirm-button': 'inline-flex items-center justify-center gap-2 rounded-md bg-primary text-primary-foreground px-4 py-2 text-sm font-medium shadow-sm hover:bg-primary/90 transition-colors formio-dialog-button',
  'form-group': 'formio-form-group',
};
```

**Step 2: Update transform() in index.ts**

```typescript
// In the export default object, replace the transform function:
transform(type: string, text: string, instance: any) {
  if (!text) {
    return text;
  }
  switch (type) {
    case 'class': {
      let additionalClasses = '';
      if (text === 'form-group') {
        additionalClasses = 'mb-3 ';
        if (instance && instance.component.block) {
          additionalClasses += 'grid ';
        }
      }
      return `${additionalClasses}${Object.prototype.hasOwnProperty.call(this.cssClasses, text.toString()) ? this.cssClasses[text.toString()] : text}`;
    }
  }
  return text;
},
defaultIconset: 'lucide',
```

**Step 3: Run tests**

Run: `cd goformx-formio && npm test`
Expected: PASS

**Step 4: Commit**

```bash
cd goformx-formio
git add src/templates/goforms/cssClasses.ts src/templates/goforms/index.ts
git commit -m "feat: update cssClasses and transform for Tailwind + shadcn tokens"
```

---

### Task 3: Convert Tier 1 Form Field Templates

**Files:** All `form.ejs` files in these directories under `goformx-formio/src/templates/goforms/`:
- `input/form.ejs`
- `button/form.ejs`
- `checkbox/form.ejs`
- `radio/form.ejs`
- `select/form.ejs`
- `selectOption/form.ejs`
- `field/form.ejs`
- `label/form.ejs`
- `errorsList/form.ejs`
- `alert/form.ejs`
- `message/form.ejs`
- `icon/form.ejs`

**Conversion instructions:** For each file, apply the class reference table from above. Specific conversions per template:

#### input/form.ejs
- `input-group` → `flex items-stretch`
- `input-group-text` → `inline-flex items-center rounded-md border border-input bg-muted px-3 text-sm text-muted-foreground`
- `form-floating` → `relative`
- `col-form-label` → `text-sm font-medium leading-none text-foreground`
- `form-text` → `text-sm text-muted-foreground`
- `float-end` → `float-right`
- `text-end` → `text-right`
- `text-muted` → `text-muted-foreground`
- `visually-hidden` → `sr-only`

#### button/form.ejs
- `text-muted` → `text-muted-foreground`
- `help-block` → `text-sm text-muted-foreground`

#### checkbox/form.ejs
- `form-check checkbox` → `flex items-start gap-2`
- `form-check-label` → `text-sm font-normal leading-none`
- `text-muted` → `text-muted-foreground`

#### radio/form.ejs
- `form-radio radio` → `space-y-2`
- `form-check` → `flex items-center gap-2`
- `form-check-inline` → `inline-flex items-center gap-2`
- `form-check-label` → `text-sm font-normal leading-none`
- `ps-0` → `pl-0`

#### select/form.ejs
- No Bootstrap-specific classes to change (uses `ctx.input.attr` for class)

#### field/form.ejs
- `form-text text-muted` → `text-sm text-muted-foreground mt-1.5`

#### label/form.ejs
- `col-form-label` → `text-sm font-medium leading-none text-foreground`
- `visually-hidden` → `sr-only`
- `text-muted` → `text-muted-foreground`

#### icon/form.ejs
- Change from `<i ref="{{ctx.ref}}" class="{{ctx.className}}" style="{{ctx.styles}}">{{ctx.content}}</i>` to render inline SVG when possible. Keep the `<i>` wrapper for CSS positioning but inject SVG content.

#### errorsList/form.ejs
- No Bootstrap classes to change (just uses formio-specific classes)

#### alert/form.ejs
- No Bootstrap classes to change (class comes from `ctx.attrs`)

#### message/form.ejs
- No Bootstrap classes (uses inline class from `ctx.level`)

**Step 1: Convert each template file**

Apply the class mapping table to each file. Keep all `ref=` attributes, `aria-*` attributes, `ctx.*` interpolations, and EJS logic (`{% %}`) exactly as-is. Only change class strings.

**Step 2: Build the package**

Run: `cd goformx-formio && npm run build`
Expected: Build succeeds with no TypeScript or EJS errors

**Step 3: Commit**

```bash
cd goformx-formio
git add src/templates/goforms/input/ src/templates/goforms/button/ src/templates/goforms/checkbox/ src/templates/goforms/radio/ src/templates/goforms/select/ src/templates/goforms/selectOption/ src/templates/goforms/field/ src/templates/goforms/label/ src/templates/goforms/errorsList/ src/templates/goforms/alert/ src/templates/goforms/message/ src/templates/goforms/icon/
git commit -m "feat: convert Tier 1 form field templates to Tailwind + shadcn"
```

---

### Task 4: Convert Tier 2 Layout Templates

**Files:** Under `goformx-formio/src/templates/goforms/`:
- `panel/form.ejs`
- `columns/form.ejs`
- `fieldset/form.ejs`
- `container/form.ejs`
- `well/form.ejs`
- `tab/form.ejs`
- `table/form.ejs`
- `webform/form.ejs`
- `components/form.ejs`
- `tableComponents/form.ejs`

**Conversion notes per template:**

#### panel/form.ejs
- `mb-2 card border` → `mb-3 rounded-lg border border-border bg-card text-card-foreground shadow-sm`
- `card-header` → `flex items-center justify-between border-b border-border px-4 py-3`
- `card-title` → `text-sm font-semibold`
- `card-body` → `p-4`
- `text-light` → `text-primary-foreground`
- `text-muted` → `text-muted-foreground`
- `bg-{theme}` dynamic class: keep the `ctx.transform('class', 'bg-' + ctx.component.theme)` call — add theme mappings to transform if needed

#### columns/form.ejs
- `col-{{column.size}}-{{column.currentWidth}}` → `col-span-{{column.currentWidth}}`
- `offset-{{column.size}}-{{column.offset}}` → dynamically calculate `col-start` if offset > 0
- Wrap columns in a parent `grid grid-cols-12 gap-4` div

#### well/form.ejs
- `card card-body bg-light mb-3` → `rounded-lg border border-border bg-muted p-4 mb-3`

#### tab/form.ejs
- `card` → `rounded-lg border border-border bg-card`
- `card-header` → `border-b border-border`
- `nav nav-tabs card-header-tabs` → `flex`
- `nav-item` → (remove class or leave empty)
- `nav-link` → `inline-flex items-center px-4 py-2 text-sm font-medium text-muted-foreground hover:text-foreground border-b-2 border-transparent transition-colors`
- `nav-link active` → add `border-primary text-foreground`
- `card-body tab-pane` → `p-4`
- `card-vertical` → `flex flex-row`
- `nav-link-vertical` → add vertical-specific styles
- `nav-tabs-vertical` → `flex flex-col border-b-0 border-r border-border`

#### table/form.ejs
- `table` → `w-full text-sm`
- `table-bordered` → `border border-border [&_th]:border [&_th]:border-border [&_td]:border [&_td]:border-border`
- `table-striped` → `[&_tbody_tr:nth-child(odd)]:bg-muted/50`
- `table-hover` → `[&_tbody_tr]:hover:bg-muted/50`
- `table-sm` → `[&_th]:px-2 [&_th]:py-1 [&_td]:px-2 [&_td]:py-1`
- `visually-hidden` → `sr-only`

#### fieldset/form.ejs
- `text-muted` → `text-muted-foreground`
- `formio-clickable` → `cursor-pointer` (keep formio-clickable too if it's a JS hook)

#### container/form.ejs, components/form.ejs, webform/form.ejs, tableComponents/form.ejs
- No Bootstrap classes to change — these are structural wrappers

**Step 1: Convert each template**

Apply the class reference table. For `columns/form.ejs`, add a wrapping `<div class="grid grid-cols-12 gap-4">` since Bootstrap's `.row` is implicit on the parent.

**Step 2: Build**

Run: `cd goformx-formio && npm run build`
Expected: PASS

**Step 3: Commit**

```bash
cd goformx-formio
git add src/templates/goforms/panel/ src/templates/goforms/columns/ src/templates/goforms/fieldset/ src/templates/goforms/container/ src/templates/goforms/well/ src/templates/goforms/tab/ src/templates/goforms/table/ src/templates/goforms/webform/ src/templates/goforms/components/ src/templates/goforms/tableComponents/
git commit -m "feat: convert Tier 2 layout templates to Tailwind + shadcn"
```

---

### Task 5: Convert Tier 3 Builder Chrome Templates

**Files:** Under `goformx-formio/src/templates/goforms/`:
- `builder/form.ejs`
- `builderComponent/form.ejs`
- `builderComponents/form.ejs`
- `builderEditForm/form.ejs`
- `builderPlaceholder/form.ejs`
- `builderSidebar/form.ejs`
- `builderSidebarGroup/form.ejs`
- `builderWizard/form.ejs`

**Conversion notes per template:**

#### builder/form.ejs
- `row formbuilder` → `grid grid-cols-12 gap-0 formbuilder`
- `col-xs-4 col-sm-3 col-md-2 formcomponents` → `col-span-2 formcomponents`
- `col-xs-8 col-sm-9 col-md-10 formarea` → `col-span-10 formarea`

#### builderComponent/form.ejs
- `btn btn-xxs btn-danger` → `inline-flex items-center justify-center rounded-md bg-destructive text-destructive-foreground px-1.5 py-0.5 text-xs shadow-sm hover:bg-destructive/90 transition-colors`
- `btn btn-xxs btn-default` → `inline-flex items-center justify-center rounded-md border border-input bg-background text-foreground px-1.5 py-0.5 text-xs shadow-sm hover:bg-accent transition-colors`
- `btn btn-xxs btn-secondary` → `inline-flex items-center justify-center rounded-md bg-secondary text-secondary-foreground px-1.5 py-0.5 text-xs shadow-sm hover:bg-secondary/80 transition-colors`

#### builderSidebar/form.ejs
- `accordion builder-sidebar` → `divide-y divide-border rounded-lg border border-border builder-sidebar`
- `form-control builder-sidebar_search` → `flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring builder-sidebar_search`

#### builderSidebarGroup/form.ejs
- `card form-builder-panel accordion-item` → `border-b border-border form-builder-panel`
- `card-header form-builder-group-header p-0` → `form-builder-group-header p-0`
- `mb-0 mt-0 d-grid accordion-header` → `mb-0 mt-0 grid`
- `btn builder-group-button` → `inline-flex w-full items-center justify-start px-3 py-2 text-sm font-medium text-foreground hover:bg-accent transition-colors builder-group-button`
- `accordion-collapse collapse` → `overflow-hidden transition-all`
- `show` → (remove — controlled by JS)
- `d-grid gap-1 no-drop p-2 w-100` → `grid gap-1 no-drop p-2 w-full`
- `btn btn-outline-primary btn-sm formcomponent drag-copy` → `inline-flex items-center justify-start gap-1 rounded-md border border-primary text-primary bg-transparent px-3 py-1.5 text-sm font-medium hover:bg-primary hover:text-primary-foreground transition-colors formcomponent drag-copy`
- Remove `data-bs-toggle`, `data-bs-target`, `data-bs-parent` attributes (these are Bootstrap JS-specific — Form.io handles accordion state via its own JS)

#### builderEditForm/form.ejs
- `row` → `grid grid-cols-12 gap-4`
- `col col-sm-6` → `col-span-6`
- `col col-sm-12` → `col-span-12`
- `lead` → `text-lg font-medium`
- `float-end` → `float-right`
- `btn btn-success` → green button style
- `btn btn-secondary` → secondary button style
- `btn btn-danger` → destructive button style
- `btn btn-primary float-right` → primary button + `float-right`
- `card panel preview-panel` → `rounded-lg border border-border bg-card shadow-sm`
- `card-header` → `border-b border-border px-4 py-3`
- `card-title mb-0` → `text-sm font-semibold mb-0`
- `card-body` → `p-4`
- `card card-body bg-light formio-settings-help` → `rounded-lg border border-border bg-muted p-4`

#### builderPlaceholder/form.ejs
- `alert alert-info` → `rounded-md border border-blue-500/50 bg-blue-500/10 px-4 py-3 text-sm text-blue-700 dark:text-blue-400`

#### builderWizard/form.ejs
- Same grid conversions as `builder/form.ejs`
- `breadcrumb wizard-pages` → `flex flex-wrap items-center gap-2 mb-4 wizard-pages`
- `badge bg-primary` → primary badge style
- `badge bg-info` → info badge style
- `badge bg-success` → success badge style
- `me-2` → `mr-2`

**Step 1: Convert each template**

Apply the class reference table. Keep all `data-noattach`, `ref=`, and structural classes that Form.io's JS uses as hooks (like `formcomponents`, `formarea`, `builder-component`, `component-btn-group`, `component-settings-button`, `drag-copy`, etc.).

**Step 2: Build**

Run: `cd goformx-formio && npm run build`
Expected: PASS

**Step 3: Commit**

```bash
cd goformx-formio
git add src/templates/goforms/builder/ src/templates/goforms/builderComponent/ src/templates/goforms/builderComponents/ src/templates/goforms/builderEditForm/ src/templates/goforms/builderPlaceholder/ src/templates/goforms/builderSidebar/ src/templates/goforms/builderSidebarGroup/ src/templates/goforms/builderWizard/
git commit -m "feat: convert Tier 3 builder chrome templates to Tailwind + shadcn"
```

---

### Task 6: Convert Tier 4 Modals and Specialized Templates

**Files:** Under `goformx-formio/src/templates/goforms/`:
- `modaldialog/form.ejs`, `modaledit/form.ejs`, `modalPreview/form.ejs`, `componentModal/form.ejs`, `dialog/form.ejs`
- `signature/form.ejs`, `file/form.ejs`, `datagrid/form.ejs`, `editgrid/form.ejs`, `editgridTable/form.ejs`
- `day/form.ejs`, `address/form.ejs`, `survey/form.ejs`, `tree/form.ejs`, `tree/partials/edit.ejs`, `tree/partials/view.ejs`
- `pdf/form.ejs`, `pdfBuilder/form.ejs`, `pdfBuilderUpload/form.ejs`
- `wizard/form.ejs`, `wizardHeader/form.ejs`, `wizardHeaderClassic/form.ejs`, `wizardHeaderVertical/form.ejs`, `wizardNav/form.ejs`
- `html/form.ejs`, `loader/form.ejs`, `loading/form.ejs`, `map/form.ejs`
- `multiValueRow/form.ejs`, `multiValueTable/form.ejs`, `multipleMasksInput/form.ejs`, `resourceAdd/form.ejs`
- `component/form.ejs`

**Conversion approach:** Same class reference table. Key conversions per group:

#### Modal templates
- Keep all `formio-dialog*` classes (they're JS hooks)
- `btn btn-primary btn-xs` → primary button with xs size
- `btn btn-warning` → warning button style
- `btn btn-secondary btn-sm` → secondary button sm
- `btn btn-success` → green button
- `float-end` → `float-right`
- `visually-hidden` → `sr-only`

#### Signature
- `btn btn-sm btn-light` → light button sm
- `form-control-feedback text-danger` → `text-destructive`

#### File
- `list-group list-group-striped` → `divide-y divide-border rounded-lg border border-border`
- `list-group-item` → `px-4 py-3`
- `list-group-header` → `font-semibold bg-muted/50`
- `row` → `grid grid-cols-12 gap-4`
- `col-md-{n}` → `col-span-{n}`
- `btn btn-primary btn-sm` → primary button sm
- `progress` → `h-2 w-full overflow-hidden rounded-full bg-muted`
- `progress-bar` → `h-full bg-primary transition-all`
- `alert alert-warning` → warning alert style
- `visually-hidden` → `sr-only`
- `text-danger` → `text-destructive`
- `text-end` → `text-right`

#### Datagrid
- `table datagrid-table table-bordered` → `w-full text-sm border border-border [&_th]:border [&_th]:border-border [&_td]:border [&_td]:border-border datagrid-table`
- Same table modifiers (striped, hover, sm)
- `btn btn-primary formio-button-add-row` → primary button
- `btn btn-secondary formio-button-remove-row` → secondary button
- `btn btn-default bi bi-list` → default button (remove `bi bi-list` — the reorder handle)
- `text-muted` → `text-muted-foreground`
- `visually-hidden` → `sr-only`

#### EditGrid
- `editgrid-listgroup list-group` → `divide-y divide-border rounded-lg border border-border editgrid-listgroup`
- `list-group-item` → `px-4 py-3`
- `btn btn-primary` / `btn btn-danger` → styled buttons
- `has-error` → `text-destructive`
- `help-block` → `text-sm text-destructive`

#### Day
- `row` → `grid grid-cols-12 gap-4`
- `col col-xs-3` → `col-span-3`
- `col col-xs-4` → `col-span-4`
- `col col-xs-5` → `col-span-5`
- `form-control` → Tailwind input classes
- `field-required` → `after:content-['*'] after:ml-0.5 after:text-destructive`

#### Address
- `form-check checkbox` → `flex items-start gap-2`
- `form-check-label` → `text-sm font-normal leading-none`
- `form-check-input` → checkbox input classes
- `fa fa-times bi bi-x` → remove (will use Lucide x icon)

#### Survey
- `table table-striped table-bordered` → Tailwind table classes
- `text-muted` → `text-muted-foreground`

#### Tree
- `list-group-item` → `px-4 py-3 border-b border-border`
- `list-group` → `divide-y divide-border`
- `col-sm-2` → `col-span-2`
- `col-sm-3` → `col-span-3`
- `btn-group pull-right` → `flex gap-1 ml-auto`
- `btn btn-default btn-sm` → default button sm
- `btn btn-danger btn-sm` → destructive button sm

#### PDF
- `btn btn-default btn-secondary` → default button

#### Wizard
- `row` → `grid grid-cols-12 gap-4`
- `col-sm-2` → `col-span-2`
- `col-sm-10` → `col-span-10`
- `col-sm-offset-2` → `col-start-3`

#### WizardHeader
- `pagination` → `flex gap-1`
- `page-item` → (style on child)
- `page-link` → `inline-flex items-center justify-center rounded-md px-3 py-1.5 text-sm font-medium border border-input hover:bg-accent transition-colors`
- `page-item active` → add `bg-primary text-primary-foreground` to page-link

#### WizardNav
- `list-inline` → `flex flex-wrap gap-2`
- Button classes per button type (cancel=secondary, previous=primary, next=primary, submit=primary)

#### MultiValueRow/MultiValueTable
- `table table-bordered` → Tailwind table
- `btn btn-secondary` → secondary button
- `btn btn-primary formio-button-add-another` → primary button

#### ResourceAdd
- Same as MultiValueTable

#### Component
- `invalid-feedback` → `text-sm text-destructive`

**Step 1: Convert all templates in this tier**

Apply the class reference table to each file. This is the largest batch — work through systematically.

**Step 2: Build**

Run: `cd goformx-formio && npm run build`
Expected: PASS

**Step 3: Commit**

```bash
cd goformx-formio
git add src/templates/goforms/
git commit -m "feat: convert Tier 4 modal and specialized templates to Tailwind + shadcn"
```

---

### Task 7: Convert HTML-Mode Templates

**Files:** All `html.ejs` files under `goformx-formio/src/templates/goforms/`:
- `input/html.ejs` — no Bootstrap classes, no change needed
- `button/html.ejs` — empty file, no change needed
- `checkbox/html.ejs` — `{{ctx.input.labelClass}}` is dynamic, no change needed
- `radio/html.ejs` — no Bootstrap classes, no change needed
- `select/html.ejs` — no Bootstrap classes, no change needed
- `selectOption/html.ejs` — no Bootstrap classes, no change needed
- `address/html.ejs` — no Bootstrap classes, no change needed
- `signature/html.ejs` — no Bootstrap classes, no change needed
- `datagrid/html.ejs` — `table datagrid-table table-bordered` → same table conversion
- `editgrid/html.ejs` — `list-group` → same list conversion, button classes
- `editgridTable/html.ejs` — `table` → same table conversion, button classes
- `survey/html.ejs` — `table table-striped table-bordered` → same table conversion

**Step 1: Convert the 4 templates that need changes**

Only `datagrid/html.ejs`, `editgrid/html.ejs`, `editgridTable/html.ejs`, and `survey/html.ejs` have Bootstrap classes to convert. Apply the same class mappings used in their `form.ejs` counterparts.

**Step 2: Build**

Run: `cd goformx-formio && npm run build`
Expected: PASS

**Step 3: Commit**

```bash
cd goformx-formio
git add src/templates/goforms/
git commit -m "feat: convert HTML-mode templates to Tailwind"
```

---

### Task 8: Create Structural CSS File

**Files:**
- Create: `goformx-formio/src/css/formio-structural.css`

**Context:** `@formio/js/dist/formio.full.css` bundles Bootstrap + Form.io structural CSS together. We need to extract just the structural styles that Form.io's JS relies on (visibility toggles, dialog overlays, drag-and-drop positioning, etc.) and provide them separately.

**Step 1: Identify structural CSS classes**

Search `@formio/js` source for CSS classes used by the JS runtime (not for styling). Key structural classes:

```css
/* formio-structural.css — JS-driven structural styles only */

/* Dialog overlays */
.formio-dialog {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
}

.formio-dialog-overlay {
  position: fixed;
  inset: 0;
  background-color: hsl(var(--foreground) / 0.5);
  z-index: -1;
}

.formio-dialog-content,
.formio-modaledit-content {
  position: relative;
  z-index: 51;
  max-width: 42rem;
  width: 100%;
  max-height: 90vh;
  overflow-y: auto;
  background-color: hsl(var(--background));
  border: 1px solid hsl(var(--border));
  border-radius: var(--radius);
  padding: 1.5rem;
  box-shadow: 0 25px 50px -12px rgb(0 0 0 / 0.25);
}

.formio-dialog-close {
  position: absolute;
  top: 0.75rem;
  right: 0.75rem;
  cursor: pointer;
}

/* Component visibility (JS-toggled) */
.formio-component-hidden { display: none !important; }
.formio-hidden { display: none !important; }
.component-rendering-hidden { display: none !important; }

/* Drag and drop */
.drag-container { min-height: 2rem; }
.formio-builder-form .drag-container { padding: 0.5rem; }
.drag-copy { cursor: grab; }
.gu-mirror { cursor: grabbing; opacity: 0.8; }
.gu-transit { opacity: 0.2; }
.gu-hide { display: none !important; }
.gu-unselectable { user-select: none; }

/* Builder component positioning */
.builder-component { position: relative; }
.component-btn-group {
  position: absolute;
  right: 0;
  top: 0;
  z-index: 10;
  display: none;
}
.builder-component:hover > .component-btn-group,
.builder-component:focus-within > .component-btn-group {
  display: flex;
  gap: 0.125rem;
}

/* Disabled state */
.formio-disabled-input { pointer-events: none; opacity: 0.5; }

/* Loader */
.formio-loader { position: relative; }
.loader-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
}

/* Choices.js select widget overrides */
.choices__inner {
  background-color: hsl(var(--background)) !important;
  border-color: hsl(var(--input)) !important;
  border-radius: var(--radius) !important;
  min-height: 2.5rem !important;
  padding: 0.25rem 0.5rem !important;
  font-size: 0.875rem !important;
}
.choices__list--dropdown {
  background-color: hsl(var(--background)) !important;
  border-color: hsl(var(--input)) !important;
  border-radius: var(--radius) !important;
  z-index: 100 !important;
}
.choices__item--selectable.is-highlighted {
  background-color: hsl(var(--accent)) !important;
  color: hsl(var(--accent-foreground)) !important;
}
.choices__input {
  background-color: transparent !important;
  font-size: 0.875rem !important;
}

/* Collapse animation (replaces Bootstrap collapse) */
.formio-collapse-icon { cursor: pointer; }

/* Signature pad */
.signature-pad-body { position: relative; }
.signature-pad-canvas { border: 1px solid hsl(var(--border)); border-radius: var(--radius); }
.signature-pad-refresh { position: absolute; top: 0.25rem; right: 0.25rem; z-index: 1; }

/* File upload */
.fileSelector {
  padding: 1rem;
  border: 2px dashed hsl(var(--border));
  border-radius: var(--radius);
  text-align: center;
}
.fileSelector:hover { border-color: hsl(var(--primary)); }

/* Inline icon styling */
.formio-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1em;
  height: 1em;
  vertical-align: -0.125em;
}
.formio-icon svg {
  width: 100%;
  height: 100%;
}

/* Reset margins for modal content */
.reset-margins > * { margin: 0; }

/* Accordion state for builder sidebar */
.builder-sidebar_search {
  margin-bottom: 0.5rem;
}
```

**Step 2: Add CSS file to the package exports**

Update `goformx-formio/package.json` to include the CSS file in the `exports` and `files` fields:

```json
"exports": {
  ".": { /* existing */ },
  "./components": { /* existing */ },
  "./css": "./src/css/formio-structural.css"
},
```

**Step 3: Commit**

```bash
cd goformx-formio
git add src/css/formio-structural.css package.json
git commit -m "feat: add formio-structural.css with JS-driven styles and Choices.js theming"
```

---

### Task 9: Update Laravel App — Remove Bootstrap, Wire Tailwind

**Files:**
- Modify: `goformx-laravel/resources/css/app.css`
- Modify: `goformx-laravel/package.json`

**Step 1: Update app.css**

Remove the three Bootstrap/Form.io CSS imports and replace with the structural CSS:

```css
/* REMOVE these three lines: */
/* @import 'bootstrap/dist/css/bootstrap.min.css' layer(formio); */
/* @import 'bootstrap-icons/font/bootstrap-icons.css' layer(formio); */
/* @import '@formio/js/dist/formio.full.css' layer(formio); */

/* ADD this line (before tailwindcss): */
@import '@goformx/formio/css';

@import 'tailwindcss';
/* ... rest stays the same */
```

**Step 2: Remove Bootstrap npm dependencies**

Run: `cd goformx-laravel && ddev exec npm uninstall bootstrap bootstrap-icons`

**Step 3: Build frontend**

Run: `cd goformx-laravel && ddev exec npm run build`
Expected: Build succeeds with no errors

**Step 4: Commit**

```bash
cd goformx-laravel
git add resources/css/app.css package.json package-lock.json
git commit -m "feat: remove Bootstrap deps, import formio-structural.css"
```

---

### Task 10: Visual Testing and Polish

**Files:** Various — based on issues found

**Step 1: Start the dev environment**

Run: `cd /home/jones/dev/goformx && task dev`

**Step 2: Test form builder**

1. Navigate to the form builder page
2. Verify the builder sidebar renders with Tailwind styling
3. Drag a component onto the canvas
4. Verify the builder component chrome (action buttons) renders correctly
5. Click edit on a component — verify the edit form modal renders

**Step 3: Test form fields**

Create a form with each field type and verify styling:
- Text field, textarea, number, email, phone
- Checkbox, radio buttons, select dropdown
- Date/time picker
- Panel, columns, fieldset, well
- Button

**Step 4: Test form preview**

1. Click preview — verify read-only form renders with Tailwind styling
2. Check dark mode toggle — verify all elements respond to theme change

**Step 5: Test public form fill**

1. Open a public form URL
2. Verify form renders with Tailwind styling
3. Submit the form — verify validation errors render correctly

**Step 6: Fix any visual issues**

Iterate on template fixes as needed. Common issues to watch for:
- Missing spacing (Bootstrap's default margins vs Tailwind's reset)
- Collapsed accordion panels not toggling (removed Bootstrap JS data attributes)
- Choices.js select widget styling gaps
- Dark mode contrast issues

**Step 7: Commit fixes**

```bash
git add -A
git commit -m "fix: visual polish for Tailwind Form.io templates"
```

---

### Task 11: Bump Package Version and Final Build

**Files:**
- Modify: `goformx-formio/package.json` (version bump)

**Step 1: Bump version**

Update version from `0.1.4` to `0.2.0` (minor bump — breaking change for consumers expecting Bootstrap output).

**Step 2: Full build and test**

Run:
```bash
cd goformx-formio && npm run build && npm test
cd ../goformx-laravel && ddev exec npm run build
```

**Step 3: Commit**

```bash
cd goformx-formio
git add package.json
git commit -m "chore: bump @goformx/formio to 0.2.0 for Tailwind template conversion"
```
