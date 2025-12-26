package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"github.com/jonesrussell/auth/internal/api"
	"github.com/jonesrussell/auth/internal/config"
	"github.com/jonesrussell/auth/internal/database"
	"github.com/jonesrussell/auth/internal/logger"
	"github.com/jonesrussell/auth/internal/middleware"
	"github.com/jonesrussell/auth/internal/models"
	"github.com/jonesrussell/auth/internal/repository"
	infracontext "github.com/north-cloud/infrastructure/context"
	"golang.org/x/crypto/bcrypt"
)

var (
	version = "dev"
)

func main() {
	// Check if migrate command is requested
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		runMigrateCommand()
		os.Exit(0)
		return
	}

	// Check if seed command is requested
	if len(os.Args) > 1 && os.Args[1] == "seed" {
		// Run seed command and exit
		runSeedCommand()
		os.Exit(0)
		return
	}

	var configPath string
	var autoMigrate bool
	flag.StringVar(&configPath, "config", "config.yml", "Path to configuration file")
	flag.BoolVar(&autoMigrate, "auto-migrate", false, "Run migrations automatically on startup")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		tempLogger, _ := logger.NewLogger(true)
		tempLogger.Error("Failed to load config",
			logger.String("config_path", configPath),
			logger.Error(err),
		)
		_ = tempLogger.Sync()
		os.Exit(1)
	}

	// Initialize logger
	appLogger, err := logger.NewLogger(cfg.Debug)
	if err != nil {
		tempLogger, _ := logger.NewLogger(true)
		tempLogger.Error("Failed to create logger",
			logger.Error(err),
		)
		_ = tempLogger.Sync()
		os.Exit(1)
	}
	defer func() {
		_ = appLogger.Sync()
	}()

	appLogger = appLogger.With(
		logger.String("service", "auth"),
		logger.String("version", version),
	)

	// Run migrations automatically if requested
	if autoMigrate || os.Getenv("AUTO_MIGRATE") == "true" {
		appLogger.Info("Running automatic migrations on startup")
		if err := database.RunMigrations(cfg, appLogger); err != nil {
			appLogger.Error("Automatic migration failed",
				logger.Error(err),
			)
			os.Exit(1)
		}
		appLogger.Info("Automatic migrations completed successfully")
	}

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

	// Initialize JWT middleware
	jwtMiddleware := middleware.NewJWTMiddleware(cfg, appLogger)

	// Initialize router
	router := api.NewRouter(userRepo, jwtMiddleware, cfg, appLogger)

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		appLogger.Info("Starting HTTP server",
			logger.String("host", cfg.Server.Host),
			logger.Int("port", cfg.Server.Port),
		)

		if serveErr := srv.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			appLogger.Error("HTTP server failed",
				logger.Error(serveErr),
			)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server")

	// Graceful shutdown
	ctx, cancel := infracontext.WithShutdownTimeout()
	defer cancel()

	if shutdownErr := srv.Shutdown(ctx); shutdownErr != nil {
		appLogger.Error("Server forced to shutdown",
			logger.Error(shutdownErr),
		)
	}

	appLogger.Info("Server exited")
}

