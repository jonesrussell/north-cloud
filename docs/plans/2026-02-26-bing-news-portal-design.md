# Bing-Style News Portal Design

**Date**: 2026-02-26
**Status**: Approved

## Overview

Redesign the search-frontend homepage from a search-bar-centric layout into a visual, discovery-focused news portal inspired by Bing's news homepage (`bing.com/news`). Search moves to a compact header bar. The homepage becomes a browsable grid of news cards organized by publisher channel.

## Goals

- Replace the hero search bar homepage with a visual news portal
- Organize content by publisher channel (Crime, Mining, Entertainment)
- Surface `og:image` thumbnails to make cards visually rich
- Keep the existing search results page (`/search`) for actual search queries

## Page Architecture

**Two pages:**

1. **Homepage** (`/`) — Discovery portal with channel-based news card grids
2. **Search Results** (`/search?q=...`) — Existing faceted results page (preserved as-is, with thumbnail enhancement)

**Persistent header** across both pages:
- Left: North Cloud logo/brand
- Right: Compact pill-shaped search input
- Submitting search navigates to `/search?q=...`

## Homepage Layout

### Top Stories (Hero Section)
- Fetches from `/feed.json` (default feed, all channels, highest quality)
- 3-column grid on desktop, 2 on tablet, 1 on mobile
- 6 articles (2 rows of 3)
- Large image cards with `og:image` thumbnails

### Channel Sections
One section per active publisher channel:

| Channel | Feed Endpoint | Topics |
|---------|--------------|--------|
| Crime | `/feed/crime.json` | violent_crime, property_crime, drug_crime, organized_crime, criminal_justice |
| Mining | `/feed/mining.json` | mining |
| Entertainment | `/feed/entertainment.json` | entertainment |

Each section:
- Heading: channel name + "See more >" link → `/search?topics={topic}`
- 3-column grid of medium image cards
- 6 articles per section

### Responsive Breakpoints
- Desktop (>=1024px): 3-column grids
- Tablet (>=640px): 2-column grids
- Mobile (<640px): 1-column, full-width cards

## Card Design

```
+----------------------------+
|                            |
|   [og:image thumbnail]     |
|   (16:9 aspect ratio)      |
|                            |
+----------------------------+
| Source Name  · 2h ago      |
|                            |
| Bold Headline Text That    |
| Can Wrap To Two Lines      |
|                            |
| Short snippet in muted     |
| gray, one line truncated...|
+----------------------------+
```

**Image handling:**
- `og:image` displayed at 16:9 with `object-fit: cover`
- Fallback (no image): gradient background using channel accent color with topic name as faded text

**Typography and colors:**
- Existing North Cloud design tokens (teal `#0d5c63`, amber `#c17f2e`)
- DM Sans for body, Instrument Serif for section headings
- Headlines: bold, dark, 16-18px
- Source/time: muted gray, 13px, relative time format ("2h ago", "1d ago")
- Snippets: muted gray, 14px, single-line truncated
- Cards: white background, subtle shadow on hover, `rounded-lg`

## Backend Changes

### 1. Add `og_image` to `PublicFeedItem`

The feed endpoints already exist and work. Just add the image field to the response.

**Files:**
- `search/internal/domain/search.go` — add `OGImage string` to `PublicFeedItem`
- `search/internal/service/search_service.go` — include `og_image` in ES source fields, map to response

### 2. Add `og_image` to `SearchHit`

For the search results page to also show thumbnails.

**Files:**
- `search/internal/domain/search.go` — add `OGImage string` to `SearchHit`
- `search/internal/domain/content.go` — ensure `ClassifiedContent` has `OGImage`
- `search/internal/service/search_service.go` — map the field in result conversion

**No new endpoints, no database changes, no migrations.**

## Frontend Changes

### New Components
- `NewsCard.vue` — reusable image card (thumbnail, source, time, headline, snippet)
- `ChannelSection.vue` — section with heading + grid of NewsCards for a channel
- `TopStories.vue` — hero section using the default feed
- `HeaderSearchBar.vue` — compact pill-shaped search input for the persistent header

### Modified Components
- `App.vue` — replace current header with persistent header containing search bar
- `HomeView.vue` — complete redesign from search-bar hero to news portal sections

### Preserved Components
- `ResultsView.vue` — keep existing search results (add thumbnail to result items)
- `SearchResultItem.vue` — add `og:image` thumbnail display
- All filter, pagination, and composable code — unchanged

### Data Fetching
- Homepage sections fetch from existing public feed endpoints (`/feed.json`, `/feed/{slug}.json`)
- No auth required for feeds
- Each section independently loaded (parallel requests)

## Out of Scope
- Dark mode
- Category sidebar navigation (categories are scroll sections)
- Trending/analytics-based sections
- New backend API endpoints
- SSR/Nuxt migration
