# Search Frontend Obsidian Editorial Redesign — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Overhaul the search-frontend from a light teal/serif theme to a dark "Obsidian Editorial" aesthetic with Bing News-inspired layout patterns.

**Architecture:** Pure visual/layout refactor — no API changes, no new dependencies, no backend work. Every file in `search-frontend/src/` gets restyled. Source Sans 3 replaces both Instrument Serif and DM Sans. Dark palette replaces light. Bing-style horizontal category tabs and ordering controls replace the current sidebar-centric layout.

**Tech Stack:** Vue 3 + TypeScript + Tailwind CSS v4 + Vite (unchanged)

**Design doc:** `docs/plans/2026-02-26-search-frontend-redesign-design.md`

---

## Task 1: Foundation — Font swap and dark design tokens

**Files:**
- Modify: `search-frontend/index.html`
- Modify: `search-frontend/src/style.css`

**Step 1: Swap Google Fonts in index.html**

Replace the current font link (DM Sans + Instrument Serif) with Source Sans 3:

```html
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Source+Sans+3:ital,wght@0,400;0,500;0,600;0,700;1,400&display=swap" rel="stylesheet">
```

**Step 2: Replace entire style.css with dark palette**

Replace all CSS custom properties with the dark Obsidian Editorial tokens from the design doc. Key changes:
- `--font-display` and `--font-body` both become `"Source Sans 3", system-ui, sans-serif`
- All `--nc-bg-*` tokens become dark values (#111114, #1a1a1f, #242429, #2a2a30)
- All `--nc-text-*` tokens become light values (#f0f0f3, #a0a0ab, #6b6b78)
- `--nc-primary` becomes `#3b82f6` (electric blue)
- Shadows become darker (rgba(0,0,0,...))
- Background `body::before` radial gradient removed
- Noise overlay opacity reduced to 0.02
- `.font-display` class stays (now renders Source Sans 3)

```css
@import "tailwindcss";

:root {
  --font-display: "Source Sans 3", system-ui, sans-serif;
  --font-body: "Source Sans 3", system-ui, sans-serif;

  --nc-bg: #111114;
  --nc-bg-elevated: #1a1a1f;
  --nc-bg-muted: #242429;
  --nc-bg-surface: #2a2a30;
  --nc-border: #2e2e35;
  --nc-border-strong: #3e3e48;

  --nc-text: #f0f0f3;
  --nc-text-secondary: #a0a0ab;
  --nc-text-muted: #6b6b78;

  --nc-primary: #3b82f6;
  --nc-primary-hover: #60a5fa;
  --nc-primary-muted: rgba(59, 130, 246, 0.12);
  --nc-accent: #f59e0b;
  --nc-accent-hover: #d97706;
  --nc-accent-muted: rgba(245, 158, 11, 0.12);

  --nc-success: #22c55e;
  --nc-success-muted: rgba(34, 197, 94, 0.12);
  --nc-warning: #f59e0b;
  --nc-warning-muted: rgba(245, 158, 11, 0.12);
  --nc-error: #ef4444;
  --nc-error-muted: rgba(239, 68, 68, 0.12);

  --nc-highlight-bg: rgba(59, 130, 246, 0.2);
  --nc-highlight-fg: #93c5fd;

  --nc-shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.3);
  --nc-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
  --nc-shadow-lg: 0 12px 28px rgba(0, 0, 0, 0.5);

  --nc-ease-out: cubic-bezier(0.22, 1, 0.36, 1);
  --nc-duration-fast: 150ms;
  --nc-duration: 250ms;
  --nc-duration-slow: 400ms;
}
```

Body styles: keep box-sizing reset, scroll-behavior, font-family, antialiased. Change `body::before` to remove the teal radial gradient (flat dark bg). Keep noise `body::after` but reduce to `opacity: 0.02`.

**Step 3: Verify dev server renders with dark background and new font**

Run: `cd search-frontend && npm run dev`
Expected: Page renders with dark #111114 background, Source Sans 3 text, all existing components visible (colors will look wrong until restyled — that's expected).

**Step 4: Commit**

```bash
git add search-frontend/index.html search-frontend/src/style.css
git commit -m "feat(search-frontend): dark palette + Source Sans 3 font foundation"
```

---

## Task 2: App shell — Dark header with category tabs

**Files:**
- Modify: `search-frontend/src/App.vue`

**Step 1: Restyle the header**

The header already uses `--nc-bg-elevated` and `--nc-border` CSS vars, so the dark palette from Task 1 will auto-apply. Changes needed:
- Remove `font-display` class from "North Cloud" text (was Instrument Serif, now use body font)
- Change wordmark to `font-semibold text-xl tracking-tight` (Source Sans 3 at 600 weight)
- Restyle search input for dark: `bg-[var(--nc-bg-surface)]` instead of `bg-[var(--nc-bg-muted)]`

**Step 2: Add horizontal category tabs below header**

Add a nav element inside the header's sticky container, below the search bar row. Tabs: All, Top Stories, Crime, Mining, Entertainment. Use `router-link` for navigation:
- "All" links to `/` (home)
- "Top Stories" links to `/search?topics=top_stories` or scrolls to section on home
- "Crime" links to `/search?topics=crime`
- "Mining" links to `/search?topics=mining`
- "Entertainment" links to `/search?topics=entertainment`

Active tab detection: check `route.query.topics` or `route.name` to determine which tab is active.

Tab styling:
- Default: `text-[var(--nc-text-muted)] hover:text-[var(--nc-text-secondary)]`
- Active: `text-[var(--nc-primary)]` with a 2px bottom border in `--nc-primary`
- Tab row has a top border `border-t border-[var(--nc-border)]`

**Step 3: Verify header renders dark with tabs**

Run dev server, confirm:
- Dark elevated header background
- "North Cloud" in Source Sans 3 (no serif)
- Category tabs visible with correct active state
- Search input styled dark

**Step 4: Commit**

```bash
git add search-frontend/src/App.vue
git commit -m "feat(search-frontend): dark header with category navigation tabs"
```

---

## Task 3: Search bar — Dark restyle

**Files:**
- Modify: `search-frontend/src/components/search/SearchBar.vue`

**Step 1: Restyle search input**

Change the input classes:
- Background: `bg-[var(--nc-bg-surface)]` (was `bg-[var(--nc-bg-elevated)]`)
- Border: `border-[var(--nc-border)]` (keep)
- Focus: `focus:ring-2 focus:ring-[var(--nc-primary)]/30 focus:border-[var(--nc-primary)]`
- Text: `text-[var(--nc-text)]` (keep — now resolves to light)
- Placeholder: `placeholder-[var(--nc-text-muted)]` (keep — now resolves to dim)

**Step 2: Restyle dropdown**

Change dropdown container:
- Background: `bg-[var(--nc-bg-elevated)]` (keep — now dark)
- Border: `border-[var(--nc-border)]` (keep — now dark)
- Hover items: `hover:bg-[var(--nc-bg-muted)]` instead of `hover:bg-[var(--nc-primary-muted)]` for a subtler dark hover
- Section headers ("Suggestions", "Recent"): `text-[var(--nc-text-muted)]` (keep)

**Step 3: Restyle search button**

Change from amber to blue:
- `bg-[var(--nc-primary)] hover:bg-[var(--nc-primary-hover)] focus:ring-[var(--nc-primary)]`

**Step 4: Verify search bar looks correct on dark bg**

Run dev server, navigate to `/search`, confirm input and dropdown render cleanly on dark.

**Step 5: Commit**

```bash
git add search-frontend/src/components/search/SearchBar.vue
git commit -m "feat(search-frontend): dark search bar with blue accent"
```

---

## Task 4: News cards and channel sections — Dark restyle

**Files:**
- Modify: `search-frontend/src/components/news/NewsCard.vue`
- Modify: `search-frontend/src/components/news/ChannelSection.vue`

**Step 1: Restyle NewsCard**

Most classes already use CSS vars that auto-resolve to dark. Specific changes:
- Card: keep `bg-[var(--nc-bg-elevated)]` and `border-[var(--nc-border)]` — auto-dark
- Hover: change from `hover:shadow-[var(--nc-shadow-lg)]` to `hover:border-[var(--nc-border-strong)]` (border brightens, no shadow on dark)
- Fallback div (no image): keep channel color but change opacity from 0.15 to 0.25 for better visibility on dark
- Source text: `text-[var(--nc-text-secondary)]` (was `text-[var(--nc-text-secondary)]` — no change, auto-resolves)
- Headline hover: `group-hover:text-[var(--nc-primary)]` (keep — now blue instead of teal)

**Step 2: Restyle ChannelSection**

- Section title: remove `font-display` class (was Instrument Serif). Use `font-semibold text-2xl sm:text-3xl` instead
- "See more" link: `text-[var(--nc-primary)]` (keep — now blue)
- Skeleton cards: already use `--nc-bg-muted` which auto-resolves to dark

**Step 3: Verify home page looks correct**

Run dev server, confirm:
- Channel titles in Source Sans 3 (no serif)
- Cards dark with subtle borders
- Hover shows border brightening, not shadow

**Step 4: Commit**

```bash
git add search-frontend/src/components/news/NewsCard.vue search-frontend/src/components/news/ChannelSection.vue
git commit -m "feat(search-frontend): dark news cards and channel sections"
```

---

## Task 5: Home page — Hero layout and trending strip

**Files:**
- Modify: `search-frontend/src/views/HomeView.vue`

**Step 1: Add hero section for Top Stories**

Replace the plain `<ChannelSection>` for Top Stories with a custom hero layout:
- 2-column grid: `grid grid-cols-1 lg:grid-cols-3 gap-4`
- Left (2/3): First item as a large hero card with 16:9 image, gradient scrim at bottom (`bg-gradient-to-t from-black/80 via-black/30 to-transparent`), headline overlay, source + time
- Right (1/3): Stack of 2-3 smaller cards (use existing NewsCard but compact)
- Fallback to regular grid if fewer than 2 items

**Step 2: Add trending strip**

Between hero and channel sections, add a "Trending on North Cloud" horizontal strip:
- Container: `overflow-x-auto flex gap-2 py-4`
- Pills: `rounded-full px-3 py-1.5 text-sm bg-[var(--nc-bg-surface)] text-[var(--nc-text-secondary)] border border-[var(--nc-border)] hover:border-[var(--nc-primary)] hover:text-[var(--nc-primary)]`
- Each pill links to `/search?topics=X`
- Use static channel slugs: crime, mining, entertainment, local_news, technology, politics, sports

**Step 3: Verify home page hero renders**

Run dev server, confirm:
- Hero section with large card + smaller side cards
- Trending strip scrollable
- Channel sections below

**Step 4: Commit**

```bash
git add search-frontend/src/views/HomeView.vue
git commit -m "feat(search-frontend): hero layout and trending strip on homepage"
```

---

## Task 6: Results page — Bing-style layout with category sidebar and ordering pills

**Files:**
- Modify: `search-frontend/src/views/ResultsView.vue`

This is the largest single task. The ResultsView layout changes significantly.

**Step 1: Add category sidebar (left rail)**

Replace the FilterSidebar in the `<aside>` with a category navigation list:
- Categories: Top Stories, Crime, Mining, Entertainment, with icons (use simple SVG or emoji-free labels)
- Active category detected from `filters.topics`
- Clicking a category sets `filters.topics = [slug]` and searches
- Styled: vertical list, active item gets `bg-[var(--nc-primary-muted)] text-[var(--nc-primary)]` with left border accent
- Width: `w-48` (narrower than old sidebar)

**Step 2: Add ordering pills row**

Above the results list, add a row:
- "Order by" label
- Pill buttons: **Best match** (active when `sortBy === 'relevance'`), **Most fresh** (active when `sortBy === 'published_date'`)
- "Any time" dropdown button with options: Past hour, Past 24 hours, Past 7 days, Past 30 days
- Best match pill filled: `bg-[var(--nc-primary)] text-white rounded-full px-4 py-1.5`
- Inactive pills: `bg-transparent border border-[var(--nc-border)] text-[var(--nc-text-secondary)] rounded-full px-4 py-1.5 hover:border-[var(--nc-border-strong)]`
- Any time dropdown: use a `<div>` with click toggle, positioned absolute below the button

Wiring:
- Best match click: `sortBy.value = 'relevance'` then `search()`
- Most fresh click: `sortBy.value = 'published_date'` then `search()`
- Time dropdown: sets `filters.from_date` to computed date, then `applyFilters()`

**Step 3: Add "Filters" dropdown button**

In the ordering row (right-aligned), add a "Filters" button that toggles a dropdown panel containing the existing FilterSidebar component. This replaces the desktop sidebar position.

**Step 4: Restructure the layout grid**

New layout:
```
┌─────────────┬──────────────────────────────────┐
│ Category    │ Ordering pills + Filter button   │
│ sidebar     │──────────────────────────────────│
│ (w-48)      │ FilterChips (if active)          │
│             │──────────────────────────────────│
│             │ Results / Skeleton / Empty        │
│             │──────────────────────────────────│
│             │ Pagination                        │
└─────────────┴──────────────────────────────────┘
```

- Desktop: `flex gap-6` with sidebar + main content
- Mobile: sidebar hidden, category tabs become horizontal scroll strip above ordering pills
- Move FilterSidebar from sidebar to dropdown panel
- Keep FilterDrawer for mobile filter access

**Step 5: Verify results page renders with new layout**

Run dev server, search for something, confirm:
- Category sidebar on left
- Ordering pills (Best match / Most fresh / Any time)
- Results display in main area
- Filters accessible via dropdown

**Step 6: Commit**

```bash
git add search-frontend/src/views/ResultsView.vue
git commit -m "feat(search-frontend): Bing-style results layout with category sidebar and ordering controls"
```

---

## Task 7: Result cards — Bing-style layout

**Files:**
- Modify: `search-frontend/src/components/search/SearchResultItem.vue`
- Modify: `search-frontend/src/components/search/SearchResults.vue`

**Step 1: Redesign SearchResultItem**

New card layout (Bing News style):
```
┌─────────────────────────────────────────────────┬──────────┐
│ [source badge] Source Name · 2d ago             │          │
│                                                 │ [thumb]  │
│ Bold Headline Text Here                         │ 120x90   │
│                                                 │          │
│ Two-line snippet with highlighted search terms  │          │
│ that shows the relevant excerpt from content... │          │
│                                                 │          │
│ [crime] [quality 85]                            │          │
└─────────────────────────────────────────────────┴──────────┘
```

Changes to SearchResultItem.vue:
- Remove `featured` prop and all featured-specific styling
- Card: `rounded-xl border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] p-5 hover:border-[var(--nc-border-strong)]` (border hover, no shadow)
- Layout: `flex gap-4` with content on left, thumbnail on right
- Line 1 (source + time): `text-sm text-[var(--nc-text-muted)]` with source name in `font-medium text-[var(--nc-text-secondary)]`
- Headline: `text-lg font-semibold text-[var(--nc-text)] group-hover:text-[var(--nc-primary)]` (blue on hover)
- Remove the green URL line (Bing doesn't show this prominently — fold into source badge)
- Snippet: `text-sm text-[var(--nc-text-secondary)] leading-relaxed line-clamp-2`
- Topic pills: `bg-[var(--nc-primary-muted)] text-[var(--nc-primary)]` (blue on dark)
- Quality badge: keep existing logic but update colors for dark
- Thumbnail: `w-[120px] h-[90px] rounded-lg object-cover` (slightly larger than current)

**Step 2: Update SearchResults**

- Change spacing from `space-y-5` to `space-y-4`
- Remove `featured` prop from first result (`:featured="index === 0"` → remove)

**Step 3: Verify result cards look correct**

Run dev server, search for something, confirm Bing-style card layout with source+time, headline, snippet, thumbnail right.

**Step 4: Commit**

```bash
git add search-frontend/src/components/search/SearchResultItem.vue search-frontend/src/components/search/SearchResults.vue
git commit -m "feat(search-frontend): Bing-style dark result cards"
```

---

## Task 8: Filter components — Dark restyle

**Files:**
- Modify: `search-frontend/src/components/search/FilterSidebar.vue`
- Modify: `search-frontend/src/components/search/FilterChips.vue`
- Modify: `search-frontend/src/components/search/FilterDrawer.vue`

**Step 1: Restyle FilterSidebar**

Most CSS vars auto-resolve to dark. Specific changes:
- Checkboxes: ensure they render well on dark (Tailwind checkbox styling)
- Select dropdown: `bg-[var(--nc-bg-surface)]` for dark select
- Range input: `accent-[var(--nc-primary)]` (keep — now blue)
- Date inputs: `bg-[var(--nc-bg-surface)]` for dark date pickers
- "Clear all" button: `text-[var(--nc-primary)]` (now blue)

**Step 2: Restyle FilterChips**

- Chip pills: `bg-[var(--nc-bg-surface)] text-[var(--nc-text)] border border-[var(--nc-border)]`
- Close button: `hover:bg-[var(--nc-bg-muted)]` (dark hover)
- Filters button: `bg-[var(--nc-bg-elevated)] border-[var(--nc-border)]`

**Step 3: Restyle FilterDrawer**

- Backdrop: `bg-black/60` (darker overlay for dark theme)
- Drawer panel: `bg-[var(--nc-bg-elevated)]` (auto-dark)
- Close button: keep existing var classes

**Step 4: Verify filters render on dark**

Run dev server, apply some filters, confirm chips and sidebar/drawer look correct.

**Step 5: Commit**

```bash
git add search-frontend/src/components/search/FilterSidebar.vue search-frontend/src/components/search/FilterChips.vue search-frontend/src/components/search/FilterDrawer.vue
git commit -m "feat(search-frontend): dark filter components"
```

---

## Task 9: Pagination and skeleton — Dark restyle

**Files:**
- Modify: `search-frontend/src/components/search/SearchPagination.vue`
- Modify: `search-frontend/src/components/search/SearchResultsSkeleton.vue`

**Step 1: Restyle SearchPagination**

- Active page: `bg-[var(--nc-primary)] text-white` (now blue)
- Inactive pages: `bg-[var(--nc-bg-elevated)] text-[var(--nc-text)]`
- Borders: `border-[var(--nc-border)]` (auto-dark)
- Container: `bg-[var(--nc-bg-elevated)]` (auto-dark)

**Step 2: Restyle SearchResultsSkeleton**

- Pulse colors: `bg-[var(--nc-bg-muted)]` for skeleton bars (auto-dark — #242429)
- Card bg: `bg-[var(--nc-bg-elevated)]` (auto-dark)
- Add a thumbnail placeholder on the right side to match new card layout

**Step 3: Verify**

Run dev server with loading state (throttle network), confirm dark skeleton and pagination.

**Step 4: Commit**

```bash
git add search-frontend/src/components/search/SearchPagination.vue search-frontend/src/components/search/SearchResultsSkeleton.vue
git commit -m "feat(search-frontend): dark pagination and skeleton"
```

---

## Task 10: Empty, error, and related — Dark restyle

**Files:**
- Modify: `search-frontend/src/components/search/EmptySearchState.vue`
- Modify: `search-frontend/src/components/search/RelatedContent.vue`
- Modify: `search-frontend/src/components/common/ErrorAlert.vue`
- Modify: `search-frontend/src/components/common/EmptyState.vue`
- Modify: `search-frontend/src/components/common/LoadingSpinner.vue`

**Step 1: Restyle EmptySearchState**

- Icon container: `bg-[var(--nc-bg-muted)]` (auto-dark)
- "Clear filters" button: `bg-[var(--nc-primary)]` instead of `bg-[var(--nc-accent)]`
- Topic suggestion pills: `bg-[var(--nc-bg-surface)]` with blue hover

**Step 2: Restyle RelatedContent**

- Topic buttons: `bg-[var(--nc-bg-surface)]` hover → `bg-[var(--nc-primary-muted)]`
- "More from source" link: `text-[var(--nc-primary)]` (auto-resolves to blue)

**Step 3: Restyle ErrorAlert**

- Keep error color scheme but ensure contrast on dark bg
- Border: `border-[var(--nc-error)]/30` (keep)
- "Try again" button: `bg-[var(--nc-primary)]` instead of `bg-[var(--nc-accent)]`

**Step 4: Restyle EmptyState**

- Replace hardcoded `text-gray-400`, `text-gray-900`, `text-gray-500` with CSS var equivalents:
  - `text-[var(--nc-text-muted)]`, `text-[var(--nc-text)]`, `text-[var(--nc-text-secondary)]`

**Step 5: Restyle LoadingSpinner**

- Change `border-blue-600` to `border-[var(--nc-primary)]`

**Step 6: Verify all states**

Run dev server, trigger empty/error states, confirm they render cleanly on dark.

**Step 7: Commit**

```bash
git add search-frontend/src/components/search/EmptySearchState.vue search-frontend/src/components/search/RelatedContent.vue search-frontend/src/components/common/ErrorAlert.vue search-frontend/src/components/common/EmptyState.vue search-frontend/src/components/common/LoadingSpinner.vue
git commit -m "feat(search-frontend): dark empty, error, and related components"
```

---

## Task 11: Minor pages — AdvancedSearch, NotFound

**Files:**
- Modify: `search-frontend/src/views/AdvancedSearchView.vue`
- Modify: `search-frontend/src/views/NotFoundView.vue`

**Step 1: Restyle AdvancedSearchView**

- Remove `font-display` from h1 (was Instrument Serif). Use `font-semibold text-3xl`
- Form card: vars auto-resolve to dark
- Submit button: `bg-[var(--nc-primary)]` instead of `bg-[var(--nc-accent)]`
- Input backgrounds: `bg-[var(--nc-bg-surface)]` for dark inputs

**Step 2: Restyle NotFoundView**

- Remove `font-display` from "404" text. Use `font-bold text-6xl sm:text-7xl`
- "Go back home" button: `bg-[var(--nc-primary)]` instead of `bg-[var(--nc-accent)]`

**Step 3: Verify both pages**

Navigate to `/advanced` and `/nonexistent`, confirm dark styling.

**Step 4: Commit**

```bash
git add search-frontend/src/views/AdvancedSearchView.vue search-frontend/src/views/NotFoundView.vue
git commit -m "feat(search-frontend): dark advanced search and 404 pages"
```

---

## Task 12: Visual QA and final polish

**Files:**
- Potentially any file that needs tweaking

**Step 1: Full visual walkthrough**

Test every view and state:
1. Home page — hero section, trending strip, channel grids, loading skeletons
2. Search results — ordering pills, category sidebar, result cards, pagination
3. Filters — chips, sidebar dropdown, drawer on mobile
4. Empty state — no results with suggestions
5. Advanced search — form renders correctly
6. 404 page
7. Mobile responsiveness (resize browser to 375px width)

**Step 2: Fix any visual issues found**

Common things to check:
- Text contrast ratios (light text on dark bg)
- Border visibility (not too subtle)
- Input field styling on dark (date pickers, selects)
- Focus ring visibility
- Thumbnail fallbacks

**Step 3: Build check**

Run: `cd search-frontend && npm run build`
Expected: Clean build with no errors.

Run: `cd search-frontend && npm run lint`
Expected: No lint errors.

**Step 4: Final commit**

```bash
git add -A search-frontend/src/
git commit -m "fix(search-frontend): visual QA polish for Obsidian Editorial theme"
```

---

## Task 13: Update CLAUDE.md if needed

**Files:**
- Potentially: `search-frontend/CLAUDE.md`

**Step 1: Review if any CLAUDE.md content is invalidated**

Check if the design system section or component descriptions need updating to reflect the new theme. The CLAUDE.md mostly describes architecture (unchanged) but references to "font-display" class usage may need a note.

**Step 2: Commit if changed**

```bash
git add search-frontend/CLAUDE.md
git commit -m "docs(search-frontend): update CLAUDE.md for Obsidian Editorial theme"
```
