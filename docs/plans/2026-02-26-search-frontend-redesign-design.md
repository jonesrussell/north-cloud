# Search Frontend Redesign — Obsidian Editorial

**Date**: 2026-02-26
**Status**: Approved
**Scope**: Visual overhaul of `search-frontend/` — all views and components

## Summary

Full redesign of the search-frontend from a light teal/amber theme with serif headings to a dark, professional "Obsidian Editorial" aesthetic with Bing News-inspired layout patterns. The goal is a premium, content-first news portal that feels fresh and enticing.

## Design Decisions

- **Aesthetic**: Dark sophistication — deep charcoal base, crisp off-white text, electric blue accent
- **Typography**: Source Sans 3 (sans-serif) replaces both Instrument Serif and DM Sans
- **Layout**: Full Bing News-inspired redesign — horizontal category tabs, ordering controls, vertical card results, category sidebar on results page
- **No new dependencies**: Font swap via Google Fonts, everything stays CSS/Tailwind

## Design Tokens

### Color Palette

| Token | Value | Usage |
|-------|-------|-------|
| `--nc-bg` | `#111114` | Page background |
| `--nc-bg-elevated` | `#1a1a1f` | Cards, panels, header |
| `--nc-bg-muted` | `#242429` | Hover states, skeleton fills |
| `--nc-bg-surface` | `#2a2a30` | Secondary surfaces, inputs |
| `--nc-border` | `#2e2e35` | Card/panel borders |
| `--nc-border-strong` | `#3e3e48` | Emphasized borders, hover |
| `--nc-text` | `#f0f0f3` | Primary text (headlines, body) |
| `--nc-text-secondary` | `#a0a0ab` | Snippets, metadata |
| `--nc-text-muted` | `#6b6b78` | Timestamps, placeholders |
| `--nc-primary` | `#3b82f6` | Links, active tabs, accent |
| `--nc-primary-hover` | `#60a5fa` | Hover states (lighter on dark) |
| `--nc-primary-muted` | `rgba(59,130,246,0.12)` | Active tab bg, selected states |
| `--nc-accent` | `#f59e0b` | Quality badges, channel pips |
| `--nc-accent-hover` | `#d97706` | Accent hover |
| `--nc-accent-muted` | `rgba(245,158,11,0.12)` | Accent backgrounds |
| `--nc-success` | `#22c55e` | Good quality, source badges |
| `--nc-success-muted` | `rgba(34,197,94,0.12)` | Success backgrounds |
| `--nc-warning` | `#f59e0b` | Medium quality |
| `--nc-warning-muted` | `rgba(245,158,11,0.12)` | Warning backgrounds |
| `--nc-error` | `#ef4444` | Errors, crime channel pip |
| `--nc-error-muted` | `rgba(239,68,68,0.12)` | Error backgrounds |
| `--nc-highlight-bg` | `rgba(59,130,246,0.2)` | Search term highlight background |
| `--nc-highlight-fg` | `#93c5fd` | Search term highlight text |

### Shadows

| Token | Value |
|-------|-------|
| `--nc-shadow-sm` | `0 1px 2px rgba(0,0,0,0.3)` |
| `--nc-shadow` | `0 4px 12px rgba(0,0,0,0.4)` |
| `--nc-shadow-lg` | `0 12px 28px rgba(0,0,0,0.5)` |

### Typography

- **Font family**: Source Sans 3 (variable, 400/500/600/700)
- `--font-display`: `"Source Sans 3", system-ui, sans-serif`
- `--font-body`: `"Source Sans 3", system-ui, sans-serif`
- Headlines: 600 weight, `tracking-tight`
- Body: 400 weight, `leading-relaxed`
- Labels/metadata: 500 weight, uppercase, `tracking-wider`

### Background Treatment

- Flat `--nc-bg` background
- Noise texture overlay at `opacity: 0.02`
- No radial gradient (removed — was teal-tinted)

## Layout — Header & Navigation

### Header (App.vue)

- Sticky top, `--nc-bg-elevated` surface, bottom border
- Left: "North Cloud" wordmark — Source Sans 3, 600 weight, no icon
- Center: Compact search input — rounded-full, `--nc-bg-surface`, subtle border, search icon left
- Search input focus: blue ring glow (`ring-[--nc-primary]/30`)

### Category Tabs (new)

