//nolint:testpackage // Testing with sqlmock requires same-package access
package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockBandOfficeRepo(t *testing.T) (*BandOfficeRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := NewBandOfficeRepository(db, newTestLogger(t))
	cleanup := func() {
		mock.ExpectClose()
		db.Close()
	}

	return repo, mock, cleanup
}

func bandOfficeCols() []string {
	return []string{
		"id", "community_id", "data_source", "verified", "created_at", "updated_at",
		"address_line1", "address_line2", "city", "province", "postal_code",
		"phone", "fax", "email", "toll_free", "office_hours", "source_url", "verified_at",
		"verification_confidence", "verification_issues",
	}
}

func TestBandOfficeRepository_Create_Mock(t *testing.T) {
	repo, mock, cleanup := newMockBandOfficeRepo(t)
	defer cleanup()

	ctx := context.Background()
	bo := &models.BandOffice{
		CommunityID: "c-1",
		DataSource:  "manual",
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO band_offices")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.Create(ctx, bo)
	require.NoError(t, err)
	assert.NotEmpty(t, bo.ID)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBandOfficeRepository_GetByCommunity_Mock(t *testing.T) {
	repo, mock, cleanup := newMockBandOfficeRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT " + bandOfficeColumns)).
		WithArgs("c-1").
		WillReturnRows(
			sqlmock.NewRows(bandOfficeCols()).AddRow(
				"bo-1", "c-1", "manual", false, now, now,
				nil, nil, nil, nil, nil,
				nil, nil, nil, nil, nil, nil, nil,
				nil, nil,
			),
		)

	bo, err := repo.GetByCommunity(ctx, "c-1")
	require.NoError(t, err)
	require.NotNil(t, bo)
	assert.Equal(t, "c-1", bo.CommunityID)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBandOfficeRepository_GetByCommunity_NotFound(t *testing.T) {
	repo, mock, cleanup := newMockBandOfficeRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT " + bandOfficeColumns)).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows(bandOfficeCols()))

	bo, err := repo.GetByCommunity(ctx, "missing")
	require.NoError(t, err)
	assert.Nil(t, bo)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBandOfficeRepository_Delete_Mock(t *testing.T) {
	repo, mock, cleanup := newMockBandOfficeRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM band_offices")).
		WithArgs("c-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.DeleteByCommunity(ctx, "c-1")
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBandOfficeRepository_Delete_NotFound(t *testing.T) {
	repo, mock, cleanup := newMockBandOfficeRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM band_offices")).
		WithArgs("missing").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.DeleteByCommunity(ctx, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBandOfficeRepository_Update_NotFound(t *testing.T) {
	repo, mock, cleanup := newMockBandOfficeRepo(t)
	defer cleanup()

	ctx := context.Background()
	bo := &models.BandOffice{ID: "missing", CommunityID: "c-1"}

	mock.ExpectExec(regexp.QuoteMeta("UPDATE band_offices")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Update(ctx, bo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	require.NoError(t, mock.ExpectationsWereMet())
}
