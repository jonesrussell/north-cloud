# People & Band Offices Tables Design

**Issue:** #271
**Depends on:** #264 (communities table)
**Date:** 2026-03-10

## Summary

Add `people`, `people_history`, and `band_offices` tables to the source-manager database. These store structured leadership and contact data for communities.

## Decisions

- **Repository-level transactional archive** for leadership term changes — consistent with existing codebase, explicit, testable, no service layer or triggers needed.
- **Two migrations** — people + people_history together (structurally coupled), band_offices separate (independent concept).
- **No API routes in this issue** — data layer only (models, repositories, migrations). Handlers/routes are a follow-up.

## Migrations

### 010_create_people_tables.up.sql

Creates `people` and `people_history` tables.

**people:**
- FK to `communities(id)` ON DELETE CASCADE
- Unique constraint: `(community_id, name, role)`
- Indexes: `community_id`, `role`, partial index on `is_current = true`
- `updated_at` trigger (reuses existing `update_updated_at_column()`)

**people_history:**
- FKs to `people(id)` and `communities(id)` ON DELETE CASCADE
- Indexes: `community_id`, `person_id`

### 011_create_band_offices_table.up.sql

Creates `band_offices` table.

- `community_id` UNIQUE (one office per community), FK ON DELETE CASCADE
- `updated_at` trigger

Down migrations drop in reverse order.

## Go Models

### internal/models/person.go

```go
// Person represents a community leader or official.
type Person struct {
    // Required fields (value types)
    ID          string    // db:"id"           json:"id"
    CommunityID string    // db:"community_id" json:"community_id"
    Name        string    // db:"name"         json:"name"
    Slug        string    // db:"slug"         json:"slug"
    Role        string    // db:"role"         json:"role"
    DataSource  string    // db:"data_source"  json:"data_source"
    IsCurrent   bool      // db:"is_current"   json:"is_current"
    CreatedAt   time.Time // db:"created_at"   json:"created_at"
    UpdatedAt   time.Time // db:"updated_at"   json:"updated_at"

    // Optional fields (pointers, omitempty)
    RoleTitle  *string    // db:"role_title"  json:"role_title,omitempty"
    Email      *string    // db:"email"       json:"email,omitempty"
    Phone      *string    // db:"phone"       json:"phone,omitempty"
    TermStart  *time.Time // db:"term_start"  json:"term_start,omitempty"
    TermEnd    *time.Time // db:"term_end"    json:"term_end,omitempty"
    SourceURL  *string    // db:"source_url"  json:"source_url,omitempty"
    Verified   bool       // db:"verified"    json:"verified"
    VerifiedAt *time.Time // db:"verified_at" json:"verified_at,omitempty"
}

// PersonHistory is an archived snapshot of a person's term.
type PersonHistory struct {
    ID          string     `db:"id"           json:"id"`
    PersonID    string     `db:"person_id"    json:"person_id"`
    CommunityID string     `db:"community_id" json:"community_id"`
    Name        string     `db:"name"         json:"name"`
    Role        string     `db:"role"         json:"role"`
    TermStart   *time.Time `db:"term_start"   json:"term_start,omitempty"`
    TermEnd     *time.Time `db:"term_end"     json:"term_end,omitempty"`
    DataSource  *string    `db:"data_source"  json:"data_source,omitempty"`
    SourceURL   *string    `db:"source_url"   json:"source_url,omitempty"`
    ArchivedAt  time.Time  `db:"archived_at"  json:"archived_at"`
}

// PersonFilter controls listing/counting queries.
type PersonFilter struct {
    CommunityID string
    Role        string
    CurrentOnly bool
    Limit       int
    Offset      int
}
```

### internal/models/band_office.go

```go
// BandOffice represents the physical office for a community (1:1).
type BandOffice struct {
    // Required
    ID          string    // db:"id"           json:"id"
    CommunityID string    // db:"community_id" json:"community_id"
    DataSource  string    // db:"data_source"  json:"data_source"
    Verified    bool      // db:"verified"     json:"verified"
    CreatedAt   time.Time // db:"created_at"   json:"created_at"
    UpdatedAt   time.Time // db:"updated_at"   json:"updated_at"

    // Optional (all pointer + omitempty)
    AddressLine1 *string    // address fields
    AddressLine2 *string
    City         *string
    Province     *string
    PostalCode   *string
    Phone        *string
    Fax          *string
    Email        *string
    TollFree     *string
    OfficeHours  *string
    SourceURL    *string
    VerifiedAt   *time.Time
}
```

## Repositories

### internal/repository/person.go — PersonRepository

**Constructor:** `NewPersonRepository(db *sql.DB, log infralogger.Logger)`

**Helper:** `scanPerson()` — reusable row scanner.

**CRUD:**
- `Create(ctx, p *Person) error` — generates UUID
- `GetByID(ctx, id string) (*Person, error)` — nil,nil for not found
- `Update(ctx, p *Person) error` — checks RowsAffected
- `Delete(ctx, id string) error`

**Query:**
- `ListByCommunity(ctx, filter PersonFilter) ([]Person, error)` — returns error if `CommunityID` is empty; dynamic WHERE builder with optional role/current_only filters, pagination
- `Count(ctx, filter PersonFilter) (int, error)`

**Archive:**
- `ArchiveTerm(ctx, personID string) error` — transaction: SELECT person → INSERT into people_history → UPDATE person (is_current=false, term_end=NOW()). Returns error if personID not found. Transaction rolls back on any step failure.

### internal/repository/band_office.go — BandOfficeRepository

**Constructor:** `NewBandOfficeRepository(db *sql.DB, log infralogger.Logger)`

**Helper:** `scanBandOffice()` — reusable row scanner.

**CRUD:**
- `Create(ctx, bo *BandOffice) error`
- `GetByCommunity(ctx, communityID string) (*BandOffice, error)` — nil,nil for not found
- `Update(ctx, bo *BandOffice) error`
- `DeleteByCommunity(ctx, communityID string) error` — consistent with the 1:1 lookup pattern

**Upsert:**
- `Upsert(ctx, bo *BandOffice) error` — ON CONFLICT (community_id) DO UPDATE

## Bootstrap Wiring

Instantiate `PersonRepository` and `BandOfficeRepository` in `SetupHTTPServer` alongside existing repositories. No new routes — data layer only for this issue.

## Testing

Follow community test patterns:
- `setupPersonTestDB(t)` / `setupBandOfficeTestDB(t)` helpers with `t.Helper()`
- `newTestPerson(...)` / `newTestBandOffice(...)` factory helpers
- Test all CRUD methods, filter combinations, ArchiveTerm transaction, and Upsert conflict handling
- ArchiveTerm tests must verify the inserted `people_history` row has correct snapshot fields, not just that `is_current` was set to false
- Integration tests skip in short mode

## Acceptance Criteria

- [ ] Migration creates all 3 tables with indexes and FKs
- [ ] Go model structs with db/json tags
- [ ] Person repository: Create, Update, Delete, GetByID, ListByCommunity, Count, ArchiveTerm
- [ ] BandOffice repository: Create, Update, GetByCommunity, DeleteByCommunity, Upsert
- [ ] ArchiveTerm archives to people_history in a transaction
- [ ] Unit tests for all repository methods
