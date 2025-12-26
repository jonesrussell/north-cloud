package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jonesrussell/auth/internal/config"
	"github.com/jonesrussell/auth/internal/database"
	"github.com/jonesrussell/auth/internal/logger"
	"github.com/jonesrussell/auth/internal/models"
	"github.com/jonesrussell/auth/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	var configPath string
	var username string
	var email string
	var password string

	flag.StringVar(&configPath, "config", "config.yml", "Path to configuration file")
	flag.StringVar(&username, "username", "admin", "Username for the admin user")
	flag.StringVar(&email, "email", "admin@northcloud.biz", "Email for the admin user")
	flag.StringVar(&password, "password", "", "Password for the admin user (required)")
	flag.Parse()

	if password == "" {
		// Try to get from environment
		password = os.Getenv("AUTH_ADMIN_PASSWORD")
		if password == "" {
			fmt.Fprintf(os.Stderr, "Error: password is required. Use -password flag or AUTH_ADMIN_PASSWORD environment variable\n")
			os.Exit(1)
		}
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	appLogger, err := logger.NewLogger(cfg.Debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = appLogger.Sync()
	}()

	// Initialize database
	db, err := database.New(cfg, appLogger)
	if err != nil {
		appLogger.Error("Failed to connect to database",
			logger.Error(err),
		)
		os.Exit(1)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			appLogger.Error("Failed to close database",
				logger.Error(closeErr),
			)
		}
	}()

	// Initialize repository
	userRepo := repository.NewUserRepository(db.DB(), appLogger)

	// Check if user already exists
	exists, err := userRepo.Exists(username)
	if err != nil {
		appLogger.Error("Failed to check if user exists",
			logger.Error(err),
		)
		os.Exit(1)
	}

	if exists {
		fmt.Printf("User '%s' already exists. Skipping creation.\n", username)
		os.Exit(0)
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		appLogger.Error("Failed to hash password",
			logger.Error(err),
		)
		os.Exit(1)
	}

	// Create user
	user := &models.User{
		ID:           uuid.New(),
		Username:     username,
		Email:        email,
		PasswordHash: string(passwordHash),
	}

	if err := userRepo.Create(user); err != nil {
		appLogger.Error("Failed to create user",
			logger.Error(err),
		)
		os.Exit(1)
	}

	fmt.Printf("Successfully created admin user:\n")
	fmt.Printf("  Username: %s\n", username)
	fmt.Printf("  Email: %s\n", email)
	fmt.Printf("  ID: %s\n", user.ID.String())
}

