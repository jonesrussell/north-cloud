# People & Band Offices Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add people, people_history, and band_offices tables to source-manager with Go models, repositories, and tests.

**Architecture:** Two migrations (people+history together, band_offices separate). Repository-level transactional archive for leadership term changes. No API routes — data layer only.

**Tech Stack:** Go 1.26+, PostgreSQL, database/sql, github.com/google/uuid, testify

**Spec:** `docs/superpowers/specs/2026-03-10-people-band-offices-design.md`

---

## File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `source-manager/migrations/010_create_people_tables.up.sql` | people + people_history DDL |
| Create | `source-manager/migrations/010_create_people_tables.down.sql` | Drop people tables |
| Create | `source-manager/migrations/011_create_band_offices_table.up.sql` | band_offices DDL |
| Create | `source-manager/migrations/011_create_band_offices_table.down.sql` | Drop band_offices table |
| Create | `source-manager/internal/models/person.go` | Person, PersonHistory, PersonFilter structs |
| Create | `source-manager/internal/models/band_office.go` | BandOffice struct |
| Create | `source-manager/internal/repository/person.go` | PersonRepository CRUD + ArchiveTerm |
| Create | `source-manager/internal/repository/person_test.go` | Person repository integration tests |
| Create | `source-manager/internal/repository/band_office.go` | BandOfficeRepository CRUD + Upsert |
| Create | `source-manager/internal/repository/band_office_test.go` | BandOffice repository integration tests |
| Modify | `source-manager/internal/bootstrap/server.go` | Wire new repositories |

---

## Chunk 1: Migrations

### Task 1: Create people + people_history migration

**Files:**
- Create: `source-manager/migrations/010_create_people_tables.up.sql`
- Create: `source-manager/migrations/010_create_people_tables.down.sql`

- [ ] **Step 1: Write the up migration**

Create `source-manager/migrations/010_create_people_tables.up.sql`:

```sql
CREATE TABLE people (
    id              VARCHAR(36)  PRIMARY KEY,
    community_id    VARCHAR(36)  NOT NULL REFERENCES communities(id) ON DELETE CASCADE,

    -- Identity
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) NOT NULL,
    role            VARCHAR(100) NOT NULL,
    role_title      VARCHAR(255),

    -- Contact
    email           TEXT,
    phone           VARCHAR(50),

    -- Term
    term_start      DATE,
    term_end        DATE,
    is_current      BOOLEAN      NOT NULL DEFAULT true,

    -- Provenance
    data_source     VARCHAR(50)  NOT NULL DEFAULT 'manual',
    source_url      TEXT,
    verified        BOOLEAN      NOT NULL DEFAULT false,
    verified_at     TIMESTAMP,

    -- Lifecycle
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_people_community_name_role UNIQUE (community_id, name, role)
);

CREATE INDEX idx_people_community ON people(community_id);
CREATE INDEX idx_people_role ON people(role);
CREATE INDEX idx_people_current ON people(is_current) WHERE is_current = true;

CREATE TRIGGER set_people_updated_at
    BEFORE UPDATE ON people
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE people_history (
    id              VARCHAR(36)  PRIMARY KEY,
    person_id       VARCHAR(36)  NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    community_id    VARCHAR(36)  NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    role            VARCHAR(100) NOT NULL,
    term_start      DATE,
    term_end        DATE,
    data_source     VARCHAR(50),
    source_url      TEXT,
    archived_at     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_people_history_community ON people_history(community_id);
CREATE INDEX idx_people_history_person ON people_history(person_id);
```

- [ ] **Step 2: Write the down migration**

Create `source-manager/migrations/010_create_people_tables.down.sql`:

```sql
DROP TABLE IF EXISTS people_history;
DROP TRIGGER IF EXISTS set_people_updated_at ON people;
DROP TABLE IF EXISTS people;
```

- [ ] **Step 3: Commit**

```bash
git add source-manager/migrations/010_create_people_tables.up.sql source-manager/migrations/010_create_people_tables.down.sql
git commit -m "feat(source-manager): add people + people_history migration (#271)"
```

### Task 2: Create band_offices migration

**Files:**
- Create: `source-manager/migrations/011_create_band_offices_table.up.sql`
- Create: `source-manager/migrations/011_create_band_offices_table.down.sql`

- [ ] **Step 1: Write the up migration**

Create `source-manager/migrations/011_create_band_offices_table.up.sql`:

```sql
CREATE TABLE band_offices (
    id              VARCHAR(36)  PRIMARY KEY,
    community_id    VARCHAR(36)  UNIQUE NOT NULL REFERENCES communities(id) ON DELETE CASCADE,

    -- Address
    address_line1   VARCHAR(255),
    address_line2   VARCHAR(255),
    city            VARCHAR(100),
    province        VARCHAR(5),
    postal_code     VARCHAR(10),

    -- Contact
    phone           VARCHAR(50),
    fax             VARCHAR(50),
    email           TEXT,
    toll_free       VARCHAR(50),

    -- Hours
    office_hours    TEXT,

    -- Provenance
    data_source     VARCHAR(50)  NOT NULL DEFAULT 'manual',
    source_url      TEXT,
    verified        BOOLEAN      NOT NULL DEFAULT false,
    verified_at     TIMESTAMP,

    -- Lifecycle
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER set_band_offices_updated_at
    BEFORE UPDATE ON band_offices
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

- [ ] **Step 2: Write the down migration**

Create `source-manager/migrations/011_create_band_offices_table.down.sql`:

```sql
DROP TRIGGER IF EXISTS set_band_offices_updated_at ON band_offices;
DROP TABLE IF EXISTS band_offices;
```

- [ ] **Step 3: Commit**

```bash
git add source-manager/migrations/011_create_band_offices_table.up.sql source-manager/migrations/011_create_band_offices_table.down.sql
git commit -m "feat(source-manager): add band_offices migration (#271)"
```

---

## Chunk 2: Go Models

### Task 3: Create Person and PersonHistory models

**Files:**
- Create: `source-manager/internal/models/person.go`
- Reference: `source-manager/internal/models/community.go` (follow struct tag patterns)

- [ ] **Step 1: Write the Person model**

Create `source-manager/internal/models/person.go`:

```go
package models

