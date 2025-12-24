# Code Review: crawler/cmd/httpd/httpd.go

## Review Criteria
- **DRY** (Don't Repeat Yourself)
- **SRP** (Single Responsibility Principle)
- **Go 1.25 Best Practices**

## Note
A refactored reference implementation is available in `httpd_refactored.go` (for review purposes only - not meant to be compiled alongside the original).

---

## 🔴 Critical Issues

### 1. SRP Violation: `newCommandDeps()` Does Too Much

**Location**: Lines 306-349

**Problem**: This function violates SRP by handling:
- Config initialization
- Config loading
- Logger configuration extraction
- Logger creation
- Dependency validation

**Impact**: Hard to test, hard to reuse, violates single responsibility.

**Recommendation**: Split into separate functions:
```go
func newCommandDeps() (*CommandDeps, error) {
    if err := initConfig(); err != nil {
        return nil, fmt.Errorf("failed to initialize config: %w", err)
    }
    
    cfg, err := loadConfig()
    if err != nil {
        return nil, fmt.Errorf("load config: %w", err)
    }
    
    log, err := createLogger()
    if err != nil {
        return nil, fmt.Errorf("create logger: %w", err)
    }
    
    deps := &CommandDeps{
        Logger: log,
        Config: cfg,
    }
    
    if err := deps.validate(); err != nil {
        return nil, fmt.Errorf("validate deps: %w", err)
    }
    
    return deps, nil
}

func loadConfig() (config.Interface, error) {
    return config.LoadConfig()
}

func createLogger() (logger.Interface, error) {
    logLevel := getLogLevel()
    logCfg := &logger.Config{
        Level:       logger.Level(logLevel),
        Development: viper.GetBool("logger.development"),
        Encoding:    viper.GetString("logger.encoding"),
        OutputPaths: viper.GetStringSlice("logger.output_paths"),
        EnableColor: viper.GetBool("logger.enable_color"),
    }
    return logger.New(logCfg)
}

func getLogLevel() string {
    logLevel := viper.GetString("logger.level")
    if logLevel == "" {
        logLevel = "info"
    }
    return strings.ToLower(logLevel)
}
```

### 2. SRP Violation: `initConfig()` Does Too Much

**Location**: Lines 362-396

**Problem**: This function handles:
- Environment file loading
- Viper configuration
- Defaults setting
- Config file reading
- Environment variable binding
- Development logging setup

**Impact**: Hard to test individual concerns, violates SRP.

**Recommendation**: Split into focused functions:
```go
func initConfig() error {
    loadEnvFile()
    setupViper()
    setDefaults()
    readConfigFile()
    
    if err := bindEnvironmentVariables(); err != nil {
        return fmt.Errorf("failed to bind env vars: %w", err)
    }
    
    setupDevelopmentLogging()
    return nil
}

func setupViper() {
    viper.AutomaticEnv()
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath(".")
    viper.AddConfigPath("./config")
}

func loadEnvFile() {
    _ = godotenv.Load() // Ignore error if file doesn't exist
}

func readConfigFile() {
    _ = viper.ReadInConfig() // Ignore error if file doesn't exist
}

func bindEnvironmentVariables() error {
    if err := bindAppEnvVars(); err != nil {
        return fmt.Errorf("failed to bind app env vars: %w", err)
    }
    if err := bindElasticsearchEnvVars(); err != nil {
        return fmt.Errorf("failed to bind elasticsearch env vars: %w", err)
    }
    return nil
}
```

### 3. DRY Violation: Database Config Mapping

**Location**: Lines 94-101

**Problem**: Repeated field-by-field mapping from config to database config.

**Current Code**:
```go
dbConfig := database.Config{
    Host:     deps.Config.GetDatabaseConfig().Host,
    Port:     deps.Config.GetDatabaseConfig().Port,
    User:     deps.Config.GetDatabaseConfig().User,
    Password: deps.Config.GetDatabaseConfig().Password,
    DBName:   deps.Config.GetDatabaseConfig().DBName,
    SSLMode:  deps.Config.GetDatabaseConfig().SSLMode,
}
```

**Recommendation**: Add a converter method to database.Config:
```go
// In internal/database/postgres.go
func ConfigFromInterface(cfg dbconfig.Config) Config {
    return Config{
        Host:     cfg.Host,
        Port:     cfg.Port,
        User:     cfg.User,
        Password: cfg.Password,
        DBName:   cfg.DBName,
        SSLMode:  cfg.SSLMode,
    }
}

// In httpd.go
dbConfig := database.ConfigFromInterface(*deps.Config.GetDatabaseConfig())
```

### 4. DRY Violation: Context Creation Pattern

**Location**: Lines 137, 251

**Problem**: Repeated pattern of creating context with timeout and deferring cancel.

**Current Code**:
```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
```

And:
```go
ctx, cancel := infracontext.WithTimeout(constants.DefaultShutdownTimeout)
defer cancel()
```

**Recommendation**: Extract to helper functions or use consistent pattern:
```go
// Helper for scheduler context
func createSchedulerContext() (context.Context, context.CancelFunc) {
    return context.WithCancel(context.Background())
}

// Helper for timeout contexts
func createTimeoutContext(timeout time.Duration) (context.Context, context.CancelFunc) {
    return infracontext.WithTimeout(timeout)
}
```

### 5. DRY Violation: Logger Level Handling

**Location**: Lines 320-324, 540-554

**Problem**: Logger level normalization logic appears twice:
- In `newCommandDeps()` (lines 320-324)
- In `setupDevelopmentLogging()` (lines 540-554)

**Recommendation**: Extract to single function:
```go
func normalizeLogLevel(level string) string {
    if level == "" {
        return "info"
    }
    return strings.ToLower(level)
}
```

### 6. SRP Violation: `createCrawlerForJobs()` Mixes Concerns

**Location**: Lines 226-287

**Problem**: This function:
- Creates event bus
- Loads sources
- Creates raw content indexer
- Ensures indexes for all sources
- Creates crawler

**Impact**: Hard to test, violates SRP, mixes infrastructure setup with business logic.

**Recommendation**: Split into focused functions:
```go
func createCrawlerForJobs(
    deps *CommandDeps,
    storageResult *StorageResult,
) (crawler.Interface, error) {
    bus := events.NewEventBus(deps.Logger)
    crawlerCfg := deps.Config.GetCrawlerConfig()
    
    sourceManager, err := loadSourceManager(deps)
    if err != nil {
        return nil, err
    }
    
    if err := ensureRawContentIndexes(deps, storageResult, sourceManager); err != nil {
        deps.Logger.Warn("Failed to ensure raw content indexes", "error", err)
        // Continue - not fatal
    }
    
    return createCrawler(deps, bus, crawlerCfg, storageResult, sourceManager)
}

func loadSourceManager(deps *CommandDeps) (sources.Interface, error) {
    sourceManager, err := sources.LoadSources(deps.Config, deps.Logger)
    if err != nil {
        return nil, fmt.Errorf("failed to load sources: %w", err)
    }
    return sourceManager, nil
}

func ensureRawContentIndexes(
    deps *CommandDeps,
    storageResult *StorageResult,
    sourceManager sources.Interface,
) error {
    rawIndexer := storage.NewRawContentIndexer(storageResult.Storage, deps.Logger)
    allSources, err := sourceManager.GetSources()
    if err != nil {
        return fmt.Errorf("failed to get sources: %w", err)
    }
    
    ctx, cancel := infracontext.WithTimeout(constants.DefaultShutdownTimeout)
    defer cancel()
    
    for i := range allSources {
        sourceHostname := extractHostnameFromURL(allSources[i].URL)
        if sourceHostname == "" {
            sourceHostname = allSources[i].Name
        }
        if err := rawIndexer.EnsureRawContentIndex(ctx, sourceHostname); err != nil {
            deps.Logger.Warn("Failed to ensure raw content index",
                "source", allSources[i].Name,
                "source_url", allSources[i].URL,
                "hostname", sourceHostname,
                "error", err)
            // Continue with other sources
        }
    }
    return nil
}

func createCrawler(
    deps *CommandDeps,
    bus *events.EventBus,
    crawlerCfg *crawlerconfig.Config,
    storageResult *StorageResult,
    sourceManager sources.Interface,
) (crawler.Interface, error) {
    crawlerResult, err := crawler.NewCrawlerWithParams(crawler.CrawlerParams{
        Logger:       deps.Logger,
        Bus:          bus,
        IndexManager: storageResult.IndexManager,
        Sources:      sourceManager,
        Config:       crawlerCfg,
        Storage:      storageResult.Storage,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create crawler: %w", err)
    }
    return crawlerResult.Crawler, nil
}
```

---

## 🟡 Medium Priority Issues

### 7. Error Wrapping Inconsistency

**Location**: Throughout file

**Problem**: Some errors use `fmt.Errorf("context: %w", err)`, others use `fmt.Errorf("failed to ...: %w", err)`.

**Recommendation**: Use consistent error wrapping pattern:
- Use `fmt.Errorf("operation failed: %w", err)` for consistency
- Consider using `errors.Join()` for multiple errors (Go 1.20+)

### 8. Magic Values

**Location**: Lines 183, 204

**Problem**: Channel buffer sizes and timeouts are hardcoded.

**Current Code**:
```go
sigChan := make(chan os.Signal, 1)
```

**Recommendation**: Extract to constants:
```go
const (
    signalChannelBufferSize = 1
    errorChannelBufferSize  = 1
)
```

### 9. Context Cancellation in `createAndStartScheduler()`

**Location**: Lines 137-138

**Problem**: Context is created and immediately cancelled with defer, but the context is passed to `dbScheduler.Start(ctx)`. This means the context is cancelled as soon as the function returns, which may not be the intended behavior.

**Current Code**:
```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
// ...
dbScheduler.Start(ctx)
```

**Recommendation**: The scheduler should manage its own context lifecycle. Either:
1. Don't cancel immediately (let scheduler manage it)
2. Or pass a parent context that the scheduler can derive from

**Better approach**:
```go
// Scheduler manages its own context internally
// Or pass a background context without immediate cancellation
ctx := context.Background()
dbScheduler.Start(ctx)
```

### 10. Unused Return Value

**Location**: Line 158

**Problem**: `api.StartHTTPServer()` returns three values, but the second return value is ignored.

**Current Code**:
```go
srv, _, err := api.StartHTTPServer(deps.Logger, searchManager, deps.Config, jobsHandler)
```

**Recommendation**: Either use the return value or document why it's ignored:
```go
srv, router, err := api.StartHTTPServer(deps.Logger, searchManager, deps.Config, jobsHandler)
// router is unused but may be needed for future middleware registration
```

---

## 🟢 Minor Issues & Best Practices

### 11. Go 1.25 Best Practices

**Recommendations**:
- ✅ Already using structured logging (good!)
- ✅ Already using context.Context (good!)
- ✅ Error wrapping with `%w` (good!)
- Consider using `errors.Join()` for multiple errors (Go 1.20+)
- Consider using `context.WithoutCancel()` if needed (Go 1.21+)
- Use `sync.Once` for one-time initialization if needed

### 12. Function Length

**Location**: `Start()` function (lines 55-85)

**Problem**: Function is doing orchestration but could be clearer.

**Recommendation**: Already well-structured with helper functions. Consider adding comments for each phase:
```go
func Start() error {
    // Phase 1: Initialize dependencies
    deps, err := newCommandDeps()
    // ...
    
    // Phase 2: Setup storage
    storageResult, err := createStorage(deps.Config, deps.Logger)
    // ...
    
    // Phase 3: Setup jobs and scheduler
    jobsHandler, dbScheduler, db := setupJobsAndScheduler(deps, storageResult)
    // ...
    
    // Phase 4: Start HTTP server
    srv, errChan, err := startHTTPServer(deps, searchManager, jobsHandler)
    // ...
    
    // Phase 5: Run until interrupted
    return runServerUntilInterrupt(deps.Logger, srv, dbScheduler, errChan)
}
```

### 13. Variable Naming

**Location**: Line 158

**Problem**: `srv` is abbreviated.

**Recommendation**: Use full name for clarity:
```go
server, errChan, err := startHTTPServer(...)
```

### 14. Error Message Consistency

**Location**: Throughout

**Problem**: Some errors say "failed to", others don't.

**Recommendation**: Use consistent prefix: "failed to {operation}"

---

## Summary

### DRY Violations Found: 3
1. Database config mapping (lines 94-101)
2. Context creation pattern (lines 137, 251)
3. Logger level handling (lines 320-324, 540-554)

### SRP Violations Found: 3
1. `newCommandDeps()` does too much (lines 306-349)
2. `initConfig()` does too much (lines 362-396)
3. `createCrawlerForJobs()` mixes concerns (lines 226-287)

### Go 1.25 Best Practices: ✅ Mostly Good
- Error wrapping: ✅ Good
- Context usage: ✅ Good (with minor issue)
- Structured logging: ✅ Good
- Consider: `errors.Join()` for multiple errors

---

## Priority Refactoring Order

1. **High Priority**: Split `newCommandDeps()` and `initConfig()` (SRP violations)
2. **High Priority**: Fix context cancellation in `createAndStartScheduler()` (bug risk)
3. **Medium Priority**: Extract database config converter (DRY)
4. **Medium Priority**: Split `createCrawlerForJobs()` (SRP)
5. **Low Priority**: Extract context creation helpers (DRY)
6. **Low Priority**: Extract logger level normalization (DRY)

