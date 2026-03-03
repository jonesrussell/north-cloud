# Social Publisher Frontend Design

## Overview

Add social publishing management pages to the existing North Cloud dashboard under the Distribution section. Three pages: content list with delivery tracking, accounts CRUD, and a publish form with scheduling support.

## Decisions

- **Location**: Inside existing dashboard (not a separate app)
- **Navigation**: Under the Distribution sidebar section
- **Architecture**: Feature module pattern (`features/social-publishing/`)
- **Backend**: social-publisher service on port 8078, all endpoints JWT-protected

## Pages & Navigation

Three new pages under Distribution:

| Route | View | Purpose |
|-------|------|---------|
| `/distribution/social-content` | `SocialContentView.vue` | Paginated content list with delivery summaries, status filters, retry actions |
| `/distribution/social-accounts` | `SocialAccountsView.vue` | List/create/edit/delete social media accounts |
| `/distribution/social-publish` | `SocialPublishView.vue` | Compose form: type, title, body, targets, schedule picker |

Sidebar additions to Distribution section:

```
Distribution
  ├── Channels        (existing)
  ├── Routes          (existing)
  ├── Delivery Logs   (existing)
  ├── Social Content  (new — FileText icon)
  ├── Social Accounts (new — Users icon)
  └── Publish         (new — Send icon, as quickAction)
```

## Data Types

New file: `dashboard/src/types/socialPublisher.ts`

```typescript
export interface SocialContent {
  id: string
  type: string
  title: string
  summary: string
  url: string
  project: string
  source: string
  published: boolean
  scheduled_at?: string
  created_at: string
  delivery_summary?: DeliverySummary
}

export interface DeliverySummary {
  total: number
  pending: number
  delivered: number
  failed: number
  retrying: number
}

export interface SocialAccount {
  id: string
  name: string
  platform: string
  project: string
  enabled: boolean
  credentials_configured: boolean
  token_expiry?: string
  created_at: string
  updated_at: string
}

export interface CreateAccountRequest {
  name: string
  platform: string
  project: string
  enabled?: boolean
  credentials?: Record<string, unknown>
  token_expiry?: string
}

export interface UpdateAccountRequest {
  name?: string
  platform?: string
  project?: string
  enabled?: boolean
  credentials?: Record<string, unknown>
  token_expiry?: string
}

export interface PublishRequest {
  type: string
  title?: string
  body?: string
  summary?: string
  url?: string
  images?: string[]
  tags?: string[]
  project?: string
  targets?: TargetConfig[]
  scheduled_at?: string
  metadata?: Record<string, string>
  source?: string
}

export interface TargetConfig {
  platform: string
  account: string
}

export interface ContentListResponse {
  items: SocialContent[]
  count: number
  total: number
  offset: number
  limit: number
}

export interface AccountsListResponse {
  items: SocialAccount[]
  count: number
}

export interface Delivery {
  id: string
  content_id: string
  platform: string
  account: string
  status: string
  attempts: number
  max_attempts: number
  error?: string
  platform_id?: string
  platform_url?: string
  delivered_at?: string
  created_at: string
}
```

## API Client

New axios instance in `dashboard/src/api/client.ts` pointing at `/api/social-publisher` (proxy to port 8078):

```typescript
socialPublisherApi.content.list(params)    // GET  /content?limit=&offset=&status=&type=
socialPublisherApi.content.status(id)      // GET  /status/:id
socialPublisherApi.content.publish(data)   // POST /publish
socialPublisherApi.content.retry(id)       // POST /retry/:id
socialPublisherApi.accounts.list()         // GET  /accounts
socialPublisherApi.accounts.get(id)        // GET  /accounts/:id
socialPublisherApi.accounts.create(data)   // POST /accounts
socialPublisherApi.accounts.update(id, d)  // PUT  /accounts/:id
socialPublisherApi.accounts.delete(id)     // DELETE /accounts/:id
```

Vite dev proxy addition:

```typescript
'/api/social-publisher': {
  target: process.env.SOCIAL_PUBLISHER_API_URL || 'http://localhost:8078',
  changeOrigin: true,
  rewrite: (path) => path.replace(/^\/api\/social-publisher/, '/api/v1'),
  // + auth header forwarding (same pattern as publisher proxy)
}
```

## File Structure

```
dashboard/src/
├── features/social-publishing/
│   ├── composables/
│   │   ├── useContentTable.ts       # useServerPaginatedTable for content
│   │   └── useAccountsTable.ts      # useServerPaginatedTable for accounts
│   └── index.ts                     # barrel export
├── components/domain/social-publishing/
│   ├── ContentTable.vue             # Table: type badge, title, source, delivery badges, retry
│   ├── ContentFilterBar.vue         # Status dropdown + type dropdown
│   ├── DeliverySummaryBadges.vue    # Inline colored count badges
│   ├── AccountsTable.vue            # Table: name, platform, project, enabled, creds status
│   ├── AccountFormDialog.vue        # Modal: create/edit account form
│   └── PublishForm.vue              # Form: type, title, body, targets, schedule
├── views/distribution/
│   ├── SocialContentView.vue        # Content list page
│   ├── SocialAccountsView.vue       # Accounts CRUD page
│   └── SocialPublishView.vue        # Publish form page
└── types/
    └── socialPublisher.ts           # All types above
```

## Page Behaviors

### Social Content

- `useContentTable` composable wraps `useServerPaginatedTable` with content list API
- Filter bar: status dropdown (all/delivered/failed/pending) and type dropdown
- Table columns: Type (badge), Title, Source, Created, Delivery Summary (inline badges), Actions
- Delivery summary: colored badges — green "N delivered", red "N failed", yellow "N pending"
- Failed deliveries show retry button — calls `POST /retry/:deliveryId` then refetches
- Row click expands inline panel showing full delivery list from `GET /status/:id`

### Social Accounts

- `useAccountsTable` composable wraps `useServerPaginatedTable` with accounts list API
- Table columns: Name, Platform (badge), Project, Enabled (toggle), Credentials (check/x), Token Expiry, Actions
- "Add Account" button opens `AccountFormDialog` in create mode
- Edit button opens same dialog pre-populated
- Platform: `<Select>` with known platforms (x, facebook, etc.)
- Credentials: JSON textarea, only shown in create/edit dialog, never in table
- Delete: `confirm()` dialog, same pattern as sources

### Publish Form

- Fields: Type (select), Title (input), Body (textarea), Summary (input), URL (input), Tags (comma-separated), Project (input)
- Target accounts: checkboxes from `accounts.list()`, filtered to enabled, showing name + platform badge
- Schedule: toggle "Publish now" / "Schedule for later" with datetime picker for `scheduled_at`
- Submit: `useMutation` wrapping `POST /publish`, loading state on button, toast on success, navigate to content list

### Error Handling (all pages)

- Loading: `Loader2` spinner
- Error: destructive Card with message
- Empty: Card with icon + descriptive message
- Mutations: toast notifications via Sonner