import "time"

// Person represents a community leader or official.
type Person struct {
	ID          string    `db:"id"           json:"id"`
	CommunityID string   `db:"community_id" json:"community_id"`
	Name        string    `db:"name"         json:"name"`
	Slug        string    `db:"slug"         json:"slug"`
	Role        string    `db:"role"         json:"role"`
	DataSource  string    `db:"data_source"  json:"data_source"`
	IsCurrent   bool      `db:"is_current"   json:"is_current"`
	Verified    bool      `db:"verified"     json:"verified"`
	CreatedAt   time.Time `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"   json:"updated_at"`

	// Optional fields
	RoleTitle  *string    `db:"role_title"  json:"role_title,omitempty"`
	Email      *string    `db:"email"       json:"email,omitempty"`
	Phone      *string    `db:"phone"       json:"phone,omitempty"`
	TermStart  *time.Time `db:"term_start"  json:"term_start,omitempty"`
	TermEnd    *time.Time `db:"term_end"    json:"term_end,omitempty"`
	SourceURL  *string    `db:"source_url"  json:"source_url,omitempty"`
	VerifiedAt *time.Time `db:"verified_at" json:"verified_at,omitempty"`
}

// PersonHistory is an archived snapshot of a person's term.
type PersonHistory struct {
	ID          string     `db:"id"           json:"id"`
	PersonID    string     `db:"person_id"    json:"person_id"`
	CommunityID string    `db:"community_id" json:"community_id"`
	Name        string     `db:"name"         json:"name"`
	Role        string     `db:"role"         json:"role"`
	TermStart   *time.Time `db:"term_start"   json:"term_start,omitempty"`
	TermEnd     *time.Time `db:"term_end"     json:"term_end,omitempty"`
	DataSource  *string    `db:"data_source"  json:"data_source,omitempty"`
	SourceURL   *string    `db:"source_url"   json:"source_url,omitempty"`
	ArchivedAt  time.Time  `db:"archived_at"  json:"archived_at"`
}

