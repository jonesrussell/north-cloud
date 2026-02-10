# FilterBar Pattern

All dashboard table FilterBars follow a consistent structure for predictable UX across Sources, Jobs, Reputation, Discovered Links, Delivery Logs, Articles, and Review Queue.

## Structure

### 1. Search Field

- Debounced text input (or immediate emit; composable may debounce)
- Placeholder describes scope: "Search by…", "Search links by URL…"
- Icon: `Search` (left-aligned in input)
- Emits `update:search` with the current value

### 2. Dropdown Filters

- Source, channel, status, category, etc. per domain
- First option: "All Sources", "All Channels", "All", etc.
- Width: `sm:w-48` or `sm:w-64` for long labels
- Emits `update:{filterKey}` with selected value

### 3. Pill Filters

- Toggleable pills for status, health, category, enabled state
- Active: `bg-primary text-primary-foreground`
- Inactive: `bg-muted text-muted-foreground hover:bg-muted/80`
- Optional badge with count per pill (e.g., JobsFilterBar)
- Emits via composable `setFilter` / `toggleStatusFilter`

### 4. Clear Button

- Visible when `hasActiveFilters` is true
- Label: "Clear ({{ activeFilterCount }})"
- Calls `clearFilters()` on the table composable
- `variant="outline"`, `size="sm"`

### 5. Layout

- **Row 1**: Search + dropdowns + Clear button (flex, wrap on small screens)
- **Row 2** (optional): Pills (flex-wrap)
- Spacing: `gap-3`, `space-y-3` between rows

## Props Interface

FilterBar components receive:

| Prop              | Type      | Description                                      |
|-------------------|-----------|--------------------------------------------------|
| `filters`         | `F`       | Current filter values from composable            |
| `hasActiveFilters`| `boolean` | Whether any filter is applied                    |
| `activeFilterCount` | `number` | Number of active filters (for Clear label)      |
| Domain-specific   | varies    | e.g. `sources`, `channels`, `categories`        |

## Events

| Event            | Payload              | Description                    |
|------------------|----------------------|--------------------------------|
| `update:search`  | `string`             | Search query                   |
| `update:{key}`   | `string \| undefined`| Single filter value            |
| `clear-filters`  | —                    | Reset all filters              |

## Composable Contract

Table composables expose:

- `filters` – `Ref<F>` with current filter values
- `hasActiveFilters` – `ComputedRef<boolean>`
- `activeFilterCount` – `ComputedRef<number>`
- `setFilter(key, value)` – set one filter
- `clearFilters()` – reset all filters

## Reference Implementations

- **JobsFilterBar** – search, source dropdown, status pills, clear
- **SourcesFilterBar** – search, enabled pills (All/Active/Inactive), clear
- **ReputationFilterBar** – search, category dropdown, clear
- **DiscoveredLinksFilterBar** – search, status dropdown, source dropdown, clear
- **DeliveryLogsFilterBar** – channel dropdown, clear
- **ArticlesFilterBar** – channel dropdown, clear, refresh, polling indicator

## Optional: FilterBarBase

If FilterBars share >80% structure, consider extracting a `FilterBarBase` with slots for search, dropdowns, and pills. Currently each FilterBar is standalone; the pattern is documented for consistency.