// runSeedCommand runs the seed command to create admin user
func runSeedCommand() {
	// Create a new flag set for seed command
	seedFlags := flag.NewFlagSet("seed", flag.ExitOnError)
	var configPath string
	var username string
	var email string
	var password string
	
	seedFlags.StringVar(&configPath, "config", "config.yml", "Path to configuration file")
	seedFlags.StringVar(&username, "username", "admin", "Username for the admin user")
	seedFlags.StringVar(&email, "email", "admin@northcloud.biz", "Email for the admin user")
	seedFlags.StringVar(&password, "password", "", "Password for the admin user (required)")
	
	// Parse flags, skipping "seed" command
	if len(os.Args) > 2 {
		seedFlags.Parse(os.Args[2:])
	} else {
		seedFlags.Parse([]string{})
	}
	
	if password == "" {
		password = os.Getenv("AUTH_ADMIN_PASSWORD")
		if password == "" {
			fmt.Fprintf(os.Stderr, "Error: password is required. Use -password flag or AUTH_ADMIN_PASSWORD environment variable\n")
			os.Exit(1)
		}
	}
	
	// Load config
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
		appLogger.Error("Failed to connect to database", logger.Error(err))
		os.Exit(1)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			appLogger.Error("Failed to close database", logger.Error(closeErr))
		}
	}()
	
	// Initialize repository
	userRepo := repository.NewUserRepository(db.DB(), appLogger)
	
	// Check if user already exists
	exists, err := userRepo.Exists(username)
	if err != nil {
		appLogger.Error("Failed to check if user exists", logger.Error(err))
		os.Exit(1)
	}
	
	if exists {
		fmt.Printf("User '%s' already exists. Skipping creation.\n", username)
		os.Exit(0)
	}
	
	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		appLogger.Error("Failed to hash password", logger.Error(err))
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
		appLogger.Error("Failed to create user", logger.Error(err))
		os.Exit(1)
	}
	
	fmt.Printf("Successfully created admin user:\n")
	fmt.Printf("  Username: %s\n", username)
	fmt.Printf("  Email: %s\n", email)
	fmt.Printf("  ID: %s\n", user.ID.String())
}

// runMigrateCommand runs the migration command
func runMigrateCommand() {
	// Create a new flag set for migrate command
	migrateFlags := flag.NewFlagSet("migrate", flag.ExitOnError)
	var configPath string
	var steps int
	var version int

	migrateFlags.StringVar(&configPath, "config", "config.yml", "Path to configuration file")
	migrateFlags.IntVar(&steps, "steps", 1, "Number of migrations to rollback (for down command)")
	migrateFlags.IntVar(&version, "version", 0, "Migration version (for force command)")

	// Parse flags, skipping "migrate" command
	if len(os.Args) > 2 {
		migrateFlags.Parse(os.Args[2:])
	} else {
		migrateFlags.Parse([]string{})
	}

	// Determine subcommand (up, down, version, force)
	subcommand := "up"
	if len(os.Args) > 2 {
		subcommand = os.Args[2]
	}

	// Load config
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

	switch subcommand {
	case "up":
		if err := database.RunMigrations(cfg, appLogger); err != nil {
			appLogger.Error("Migration failed", logger.Error(err))
			fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations applied successfully")

	case "down":
		if err := database.MigrateDown(cfg, steps, appLogger); err != nil {
			appLogger.Error("Migration rollback failed", logger.Error(err))
			fmt.Fprintf(os.Stderr, "Migration rollback failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Rolled back %d migration(s) successfully\n", steps)

	case "version":
		ver, dirty, err := database.GetMigrationVersion(cfg, appLogger)
		if err != nil {
			appLogger.Error("Failed to get migration version", logger.Error(err))
			fmt.Fprintf(os.Stderr, "Failed to get migration version: %v\n", err)
			os.Exit(1)
		}
		if dirty {
			fmt.Printf("Current version: %d (dirty)\n", ver)
		} else {
			fmt.Printf("Current version: %d\n", ver)
		}

	case "force":
		if version <= 0 {
			fmt.Fprintf(os.Stderr, "Error: version is required for force command. Use -version flag\n")
			os.Exit(1)
		}
		if err := database.ForceMigrationVersion(cfg, version, appLogger); err != nil {
			appLogger.Error("Failed to force migration version", logger.Error(err))
			fmt.Fprintf(os.Stderr, "Failed to force migration version: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Migration version forced to %d\n", version)

	default:
		fmt.Fprintf(os.Stderr, "Unknown migrate subcommand: %s\n", subcommand)
		fmt.Fprintf(os.Stderr, "Usage: auth migrate [up|down|version|force]\n")
		fmt.Fprintf(os.Stderr, "  up      - Run all pending migrations\n")
		fmt.Fprintf(os.Stderr, "  down    - Rollback migrations (use -steps N)\n")
		fmt.Fprintf(os.Stderr, "  version - Show current migration version\n")
		fmt.Fprintf(os.Stderr, "  force   - Force migration version (use -version N)\n")
		os.Exit(1)
	}
}