- Horizontal tab bar below header, part of the same sticky container
- Tabs: All | Top Stories | Crime | Mining | Entertainment
- Active tab: blue text, 2px blue underline, subtle blue bg tint (`--nc-primary-muted`)
- Hover: text lightens, subtle bg shift
- Results page: tabs act as topic quick-filters
- Home page: tabs filter/scroll to channel sections

### Ordering Controls (ResultsView only)

- Row of pills below category tabs: **Best match** (filled blue) | **Most fresh** (outline) | **Any time** dropdown
- Any time dropdown options: Past hour, Past 24 hours, Past 7 days, Past 30 days
- Maps to existing sort/date filter params in `useSearch`

## Layout — Home Page

### Hero Section

- Top Stories: first article as large hero card (2/3 width, 16:9 image with gradient scrim overlay, headline on image)
- 2-3 smaller cards stacked in the remaining 1/3

### Trending Strip

- Horizontal scroll strip: "Trending on North Cloud"
- Pill-shaped topic tags, clickable (navigate to `/search?topics=X`)
- Uses facet data or static channel slugs

### Channel Sections

- Each topic channel renders as a row of 3-4 cards
- Channel header: topic name with colored pip, "See more" link
- Cards: dark elevated surface, 1px border, thumbnail top, source + time below image, bold headline
- No snippet on homepage cards (cleaner)
- Hover: border brightens, thumbnail scales

## Layout — Results Page

### Category Sidebar (left)

- Vertical list with icons (replaces FilterSidebar position)
- Categories: Top Stories, Crime, Mining, Entertainment
- Active category: blue highlight + filled icon
- Clickable topic filters

### Filter Controls

- Existing filter controls (sources, quality, dates) move into a "Filters" dropdown button in the ordering pill row
- Dropdown panel opens below the button (not a sidebar)

### Result Cards

- Full-width bordered panels, vertical stack:
  - Line 1: Source badge + source name + relative time
  - Line 2: Bold headline (600 weight, blue on hover)
  - Line 3-4: 2-line snippet with highlight marks
  - Right side: Thumbnail (~120x90px)
  - Bottom: Topic pills + quality badge
- All results equal weight (no "featured" first result)
- Spacing: `space-y-4` between cards

### Filter Chips

- Dark pills with blue text, shown above results when filters active

### Pagination

- Dark surface, blue active page, same structure

### Mobile

- Category sidebar collapses to horizontal scrollable tab strip above results
- Filter dropdown stays in ordering row

## Component Change Map

| Component | Change |
|-----------|--------|
| `style.css` | New dark palette tokens, Source Sans 3 font, dark noise overlay |
| `index.html` | Swap Google Fonts link to Source Sans 3 |
| `App.vue` | Dark header, add category tabs, restyle search input |
| `HomeView.vue` | Hero layout, trending strip, restyled channel grid |
| `ChannelSection.vue` | Dark restyle, no serif in header, darker skeleton |
| `NewsCard.vue` | Dark card, bright text, border hover, no snippet on home |
| `ResultsView.vue` | Category sidebar, ordering pills, filter dropdown, restructured layout |
| `SearchBar.vue` | Dark surface input, blue focus ring, dark dropdown |
| `SearchResultItem.vue` | Bing-style: source+time top, headline, snippet, thumbnail right, no featured variant |
| `FilterSidebar.vue` | Restyle as dropdown panel content on dark surface |
| `FilterChips.vue` | Dark pills, blue text |
| `SearchPagination.vue` | Dark surface, blue active page |
| `EmptySearchState.vue` | Dark restyle |
| `RelatedContent.vue` | Dark restyle |
| `SearchResultsSkeleton.vue` | Dark pulse colors |
| `ErrorAlert.vue` | Dark restyle |

## Micro-interactions

- **Card hover**: border `--nc-border` to `--nc-border-strong`, 200ms ease-out, no shadow lift
- **Link hover**: text to `--nc-primary-hover` (lighter blue)
- **Tab indicator**: blue underline slides via CSS transform, 250ms
- **Skeleton**: pulse between `--nc-bg-muted` and `--nc-bg-surface`
- **Search focus**: blue ring glow
- **Thumbnail hover**: `scale-105` on group hover (existing)
- **Page transitions**: none (instant route changes)

## Out of Scope

- Authentication / user accounts
- New API endpoints or backend changes
- Dark/light mode toggle (dark only for now)
- New npm dependencies