// PersonFilter controls listing/counting queries for people.
type PersonFilter struct {
	CommunityID string
	Role        string
	CurrentOnly bool
	Limit       int
	Offset      int
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd source-manager && go build ./internal/models/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add source-manager/internal/models/person.go
git commit -m "feat(source-manager): add Person and PersonHistory models (#271)"
```

### Task 4: Create BandOffice model

**Files:**
- Create: `source-manager/internal/models/band_office.go`
- Reference: `source-manager/internal/models/community.go`

- [ ] **Step 1: Write the BandOffice model**

Create `source-manager/internal/models/band_office.go`:

```go
package models

import "time"

// BandOffice represents the physical office for a community (1:1 relationship).
type BandOffice struct {
	ID          string    `db:"id"           json:"id"`
	CommunityID string   `db:"community_id" json:"community_id"`
	DataSource  string    `db:"data_source"  json:"data_source"`
	Verified    bool      `db:"verified"     json:"verified"`
	CreatedAt   time.Time `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"   json:"updated_at"`

	// Address
	AddressLine1 *string `db:"address_line1" json:"address_line1,omitempty"`
	AddressLine2 *string `db:"address_line2" json:"address_line2,omitempty"`
	City         *string `db:"city"          json:"city,omitempty"`
	Province     *string `db:"province"      json:"province,omitempty"`
	PostalCode   *string `db:"postal_code"   json:"postal_code,omitempty"`

	// Contact
	Phone    *string `db:"phone"     json:"phone,omitempty"`
	Fax      *string `db:"fax"       json:"fax,omitempty"`
	Email    *string `db:"email"     json:"email,omitempty"`
	TollFree *string `db:"toll_free" json:"toll_free,omitempty"`

	// Hours
	OfficeHours *string `db:"office_hours" json:"office_hours,omitempty"`

	// Provenance
	SourceURL  *string    `db:"source_url"  json:"source_url,omitempty"`
	VerifiedAt *time.Time `db:"verified_at" json:"verified_at,omitempty"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd source-manager && go build ./internal/models/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add source-manager/internal/models/band_office.go
git commit -m "feat(source-manager): add BandOffice model (#271)"
```

---

## Chunk 3: Person Repository + Tests

### Task 5: Create PersonRepository with CRUD methods

**Files:**
- Create: `source-manager/internal/repository/person.go`
- Reference: `source-manager/internal/repository/community.go` (follow all patterns)

- [ ] **Step 1: Write the PersonRepository**

Create `source-manager/internal/repository/person.go`:

```go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

const (
	defaultPersonLimit = 50
	maxPersonLimit     = 200
	personColumnCount  = 17
)

// personColumns is the SELECT column list for the people table.
const personColumns = `id, community_id, name, slug, role, data_source, is_current, verified,
	created_at, updated_at, role_title, email, phone, term_start, term_end, source_url, verified_at`

// PersonRepository provides CRUD operations for the people table.
type PersonRepository struct {
	db     *sql.DB
	logger infralogger.Logger
}

// NewPersonRepository creates a new PersonRepository.
func NewPersonRepository(db *sql.DB, log infralogger.Logger) *PersonRepository {
	return &PersonRepository{
		db:     db,
		logger: log,
	}
}

// scanPerson scans a single row into a Person struct.
func scanPerson(row interface{ Scan(...any) error }) (*models.Person, error) {
	var p models.Person
	scanErr := row.Scan(
		&p.ID, &p.CommunityID, &p.Name, &p.Slug, &p.Role, &p.DataSource, &p.IsCurrent, &p.Verified,
		&p.CreatedAt, &p.UpdatedAt, &p.RoleTitle, &p.Email, &p.Phone, &p.TermStart, &p.TermEnd,
		&p.SourceURL, &p.VerifiedAt,
	)
	if scanErr != nil {
		return nil, fmt.Errorf("scan person: %w", scanErr)
	}
	return &p, nil
}

// Create inserts a new person. ID and timestamps are set automatically.
func (r *PersonRepository) Create(ctx context.Context, p *models.Person) error {
	p.ID = uuid.New().String()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()

	query := `
		INSERT INTO people (
			id, community_id, name, slug, role, data_source, is_current, verified,
			created_at, updated_at, role_title, email, phone, term_start, term_end,
			source_url, verified_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15,
			$16, $17
		)`

	_, err := r.db.ExecContext(ctx, query,
		p.ID, p.CommunityID, p.Name, p.Slug, p.Role, p.DataSource, p.IsCurrent, p.Verified,
		p.CreatedAt, p.UpdatedAt, p.RoleTitle, p.Email, p.Phone, p.TermStart, p.TermEnd,
		p.SourceURL, p.VerifiedAt,
	)
	if err != nil {
		return fmt.Errorf("create person: %w", err)
	}

	return nil
}

// GetByID returns a person by ID, or nil if not found.
func (r *PersonRepository) GetByID(ctx context.Context, id string) (*models.Person, error) {
	query := `SELECT ` + personColumns + ` FROM people WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	p, err := scanPerson(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil //nolint:nilnil // nil,nil = "not found" per interface contract
		}
		return nil, fmt.Errorf("get person by id: %w", err)
	}

	return p, nil
}

// Update modifies an existing person by ID.
func (r *PersonRepository) Update(ctx context.Context, p *models.Person) error {
	p.UpdatedAt = time.Now()

	query := `
		UPDATE people SET
			community_id = $2, name = $3, slug = $4, role = $5, data_source = $6,
			is_current = $7, verified = $8, updated_at = $9, role_title = $10,
			email = $11, phone = $12, term_start = $13, term_end = $14,
			source_url = $15, verified_at = $16
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		p.ID, p.CommunityID, p.Name, p.Slug, p.Role, p.DataSource,
		p.IsCurrent, p.Verified, p.UpdatedAt, p.RoleTitle,
		p.Email, p.Phone, p.TermStart, p.TermEnd,
		p.SourceURL, p.VerifiedAt,
	)
	if err != nil {
		return fmt.Errorf("update person: %w", err)
	}

	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("update person rows affected: %w", rowsErr)
	}
	if rows == 0 {
		return errors.New("update person: not found")
	}

	return nil
}

// Delete removes a person by ID.
func (r *PersonRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM people WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete person: %w", err)
	}

	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("delete person rows affected: %w", rowsErr)
	}
	if rows == 0 {
		return errors.New("delete person: not found")
	}

	return nil
}

// buildPersonWhere constructs a WHERE clause from a PersonFilter.
// CommunityID is always required (added by caller).
func buildPersonWhere(filter models.PersonFilter) (where string, args []any) {
	var conditions []string
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("community_id = $%d", argIdx))
	args = append(args, filter.CommunityID)
	argIdx++

	if filter.Role != "" {
		conditions = append(conditions, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, filter.Role)
		argIdx++
	}

	if filter.CurrentOnly {
		conditions = append(conditions, "is_current = true")
	}

	return " WHERE " + strings.Join(conditions, " AND "), args
}

// ListByCommunity returns people for a community with optional filters.
// Returns error if CommunityID is empty.
func (r *PersonRepository) ListByCommunity(
	ctx context.Context, filter models.PersonFilter,
) ([]models.Person, error) {
	if filter.CommunityID == "" {
		return nil, errors.New("list people: community_id is required")
	}

	where, args := buildPersonWhere(filter)
	argIdx := len(args) + 1

	limit := filter.Limit
	if limit <= 0 || limit > maxPersonLimit {
		limit = defaultPersonLimit
	}

	//nolint:gosec // G201: query uses only constant column names and integer placeholders
	query := fmt.Sprintf(`SELECT `+personColumns+`
		FROM people%s ORDER BY name ASC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}
	defer rows.Close()

	people := make([]models.Person, 0, limit)
	for rows.Next() {
		p, scanErr := scanPerson(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		people = append(people, *p)
	}

	if closeErr := rows.Err(); closeErr != nil {
		return nil, fmt.Errorf("list people rows: %w", closeErr)
	}

	return people, nil
}

