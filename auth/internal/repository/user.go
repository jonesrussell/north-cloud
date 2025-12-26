package repository

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jonesrussell/auth/internal/logger"
	"github.com/jonesrussell/auth/internal/models"
)

type UserRepository struct {
	db     *sql.DB
	logger logger.Logger
}

func NewUserRepository(db *sql.DB, log logger.Logger) *UserRepository {
	return &UserRepository{
		db:     db,
		logger: log,
	}
}

func (r *UserRepository) Create(user *models.User) error {
	query := `
		INSERT INTO users (id, username, email, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	_, err := r.db.Exec(query, user.ID, user.Username, user.Email, user.PasswordHash, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		r.logger.Error("Failed to create user",
			logger.String("username", user.Username),
			logger.Error(err),
		)
		return err
	}

	return nil
}

func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, created_at, updated_at
		FROM users
		WHERE username = $1
	`

	var user models.User
	err := r.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		r.logger.Error("Failed to get user by username",
			logger.String("username", username),
			logger.Error(err),
		)
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetByID(id uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		r.logger.Error("Failed to get user by ID",
			logger.String("id", id.String()),
			logger.Error(err),
		)
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) UpdatePassword(userID uuid.UUID, passwordHash string) error {
	query := `
		UPDATE users
		SET password_hash = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.Exec(query, passwordHash, time.Now(), userID)
	if err != nil {
		r.logger.Error("Failed to update password",
			logger.String("user_id", userID.String()),
			logger.Error(err),
		)
		return err
	}

	return nil
}

func (r *UserRepository) Exists(username string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`
	var exists bool
	err := r.db.QueryRow(query, username).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

