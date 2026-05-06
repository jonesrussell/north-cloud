---
work_package_id: WP07
title: Configuration
dependencies:
- WP05
- WP06
requirement_refs:
- C-002
- FR-001
- FR-014
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T028
- T029
- T030
- T031
phase: B
agent: "claude:sonnet:implementer:implementer"
shell_pid: "229458"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/internal/config/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/internal/config/**
priority: P1
tags: []
---

# WP07 — Configuration

## Objective

Wire alert-crawler's configuration through `infrastructure/config.LoadWithDefaults[Config]` with env-tag binding (env > YAML > SetDefaults precedence). Includes a regression test that asserts SetDefaults-owned fields are absent from `config.yml` (RR-007 pitfall mitigation).

## Context

- Plan §Component Design (config), §TC-009 (per-source expiry), §TC-012 (severity table)
- Research §R-002 (signal-crawler config pattern, RR-007 pitfall)
- Spec §3 FR-001, FR-014, §5 C-002

## Branch Strategy

Standard. Depends on WP05 (scaffold) and WP06 (uses domain types like `AlertSource`).

## Subtasks

### T028 — Create `internal/config/config.go`

**Purpose**: Define the `Config` root struct with subsystem configs.

**Steps**:
1. Create `alert-crawler/internal/config/config.go`:
   ```go
   package config

   import (
       "time"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
   )

   type Config struct {
       Service      ServiceConfig         `yaml:"service" envPrefix:"SERVICE_"`
       Sources      []domain.AlertSource  `yaml:"sources" envPrefix:"SOURCE_"`
       Database     DatabaseConfig        `yaml:"database" envPrefix:"DB_"`
       Elasticsearch ESConfig             `yaml:"elasticsearch" envPrefix:"ES_"`
       Redis        RedisConfig           `yaml:"redis" envPrefix:"REDIS_"`
       Severity     SeverityConfig        `yaml:"severity" envPrefix:"SEVERITY_"`
       Observability ObservabilityConfig  `yaml:"observability" envPrefix:"OBS_"`
   }

   type ServiceConfig struct {
       Name string `yaml:"name" env:"NAME"`
   }

   type DatabaseConfig struct {
       Path           string `yaml:"path" env:"PATH"`
       MigrationsPath string `yaml:"migrations_path" env:"MIGRATIONS_PATH"`
   }

   type ESConfig struct {
       URL   string `yaml:"url" env:"URL"`
       Index string `yaml:"index" env:"INDEX"`
   }

   type RedisConfig struct {
       URL     string `yaml:"url" env:"URL"`
       Channel string `yaml:"channel" env:"CHANNEL"`
   }

   type SeverityConfig struct {
       Table map[string]domain.Severity `yaml:"table" env:"TABLE"`
   }

   type ObservabilityConfig struct {
       LogLevel string `yaml:"log_level" env:"LOG_LEVEL"`
   }
   ```
2. Use `infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"` for the loader.

**Files**:
- `alert-crawler/internal/config/config.go` (new, ~70 lines).

### T029 — Create `internal/config/defaults.go` (`SetDefaults`)

**Purpose**: Populate sensible defaults for fields whose absence in `config.yml` is intentional (RR-007).

**Steps**:
1. Create `alert-crawler/internal/config/defaults.go`:
   ```go
   func SetDefaults(c *Config) {
       if c.Service.Name == "" {
           c.Service.Name = "alert-crawler"
       }
       if c.Database.Path == "" {
           c.Database.Path = "/app/data/state.db"
       }
       if c.Elasticsearch.URL == "" {
           c.Elasticsearch.URL = "http://elasticsearch:9200"
       }
       if c.Elasticsearch.Index == "" {
           c.Elasticsearch.Index = "community_alerts"
       }
       if c.Redis.URL == "" {
           c.Redis.URL = "redis://redis:6379"
       }
       if c.Redis.Channel == "" {
           c.Redis.Channel = "community_alerts:lifecycle"
       }
       if c.Observability.LogLevel == "" {
           c.Observability.LogLevel = "info"
       }
       if len(c.Sources) == 0 {
           c.Sources = defaultSources()
       } else {
           for i := range c.Sources {
               applySourceDefaults(&c.Sources[i])
           }
       }
       if len(c.Severity.Table) == 0 {
           c.Severity.Table = defaultSeverityTable()
       }
   }

   func defaultSources() []domain.AlertSource {
       return []domain.AlertSource{
           {
               ID:                  "mhrn",
               Name:                "Manitoba Harm Reduction Network",
               FeedURL:             "https://www.safersites.ca/drugalerts.rss",
               AcquisitionStrategy: domain.AcquisitionRSS,
               PollInterval:        30 * time.Minute,
               DefaultCategory:     domain.CategoryHarmReduction,
               DefaultScope:        []string{"treaty:1", "canada:manitoba"},
               DefaultExpiry:       720 * time.Hour, // 30 days, per TC-009
               Enabled:             true,
           },
       }
   }

   func defaultSeverityTable() map[string]domain.Severity {
       return map[string]domain.Severity{
           "carfentanil":   domain.SeverityCritical,
           "nitazenes":     domain.SeverityHigh,
           "medetomidine":  domain.SeverityHigh,
           "xylazine":      domain.SeverityHigh,
           "fentanyl":      domain.SeverityHigh,  // baseline for opioid supply alerts
           "benzodiazepine": domain.SeverityHigh,
       }
   }

   func applySourceDefaults(s *domain.AlertSource) { /* per-source default fill */ }
   ```
2. Critical: every default field listed here MUST NOT appear in `alert-crawler/config.yml` (T031 enforces).

**Files**:
- `alert-crawler/internal/config/defaults.go` (new, ~80 lines).

### T030 — Wire to `infrastructure/config.LoadWithDefaults`

**Purpose**: Standard NC config loading entry point.

**Steps**:
1. In `internal/config/config.go`, add:
   ```go
   func Load(path string) (*Config, error) {
       return infraconfig.LoadWithDefaults[Config](path, SetDefaults)
   }
   ```
2. The loader handles env-tag binding and YAML merge per the existing `infrastructure/config` semantics. Precedence is: env > YAML > SetDefaults.
3. Add a brief godoc comment noting the precedence and the SetDefaults pitfall.

**Files**:
- `alert-crawler/internal/config/config.go` (modified, +~10 lines).

**Validation**:
- `Load("nonexistent")` returns a Config populated entirely from SetDefaults.
- `Load("alert-crawler/config.yml")` returns a Config with defaults intact (since config.yml is mostly comments).

### T031 — SetDefaults pitfall regression test

**Purpose**: Block future regressions where someone "helpfully" puts a real value in `config.yml` for a SetDefaults-owned field.

**Steps**:
1. Create `alert-crawler/internal/config/pitfall_test.go`:
   ```go
   func TestConfigYMLDoesNotShadowSetDefaults(t *testing.T) {
       // Read config.yml; for each SetDefaults-owned field, assert it is
       // absent from the YAML (via reflection or string-search).
       data, err := os.ReadFile("../../config.yml")
       if err != nil {
           t.Fatalf("config.yml not readable: %v", err)
       }
       yaml := string(data)
       forbidden := []string{
           "name:",                    // service.name
           "path: /app/data",          // database.path
           "url: http://elasticsearch", // elasticsearch.url
           "index: community_alerts",   // elasticsearch.index
           "url: redis://",            // redis.url
           "channel: community_alerts", // redis.channel
           "feed_url: https",          // sources[].feed_url
           "acquisition_strategy: rss", // sources[].acquisition_strategy
           "poll_interval:",           // sources[].poll_interval
           "default_category:",        // sources[].default_category
           "default_scope:",           // sources[].default_scope
           "default_expiry:",          // sources[].default_expiry
       }
       for _, line := range forbidden {
           if !strings.HasPrefix(strings.TrimSpace(line), "#") &&
              !strings.HasPrefix(yaml, "#"+line) &&
              strings.Contains(yaml, "\n"+line) {
               t.Errorf("config.yml carries SetDefaults-owned line %q (RR-007 pitfall — keep blank/commented)", line)
           }
       }
   }
   ```
   (The exact pattern-matching is illustrative; the agent should design a robust check.)
2. Add additional tests:
   - **TestLoadWithEmptyYAML**: load with an empty file path; verify all defaults are populated.
   - **TestLoadEnvOverridesYAML**: set an env var; load with a YAML; verify env wins.
   - **TestLoadDefaultsCoverAllFields**: walk the Config struct via reflection; for each field, verify SetDefaults populated it (or its parent) when the YAML did not.

**Files**:
- `alert-crawler/internal/config/pitfall_test.go` (new, ~120 lines).
- Potentially `alert-crawler/internal/config/config_test.go` for the broader load tests.

**Validation**:
- `task test:alert-crawler` passes for the config package.
- Coverage ≥80%.
- Editing `config.yml` to add a real value for a SetDefaults-owned field causes the pitfall test to fail.

## Definition of Done

- Config struct matches the operational needs of subsequent WPs.
- SetDefaults populates every default field.
- Loader is wired through `infrastructure/config.LoadWithDefaults`.
- Pitfall regression test passes and would catch regressions.
- Coverage ≥80%.

## Risks

- **RR-007**: addressed directly. Reviewer must verify the pitfall test actually catches an injected violation.
- **Env tag conflicts**: each subsystem uses an `envPrefix` so env vars don't collide (`SERVICE_NAME` vs `SOURCE_NAME`). Verify the prefix is honored.

## Reviewer Guidance

- Verify SetDefaults populates every needed field. Run a mental trace: "if config.yml is empty, would the runner have everything it needs?"
- Verify the pitfall test fails when you add a real value for, e.g., `feed_url:` to `config.yml`.
- Verify env precedence works.
- Verify the loader does not log secrets at startup.

## Implementation Command

```bash
spec-kitty agent action implement WP07 --agent <name>
```

Depends on WP05, WP06.

## Activity Log

- 2026-05-06T22:21:25Z – claude:sonnet:implementer:implementer – shell_pid=229458 – Started implementation via action command