// Count returns the number of people matching the filter.
// Returns error if CommunityID is empty.
func (r *PersonRepository) Count(ctx context.Context, filter models.PersonFilter) (int, error) {
	if filter.CommunityID == "" {
		return 0, errors.New("count people: community_id is required")
	}

	where, args := buildPersonWhere(filter)
	query := "SELECT COUNT(*) FROM people" + where

	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count people: %w", err)
	}

	return count, nil
}

// ArchiveTerm archives a person's current term to people_history and marks them
// as no longer current. Runs in a transaction; rolls back on any failure.
// Returns error if personID is not found.
func (r *PersonRepository) ArchiveTerm(ctx context.Context, personID string) error {
	tx, txErr := r.db.BeginTx(ctx, nil)
	if txErr != nil {
		return fmt.Errorf("archive term begin tx: %w", txErr)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	// Step 1: SELECT current person
	query := `SELECT ` + personColumns + ` FROM people WHERE id = $1`
	row := tx.QueryRowContext(ctx, query, personID)

	p, scanErr := scanPerson(row)
	if scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return errors.New("archive term: person not found")
		}
		return fmt.Errorf("archive term select: %w", scanErr)
	}

	// Step 2: INSERT snapshot into people_history
	historyID := uuid.New().String()
	insertQuery := `
		INSERT INTO people_history (
			id, person_id, community_id, name, role,
			term_start, term_end, data_source, source_url, archived_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	now := time.Now()
	_, insertErr := tx.ExecContext(ctx, insertQuery,
		historyID, p.ID, p.CommunityID, p.Name, p.Role,
		p.TermStart, p.TermEnd, &p.DataSource, p.SourceURL, now,
	)
	if insertErr != nil {
		return fmt.Errorf("archive term insert history: %w", insertErr)
	}

	// Step 3: UPDATE person — mark as not current, set term_end
	updateQuery := `UPDATE people SET is_current = false, term_end = $2, updated_at = $3 WHERE id = $1`
	_, updateErr := tx.ExecContext(ctx, updateQuery, personID, now, now)
	if updateErr != nil {
		return fmt.Errorf("archive term update person: %w", updateErr)
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("archive term commit: %w", commitErr)
	}

	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd source-manager && go build ./internal/repository/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add source-manager/internal/repository/person.go
git commit -m "feat(source-manager): add PersonRepository with CRUD and ArchiveTerm (#271)"
```

### Task 6: Write PersonRepository tests

**Files:**
- Create: `source-manager/internal/repository/person_test.go`
- Reference: `source-manager/internal/repository/community_test.go` (follow all patterns)

- [ ] **Step 1: Write test helpers and CRUD tests**

Create `source-manager/internal/repository/person_test.go`:

```go
package repository_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/jonesrussell/north-cloud/source-manager/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPersonTestDB(t *testing.T) (
	personRepo *repository.PersonRepository,
	communityRepo *repository.CommunityRepository,
	cleanup func(),
) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db, dbCleanup := setupTestDB(t)
	logger := testhelpers.NewTestLogger()
	personRepo = repository.NewPersonRepository(db, logger)
	communityRepo = repository.NewCommunityRepository(db, logger)

	cleanup = func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, "TRUNCATE TABLE people_history, people, communities CASCADE")
		dbCleanup()
	}

	return personRepo, communityRepo, cleanup
}

// createTestCommunity creates a community for FK references in person tests.
func createTestCommunity(t *testing.T, ctx context.Context, repo *repository.CommunityRepository) *models.Community {
	t.Helper()
	c := newTestCommunity("Test Community", "test-community", "first_nation")
	require.NoError(t, repo.Create(ctx, c))
	return c
}

func newTestPerson(communityID, name, slug, role string) *models.Person {
	return &models.Person{
		CommunityID: communityID,
		Name:        name,
		Slug:        slug,
		Role:        role,
		DataSource:  "manual",
		IsCurrent:   true,
	}
}

func TestPersonRepository_Create(t *testing.T) {
	personRepo, communityRepo, cleanup := setupPersonTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	t.Run("create valid person", func(t *testing.T) {
		p := newTestPerson(community.ID, "John Smith", "john-smith", "chief")
		roleTitle := "Chief"
		p.RoleTitle = &roleTitle

		err := personRepo.Create(ctx, p)
		require.NoError(t, err)
		assert.NotEmpty(t, p.ID)
		assert.False(t, p.CreatedAt.IsZero())
	})

	t.Run("duplicate community+name+role returns error", func(t *testing.T) {
		p := newTestPerson(community.ID, "John Smith", "john-smith-2", "chief")
		err := personRepo.Create(ctx, p)
		assert.Error(t, err)
	})
}

func TestPersonRepository_GetByID(t *testing.T) {
	personRepo, communityRepo, cleanup := setupPersonTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	t.Run("found", func(t *testing.T) {
		p := newTestPerson(community.ID, "Get Test", "get-test", "councillor")
		require.NoError(t, personRepo.Create(ctx, p))

		found, err := personRepo.GetByID(ctx, p.ID)
		require.NoError(t, err)
		assert.Equal(t, p.ID, found.ID)
		assert.Equal(t, "Get Test", found.Name)
		assert.Equal(t, "councillor", found.Role)
	})

	t.Run("not found", func(t *testing.T) {
		found, err := personRepo.GetByID(ctx, "nonexistent-id")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestPersonRepository_Update(t *testing.T) {
	personRepo, communityRepo, cleanup := setupPersonTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)
	p := newTestPerson(community.ID, "Before Update", "before-update", "councillor")
	require.NoError(t, personRepo.Create(ctx, p))

	p.Name = "After Update"
	email := "updated@example.com"
	p.Email = &email

	err := personRepo.Update(ctx, p)
	require.NoError(t, err)

	found, getErr := personRepo.GetByID(ctx, p.ID)
	require.NoError(t, getErr)
	assert.Equal(t, "After Update", found.Name)
	assert.Equal(t, "updated@example.com", *found.Email)
}

func TestPersonRepository_Delete(t *testing.T) {
	personRepo, communityRepo, cleanup := setupPersonTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)
	p := newTestPerson(community.ID, "Delete Me", "delete-me", "clerk")
	require.NoError(t, personRepo.Create(ctx, p))

	err := personRepo.Delete(ctx, p.ID)
	require.NoError(t, err)

	found, getErr := personRepo.GetByID(ctx, p.ID)
	require.NoError(t, getErr)
	assert.Nil(t, found)
}

func TestPersonRepository_ListByCommunity(t *testing.T) {
	personRepo, communityRepo, cleanup := setupPersonTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	// Seed: one chief (current), two councillors (one current, one not)
	chief := newTestPerson(community.ID, "Chief One", "chief-one", "chief")
	require.NoError(t, personRepo.Create(ctx, chief))

	councillor1 := newTestPerson(community.ID, "Councillor A", "councillor-a", "councillor")
	require.NoError(t, personRepo.Create(ctx, councillor1))

	councillor2 := newTestPerson(community.ID, "Councillor B", "councillor-b", "councillor")
	councillor2.IsCurrent = false
	require.NoError(t, personRepo.Create(ctx, councillor2))

	t.Run("all people for community", func(t *testing.T) {
		results, err := personRepo.ListByCommunity(ctx, models.PersonFilter{
			CommunityID: community.ID,
		})
		require.NoError(t, err)
		assert.Len(t, results, 3)
	})

	t.Run("filter by role", func(t *testing.T) {
		results, err := personRepo.ListByCommunity(ctx, models.PersonFilter{
			CommunityID: community.ID,
			Role:        "councillor",
		})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("current only", func(t *testing.T) {
		results, err := personRepo.ListByCommunity(ctx, models.PersonFilter{
			CommunityID: community.ID,
			CurrentOnly: true,
		})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("error when community_id empty", func(t *testing.T) {
		_, err := personRepo.ListByCommunity(ctx, models.PersonFilter{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "community_id is required")
	})

	t.Run("pagination", func(t *testing.T) {
		results, err := personRepo.ListByCommunity(ctx, models.PersonFilter{
			CommunityID: community.ID,
			Limit:       1,
			Offset:      0,
		})
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})
}

func TestPersonRepository_Count(t *testing.T) {
	personRepo, communityRepo, cleanup := setupPersonTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	require.NoError(t, personRepo.Create(ctx, newTestPerson(community.ID, "P1", "p1", "chief")))
	require.NoError(t, personRepo.Create(ctx, newTestPerson(community.ID, "P2", "p2", "councillor")))

	t.Run("count all", func(t *testing.T) {
		count, err := personRepo.Count(ctx, models.PersonFilter{CommunityID: community.ID})
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("count by role", func(t *testing.T) {
		count, err := personRepo.Count(ctx, models.PersonFilter{CommunityID: community.ID, Role: "chief"})
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("error when community_id empty", func(t *testing.T) {
		_, err := personRepo.Count(ctx, models.PersonFilter{})
		assert.Error(t, err)
	})
}

func TestPersonRepository_ArchiveTerm(t *testing.T) {
	personRepo, communityRepo, cleanup := setupPersonTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	t.Run("archives current person to history", func(t *testing.T) {
		p := newTestPerson(community.ID, "Old Chief", "old-chief", "chief")
		require.NoError(t, personRepo.Create(ctx, p))
		require.True(t, p.IsCurrent)

		err := personRepo.ArchiveTerm(ctx, p.ID)
		require.NoError(t, err)

		// Verify person is no longer current
		updated, getErr := personRepo.GetByID(ctx, p.ID)
		require.NoError(t, getErr)
		assert.False(t, updated.IsCurrent)
		assert.NotNil(t, updated.TermEnd)

		// Verify history row was created with correct snapshot fields
		var history models.PersonHistory
		historyQuery := `SELECT id, person_id, community_id, name, role, term_start, term_end,
			data_source, source_url, archived_at FROM people_history WHERE person_id = $1`
		historyRow := personRepo.DB().QueryRowContext(ctx, historyQuery, p.ID)
		scanErr := historyRow.Scan(
			&history.ID, &history.PersonID, &history.CommunityID, &history.Name, &history.Role,
			&history.TermStart, &history.TermEnd, &history.DataSource, &history.SourceURL,
			&history.ArchivedAt,
		)
		require.NoError(t, scanErr)
		assert.Equal(t, p.ID, history.PersonID)
		assert.Equal(t, community.ID, history.CommunityID)
		assert.Equal(t, "Old Chief", history.Name)
		assert.Equal(t, "chief", history.Role)
		assert.False(t, history.ArchivedAt.IsZero())
	})

	t.Run("error when person not found", func(t *testing.T) {
		err := personRepo.ArchiveTerm(ctx, "nonexistent-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}
```

**Note:** The test uses `personRepo.DB()` to directly query `people_history`. Add this accessor method to `PersonRepository`:

```go
// DB returns the underlying *sql.DB for test assertions.
func (r *PersonRepository) DB() *sql.DB {
	return r.db
}
```

- [ ] **Step 2: Add DB() accessor to PersonRepository**

Add to `source-manager/internal/repository/person.go` (after the constructor):

```go
// DB returns the underlying *sql.DB for test assertions.
func (r *PersonRepository) DB() *sql.DB {
	return r.db
}
```

- [ ] **Step 3: Verify tests compile**

Run: `cd source-manager && go test -run TestPerson -count=1 -short ./internal/repository/`
Expected: Tests skip with "Skipping integration test in short mode"

- [ ] **Step 4: Run integration tests (requires test DB)**

Run: `cd source-manager && go test -run TestPerson -count=1 -v ./internal/repository/`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add source-manager/internal/repository/person.go source-manager/internal/repository/person_test.go
git commit -m "feat(source-manager): add PersonRepository tests (#271)"
```

---

## Chunk 4: BandOffice Repository + Tests

### Task 7: Create BandOfficeRepository

**Files:**
- Create: `source-manager/internal/repository/band_office.go`
- Reference: `source-manager/internal/repository/community.go`

- [ ] **Step 1: Write the BandOfficeRepository**

Create `source-manager/internal/repository/band_office.go`:

```go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

const bandOfficeColumnCount = 18

// bandOfficeColumns is the SELECT column list for the band_offices table.
const bandOfficeColumns = `id, community_id, data_source, verified, created_at, updated_at,
	address_line1, address_line2, city, province, postal_code,
	phone, fax, email, toll_free, office_hours, source_url, verified_at`

// BandOfficeRepository provides CRUD operations for the band_offices table.
type BandOfficeRepository struct {
	db     *sql.DB
	logger infralogger.Logger
}

// NewBandOfficeRepository creates a new BandOfficeRepository.
func NewBandOfficeRepository(db *sql.DB, log infralogger.Logger) *BandOfficeRepository {
	return &BandOfficeRepository{
		db:     db,
		logger: log,
	}
}

// scanBandOffice scans a single row into a BandOffice struct.
func scanBandOffice(row interface{ Scan(...any) error }) (*models.BandOffice, error) {
	var bo models.BandOffice
	scanErr := row.Scan(
		&bo.ID, &bo.CommunityID, &bo.DataSource, &bo.Verified, &bo.CreatedAt, &bo.UpdatedAt,
		&bo.AddressLine1, &bo.AddressLine2, &bo.City, &bo.Province, &bo.PostalCode,
		&bo.Phone, &bo.Fax, &bo.Email, &bo.TollFree, &bo.OfficeHours, &bo.SourceURL, &bo.VerifiedAt,
	)
	if scanErr != nil {
		return nil, fmt.Errorf("scan band office: %w", scanErr)
	}
	return &bo, nil
}

// Create inserts a new band office. ID and timestamps are set automatically.
func (r *BandOfficeRepository) Create(ctx context.Context, bo *models.BandOffice) error {
	bo.ID = uuid.New().String()
	bo.CreatedAt = time.Now()
	bo.UpdatedAt = time.Now()

	query := `
		INSERT INTO band_offices (
			id, community_id, data_source, verified, created_at, updated_at,
			address_line1, address_line2, city, province, postal_code,
			phone, fax, email, toll_free, office_hours, source_url, verified_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17, $18
		)`

	_, err := r.db.ExecContext(ctx, query,
		bo.ID, bo.CommunityID, bo.DataSource, bo.Verified, bo.CreatedAt, bo.UpdatedAt,
		bo.AddressLine1, bo.AddressLine2, bo.City, bo.Province, bo.PostalCode,
		bo.Phone, bo.Fax, bo.Email, bo.TollFree, bo.OfficeHours, bo.SourceURL, bo.VerifiedAt,
	)
	if err != nil {
		return fmt.Errorf("create band office: %w", err)
	}

	return nil
}

// GetByCommunity returns the band office for a community, or nil if not found.
func (r *BandOfficeRepository) GetByCommunity(ctx context.Context, communityID string) (*models.BandOffice, error) {
	query := `SELECT ` + bandOfficeColumns + ` FROM band_offices WHERE community_id = $1`
	row := r.db.QueryRowContext(ctx, query, communityID)

	bo, err := scanBandOffice(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil //nolint:nilnil // nil,nil = "not found" per interface contract
		}
		return nil, fmt.Errorf("get band office by community: %w", err)
	}

	return bo, nil
}

// Update modifies an existing band office by ID.
func (r *BandOfficeRepository) Update(ctx context.Context, bo *models.BandOffice) error {
	bo.UpdatedAt = time.Now()

	query := `
		UPDATE band_offices SET
			community_id = $2, data_source = $3, verified = $4, updated_at = $5,
			address_line1 = $6, address_line2 = $7, city = $8, province = $9, postal_code = $10,
			phone = $11, fax = $12, email = $13, toll_free = $14, office_hours = $15,
			source_url = $16, verified_at = $17
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		bo.ID, bo.CommunityID, bo.DataSource, bo.Verified, bo.UpdatedAt,
		bo.AddressLine1, bo.AddressLine2, bo.City, bo.Province, bo.PostalCode,
		bo.Phone, bo.Fax, bo.Email, bo.TollFree, bo.OfficeHours,
		bo.SourceURL, bo.VerifiedAt,
	)
	if err != nil {
		return fmt.Errorf("update band office: %w", err)
	}

	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("update band office rows affected: %w", rowsErr)
	}
	if rows == 0 {
		return errors.New("update band office: not found")
	}

	return nil
}

// DeleteByCommunity removes a band office by community ID.
func (r *BandOfficeRepository) DeleteByCommunity(ctx context.Context, communityID string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM band_offices WHERE community_id = $1", communityID)
	if err != nil {
		return fmt.Errorf("delete band office: %w", err)
	}

	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("delete band office rows affected: %w", rowsErr)
	}
	if rows == 0 {
		return errors.New("delete band office: not found")
	}

	return nil
}

// Upsert inserts or updates a band office by community_id.
func (r *BandOfficeRepository) Upsert(ctx context.Context, bo *models.BandOffice) error {
	if bo.ID == "" {
		bo.ID = uuid.New().String()
	}
	bo.CreatedAt = time.Now()
	bo.UpdatedAt = time.Now()

	query := `
		INSERT INTO band_offices (
			id, community_id, data_source, verified, created_at, updated_at,
			address_line1, address_line2, city, province, postal_code,
			phone, fax, email, toll_free, office_hours, source_url, verified_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17, $18
		)
		ON CONFLICT (community_id) DO UPDATE SET
			data_source = EXCLUDED.data_source, verified = EXCLUDED.verified,
			updated_at = EXCLUDED.updated_at,
			address_line1 = EXCLUDED.address_line1, address_line2 = EXCLUDED.address_line2,
			city = EXCLUDED.city, province = EXCLUDED.province, postal_code = EXCLUDED.postal_code,
			phone = EXCLUDED.phone, fax = EXCLUDED.fax, email = EXCLUDED.email,
			toll_free = EXCLUDED.toll_free, office_hours = EXCLUDED.office_hours,
			source_url = EXCLUDED.source_url, verified_at = EXCLUDED.verified_at
		RETURNING id`

	if err := r.db.QueryRowContext(ctx, query,
		bo.ID, bo.CommunityID, bo.DataSource, bo.Verified, bo.CreatedAt, bo.UpdatedAt,
		bo.AddressLine1, bo.AddressLine2, bo.City, bo.Province, bo.PostalCode,
		bo.Phone, bo.Fax, bo.Email, bo.TollFree, bo.OfficeHours, bo.SourceURL, bo.VerifiedAt,
	).Scan(&bo.ID); err != nil {
		return fmt.Errorf("upsert band office: %w", err)
	}

	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd source-manager && go build ./internal/repository/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add source-manager/internal/repository/band_office.go
git commit -m "feat(source-manager): add BandOfficeRepository with CRUD and Upsert (#271)"
```

### Task 8: Write BandOfficeRepository tests

**Files:**
- Create: `source-manager/internal/repository/band_office_test.go`
- Reference: `source-manager/internal/repository/community_test.go`

- [ ] **Step 1: Write the tests**

Create `source-manager/internal/repository/band_office_test.go`:

```go
package repository_test

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/jonesrussell/north-cloud/source-manager/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBandOfficeTestDB(t *testing.T) (
	bandOfficeRepo *repository.BandOfficeRepository,
	communityRepo *repository.CommunityRepository,
	cleanup func(),
) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db, dbCleanup := setupTestDB(t)
	logger := testhelpers.NewTestLogger()
	bandOfficeRepo = repository.NewBandOfficeRepository(db, logger)
	communityRepo = repository.NewCommunityRepository(db, logger)

	cleanup = func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, "TRUNCATE TABLE band_offices, communities CASCADE")
		dbCleanup()
	}

	return bandOfficeRepo, communityRepo, cleanup
}

func newTestBandOffice(communityID string) *models.BandOffice {
	return &models.BandOffice{
		CommunityID: communityID,
		DataSource:  "manual",
	}
}

func TestBandOfficeRepository_Create(t *testing.T) {
	boRepo, communityRepo, cleanup := setupBandOfficeTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	t.Run("create valid band office", func(t *testing.T) {
		bo := newTestBandOffice(community.ID)
		phone := "705-555-1234"
		bo.Phone = &phone
		city := "Sudbury"
		bo.City = &city

		err := boRepo.Create(ctx, bo)
		require.NoError(t, err)
		assert.NotEmpty(t, bo.ID)
		assert.False(t, bo.CreatedAt.IsZero())
	})

	t.Run("duplicate community_id returns error", func(t *testing.T) {
		bo := newTestBandOffice(community.ID)
		err := boRepo.Create(ctx, bo)
		assert.Error(t, err)
	})
}

func TestBandOfficeRepository_GetByCommunity(t *testing.T) {
	boRepo, communityRepo, cleanup := setupBandOfficeTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	t.Run("found", func(t *testing.T) {
		bo := newTestBandOffice(community.ID)
		phone := "705-555-1234"
		bo.Phone = &phone
		require.NoError(t, boRepo.Create(ctx, bo))

		found, err := boRepo.GetByCommunity(ctx, community.ID)
		require.NoError(t, err)
		assert.Equal(t, community.ID, found.CommunityID)
		assert.Equal(t, "705-555-1234", *found.Phone)
	})

	t.Run("not found", func(t *testing.T) {
		found, err := boRepo.GetByCommunity(ctx, "nonexistent-id")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestBandOfficeRepository_Update(t *testing.T) {
	boRepo, communityRepo, cleanup := setupBandOfficeTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)
	bo := newTestBandOffice(community.ID)
	require.NoError(t, boRepo.Create(ctx, bo))

	email := "office@band.ca"
	bo.Email = &email
	hours := "Mon-Fri 8:30am-4:30pm"
	bo.OfficeHours = &hours

	err := boRepo.Update(ctx, bo)
	require.NoError(t, err)

	found, getErr := boRepo.GetByCommunity(ctx, community.ID)
	require.NoError(t, getErr)
	assert.Equal(t, "office@band.ca", *found.Email)
	assert.Equal(t, "Mon-Fri 8:30am-4:30pm", *found.OfficeHours)
}

func TestBandOfficeRepository_DeleteByCommunity(t *testing.T) {
	boRepo, communityRepo, cleanup := setupBandOfficeTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)
	bo := newTestBandOffice(community.ID)
	require.NoError(t, boRepo.Create(ctx, bo))

	err := boRepo.DeleteByCommunity(ctx, community.ID)
	require.NoError(t, err)

	found, getErr := boRepo.GetByCommunity(ctx, community.ID)
	require.NoError(t, getErr)
	assert.Nil(t, found)
}

func TestBandOfficeRepository_DeleteByCommunity_NotFound(t *testing.T) {
	boRepo, _, cleanup := setupBandOfficeTestDB(t)
	defer cleanup()

	ctx := context.Background()
	err := boRepo.DeleteByCommunity(ctx, "nonexistent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBandOfficeRepository_Upsert(t *testing.T) {
	boRepo, communityRepo, cleanup := setupBandOfficeTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	t.Run("insert when new", func(t *testing.T) {
		bo := newTestBandOffice(community.ID)
		phone := "705-555-0001"
		bo.Phone = &phone

		err := boRepo.Upsert(ctx, bo)
		require.NoError(t, err)
		assert.NotEmpty(t, bo.ID)
	})

	t.Run("update when exists", func(t *testing.T) {
		bo := newTestBandOffice(community.ID)
		phone := "705-555-9999"
		bo.Phone = &phone
		email := "new@band.ca"
		bo.Email = &email

		err := boRepo.Upsert(ctx, bo)
		require.NoError(t, err)

		found, getErr := boRepo.GetByCommunity(ctx, community.ID)
		require.NoError(t, getErr)
		assert.Equal(t, "705-555-9999", *found.Phone)
		assert.Equal(t, "new@band.ca", *found.Email)
	})
}
```

- [ ] **Step 2: Verify tests compile**

Run: `cd source-manager && go test -run TestBandOffice -count=1 -short ./internal/repository/`
Expected: Tests skip with "Skipping integration test in short mode"

- [ ] **Step 3: Run integration tests (requires test DB)**

Run: `cd source-manager && go test -run TestBandOffice -count=1 -v ./internal/repository/`
Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add source-manager/internal/repository/band_office.go source-manager/internal/repository/band_office_test.go
git commit -m "feat(source-manager): add BandOfficeRepository tests (#271)"
```

---

## Chunk 5: Bootstrap Wiring + Final Verification

### Task 9: Wire repositories into bootstrap

**Files:**
- Modify: `source-manager/internal/bootstrap/server.go`

- [ ] **Step 1: Add repository instantiation**

Update `source-manager/internal/bootstrap/server.go` to instantiate the new repositories:

```go
package bootstrap

import (
	infragin "github.com/jonesrussell/north-cloud/infrastructure/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/api"
	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	"github.com/jonesrussell/north-cloud/source-manager/internal/database"
	"github.com/jonesrussell/north-cloud/source-manager/internal/events"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
)

// SetupHTTPServer creates and configures the HTTP server.
func SetupHTTPServer(
	cfg *config.Config,
	db *database.DB,
	publisher *events.Publisher,
	log infralogger.Logger,
) *infragin.Server {
	sourceRepo := repository.NewSourceRepository(db.DB(), log)
	_ = repository.NewPersonRepository(db.DB(), log)
	_ = repository.NewBandOfficeRepository(db.DB(), log)
	return api.NewServer(sourceRepo, cfg, log, publisher)
}
```

**Note:** Repositories are assigned to `_` because no handlers consume them yet. When API routes are added in a follow-up issue, replace `_` with named variables and pass to handlers.

- [ ] **Step 2: Verify the service builds**

Run: `cd source-manager && go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add source-manager/internal/bootstrap/server.go
git commit -m "feat(source-manager): wire PersonRepository and BandOfficeRepository into bootstrap (#271)"
```

### Task 10: Run full lint and test suite

- [ ] **Step 1: Run linter**

Run: `task lint:source-manager`
Expected: No lint errors

- [ ] **Step 2: Run all tests**

Run: `task test:source-manager`
Expected: All tests pass

- [ ] **Step 3: Fix any lint or test issues**

If linter flags issues (magic numbers, line length, cognitive complexity, etc.), fix them before proceeding.

- [ ] **Step 4: Final commit if fixes were needed**

```bash
git add -u source-manager/
git commit -m "fix(source-manager): address lint issues for people/band_offices (#271)"
```
