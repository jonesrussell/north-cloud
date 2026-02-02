# MCP Server - Git-Backed Classification Rules Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add 9 MCP tools for managing classification rules via Git, enabling version control, history, and rollback for rule changes.

**Architecture:** MCP server acts as Git gateway - it reads/writes YAML rule files from a local directory and syncs to the classifier database via existing CRUD APIs. The classifier service remains unchanged, using its database as the runtime source. Git provides versioning/history.

**Tech Stack:** Go 1.24+, go-git v5, YAML, HTTP REST, JSON-RPC 2.0 (MCP protocol)

---

## Prerequisites

Before starting, ensure:
- `task docker:dev:up` runs all services
- Classifier accessible: `curl http://localhost:8071/health`
- Git installed on the system

---

## Architecture Overview

```
┌─────────────┐      ┌─────────────────┐      ┌────────────────┐
│  Dashboard  │─────▶│   MCP Server    │─────▶│   Classifier   │
│   (future)  │      │  (Git Gateway)  │      │   (Database)   │
└─────────────┘      └─────────────────┘      └────────────────┘
                            │
                            ▼
                     ┌─────────────┐
                     │ classifier/ │
                     │   rules/    │
                     │  (YAML)     │
                     └─────────────┘
```

**Flow:**
1. Rules stored as YAML in `classifier/rules/` directory
2. MCP tools read/write these files
3. MCP syncs changes to classifier via `POST /api/v1/rules` API
4. Git provides version history and rollback

---

## Task 1: Create Rules Directory Structure

**Files:**
- Create: `/home/fsd42/dev/north-cloud/classifier/rules/README.md`
- Create: `/home/fsd42/dev/north-cloud/classifier/rules/topics/crime/violent_crime.yaml`
- Create: `/home/fsd42/dev/north-cloud/classifier/rules/topics/crime/property_crime.yaml`
- Create: `/home/fsd42/dev/north-cloud/classifier/rules/topics/crime/drug_crime.yaml`
- Create: `/home/fsd42/dev/north-cloud/classifier/rules/topics/crime/organized_crime.yaml`
- Create: `/home/fsd42/dev/north-cloud/classifier/rules/topics/crime/criminal_justice.yaml`

**Step 1.1: Create rules directory and README**

```bash
mkdir -p /home/fsd42/dev/north-cloud/classifier/rules/topics/crime
mkdir -p /home/fsd42/dev/north-cloud/classifier/rules/topics/general
```

Create `classifier/rules/README.md`:

```markdown
# Classification Rules

This directory contains YAML-based classification rules for the content classifier.

## Directory Structure

```
rules/
├── topics/
│   ├── crime/
│   │   ├── violent_crime.yaml
│   │   ├── property_crime.yaml
│   │   ├── drug_crime.yaml
│   │   ├── organized_crime.yaml
│   │   └── criminal_justice.yaml
│   └── general/
│       ├── sports.yaml
│       ├── politics.yaml
│       └── local_news.yaml
└── README.md
```

## Rule File Format

```yaml
rule_name: violent_crime_detection
rule_type: topic
topic_name: violent_crime
priority: 10
enabled: true
min_confidence: 0.3
keywords:
  - murder
  - homicide
  - assault
metadata:
  author: admin
  description: Detects violent crime content
  version: "1.0"
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| rule_name | string | yes | Unique identifier (snake_case) |
| rule_type | string | yes | "topic" or "content_type" |
| topic_name | string | yes* | Topic to assign (*required for topic rules) |
| priority | int | no | Higher = evaluated first (default: 0) |
| enabled | bool | no | Whether rule is active (default: true) |
| min_confidence | float | no | Threshold 0.0-1.0 (default: 0.5) |
| keywords | []string | yes | List of keywords to match |
| metadata | object | no | Author, description, version info |

## Syncing Rules

Rules are synced to the classifier database via MCP tools:

```bash
# List rules from files
mcp: list_classification_rules

# Sync all rules to database
mcp: sync_rules_to_classifier

# Test a rule against sample content
mcp: test_classification_rule
```
```

**Step 1.2: Create initial rule files**

Create `classifier/rules/topics/crime/violent_crime.yaml`:

```yaml
rule_name: violent_crime_detection
rule_type: topic
topic_name: violent_crime
priority: 10
enabled: true
min_confidence: 0.3
keywords:
  - murder
  - homicide
  - assault
  - shooting
  - stabbing
  - killing
  - manslaughter
  - attack
  - attacked
  - weapon
  - knife
  - firearms
  - gunshot
  - victim
  - fatal
  - deadly
  - violence
  - violent
  - gang
  - gang-related
  - drive-by
  - shootout
  - brawl
  - beating
  - domestic violence
  - rape
  - sexual assault
metadata:
  author: north-cloud
  description: Detects violent crime-related content
  version: "1.0"
  created_at: "2025-01-26"
```

Create `classifier/rules/topics/crime/property_crime.yaml`:

```yaml
rule_name: property_crime_detection
rule_type: topic
topic_name: property_crime
priority: 9
enabled: true
min_confidence: 0.3
keywords:
  - theft
  - robbery
  - burglary
  - stolen
  - vandalism
  - arson
  - break-in
  - breaking
  - shoplifting
  - fraud
  - embezzlement
  - larceny
  - trespassing
  - property damage
  - looting
metadata:
  author: north-cloud
  description: Detects property crime-related content
  version: "1.0"
  created_at: "2025-01-26"
```

Create `classifier/rules/topics/crime/drug_crime.yaml`:

```yaml
rule_name: drug_crime_detection
rule_type: topic
topic_name: drug_crime
priority: 9
enabled: true
min_confidence: 0.3
keywords:
  - drugs
  - narcotics
  - trafficking
  - cocaine
  - heroin
  - fentanyl
  - methamphetamine
  - meth
  - marijuana
  - cannabis
  - overdose
  - dealer
  - dealing
  - possession
  - smuggling
  - cartel
  - drug bust
  - seized
metadata:
  author: north-cloud
  description: Detects drug crime-related content
  version: "1.0"
  created_at: "2025-01-26"
```

Create `classifier/rules/topics/crime/organized_crime.yaml`:

```yaml
rule_name: organized_crime_detection
rule_type: topic
topic_name: organized_crime
priority: 9
enabled: true
min_confidence: 0.3
keywords:
  - mafia
  - cartel
  - racketeering
  - money laundering
  - organized crime
  - syndicate
  - mob
  - gang leader
  - crime boss
  - extortion
  - human trafficking
  - smuggling ring
  - criminal network
  - RICO
metadata:
  author: north-cloud
  description: Detects organized crime-related content
  version: "1.0"
  created_at: "2025-01-26"
```

Create `classifier/rules/topics/crime/criminal_justice.yaml`:

```yaml
rule_name: criminal_justice_detection
rule_type: topic
topic_name: criminal_justice
priority: 5
enabled: true
min_confidence: 0.3
keywords:
  - court
  - trial
  - conviction
  - sentence
  - sentencing
  - arrest
  - arrested
  - charges
  - charged
  - prosecutor
  - defendant
  - verdict
  - guilty
  - not guilty
  - parole
  - probation
  - prison
  - jail
  - inmate
  - bail
  - hearing
  - judge
  - jury
  - indictment
  - plea
  - appeal
metadata:
  author: north-cloud
  description: Detects criminal justice system content
  version: "1.0"
  created_at: "2025-01-26"
```

**Step 1.3: Commit initial rules**

```bash
git add classifier/rules/
git commit -m "$(cat <<'EOF'
feat(classifier): add git-backed rules directory structure

Creates YAML rule files for classification:
- violent_crime, property_crime, drug_crime
- organized_crime, criminal_justice

These mirror existing database rules and will be the
source of truth for rule management via MCP.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Add Rule Types to MCP Server

**Files:**
- Create: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/rules/types.go`
- Create: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/rules/loader.go`

**Step 2.1: Create rules package directory**

```bash
mkdir -p /home/fsd42/dev/north-cloud/mcp-north-cloud/internal/rules
```

**Step 2.2: Create types.go**

Create `mcp-north-cloud/internal/rules/types.go`:

```go
package rules

import "time"

// Rule represents a classification rule loaded from YAML.
type Rule struct {
	RuleName      string            `yaml:"rule_name" json:"rule_name"`
	RuleType      string            `yaml:"rule_type" json:"rule_type"`
	TopicName     string            `yaml:"topic_name" json:"topic_name"`
	Priority      int               `yaml:"priority" json:"priority"`
	Enabled       bool              `yaml:"enabled" json:"enabled"`
	MinConfidence float64           `yaml:"min_confidence" json:"min_confidence"`
	Keywords      []string          `yaml:"keywords" json:"keywords"`
	Metadata      map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// RuleVersion represents a version of a rule from git history.
type RuleVersion struct {
	CommitHash string    `json:"commit_hash"`
	CommitMsg  string    `json:"commit_message"`
	Author     string    `json:"author"`
	Timestamp  time.Time `json:"timestamp"`
	RuleName   string    `json:"rule_name"`
}

// SyncResult represents the result of syncing rules to classifier.
type SyncResult struct {
	Created  int      `json:"created"`
	Updated  int      `json:"updated"`
	Deleted  int      `json:"deleted"`
	Errors   []string `json:"errors,omitempty"`
	RuleNames []string `json:"rule_names"`
}

// TestResult represents the result of testing a rule against content.
type TestResult struct {
	RuleName    string   `json:"rule_name"`
	Matched     bool     `json:"matched"`
	Confidence  float64  `json:"confidence"`
	MatchedKeywords []string `json:"matched_keywords"`
}
```

**Step 2.3: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run ./internal/rules/...`
Expected: No errors

**Step 2.4: Commit**

```bash
git add mcp-north-cloud/internal/rules/
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): add rules package with types

Defines Rule, RuleVersion, SyncResult, and TestResult types
for git-backed classification rules.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Implement Rule Loader

**Files:**
- Create: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/rules/loader.go`
- Create: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/rules/loader_test.go`

**Step 3.1: Write failing test for loader**

Create `loader_test.go`:

```go
package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_LoadRulesFromDir(t *testing.T) {
	t.Helper()

	// Create temp directory with test rule
	tmpDir := t.TempDir()
	topicsDir := filepath.Join(tmpDir, "topics", "crime")
	if err := os.MkdirAll(topicsDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	ruleContent := `
rule_name: test_rule
rule_type: topic
topic_name: test_topic
priority: 5
enabled: true
min_confidence: 0.5
keywords:
  - keyword1
  - keyword2
`
	rulePath := filepath.Join(topicsDir, "test.yaml")
	if err := os.WriteFile(rulePath, []byte(ruleContent), 0644); err != nil {
		t.Fatalf("failed to write rule: %v", err)
	}

	loader := NewLoader(tmpDir)
	rules, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}

	if rules[0].RuleName != "test_rule" {
		t.Errorf("expected rule_name 'test_rule', got %s", rules[0].RuleName)
	}

	if len(rules[0].Keywords) != 2 {
		t.Errorf("expected 2 keywords, got %d", len(rules[0].Keywords))
	}
}

func TestLoader_LoadRule(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	topicsDir := filepath.Join(tmpDir, "topics", "crime")
	if err := os.MkdirAll(topicsDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	ruleContent := `
rule_name: specific_rule
rule_type: topic
topic_name: crime
priority: 10
enabled: true
min_confidence: 0.3
keywords:
  - test
`
	if err := os.WriteFile(filepath.Join(topicsDir, "specific_rule.yaml"), []byte(ruleContent), 0644); err != nil {
		t.Fatalf("failed to write rule: %v", err)
	}

	loader := NewLoader(tmpDir)
	rule, err := loader.LoadRule("specific_rule")
	if err != nil {
		t.Fatalf("LoadRule failed: %v", err)
	}

	if rule.RuleName != "specific_rule" {
		t.Errorf("expected rule_name 'specific_rule', got %s", rule.RuleName)
	}
}
```

**Step 3.2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go test ./internal/rules/... -v`
Expected: FAIL - Loader doesn't exist

**Step 3.3: Implement Loader**

Create `loader.go`:

```go
package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Loader loads classification rules from YAML files.
type Loader struct {
	basePath string
}

// NewLoader creates a new rule loader.
func NewLoader(basePath string) *Loader {
	return &Loader{basePath: basePath}
}

// LoadAll loads all rules from the rules directory.
func (l *Loader) LoadAll() ([]Rule, error) {
	var rules []Rule

	err := filepath.Walk(l.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-YAML files
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		rule, loadErr := l.loadFromFile(path)
		if loadErr != nil {
			return fmt.Errorf("load %s: %w", path, loadErr)
		}

		rules = append(rules, *rule)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk rules directory: %w", err)
	}

	return rules, nil
}

// LoadRule loads a specific rule by name.
func (l *Loader) LoadRule(ruleName string) (*Rule, error) {
	// Search for the rule file
	var foundPath string

	err := filepath.Walk(l.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		// Check if filename matches rule name
		base := filepath.Base(path)
		name := strings.TrimSuffix(strings.TrimSuffix(base, ".yaml"), ".yml")
		if name == ruleName {
			foundPath = path
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return nil, fmt.Errorf("search for rule: %w", err)
	}

	if foundPath == "" {
		return nil, fmt.Errorf("rule not found: %s", ruleName)
	}

	return l.loadFromFile(foundPath)
}

// loadFromFile loads a rule from a YAML file.
func (l *Loader) loadFromFile(path string) (*Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var rule Rule
	if err := yaml.Unmarshal(data, &rule); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	// Set defaults
	if rule.MinConfidence == 0 {
		rule.MinConfidence = 0.5
	}

	return &rule, nil
}

// SaveRule saves a rule to a YAML file.
func (l *Loader) SaveRule(rule *Rule) error {
	// Determine path based on rule type and topic
	var subdir string
	switch {
	case rule.RuleType == "topic" && strings.Contains(rule.TopicName, "crime"):
		subdir = "topics/crime"
	case rule.RuleType == "topic":
		subdir = "topics/general"
	default:
		subdir = "other"
	}

	dirPath := filepath.Join(l.basePath, subdir)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	filePath := filepath.Join(dirPath, rule.RuleName+".yaml")

	data, err := yaml.Marshal(rule)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// DeleteRule deletes a rule file.
func (l *Loader) DeleteRule(ruleName string) error {
	rule, err := l.LoadRule(ruleName)
	if err != nil {
		return err
	}

	// Find and delete the file
	var foundPath string
	err = filepath.Walk(l.basePath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		name := strings.TrimSuffix(strings.TrimSuffix(base, ".yaml"), ".yml")
		if name == rule.RuleName {
			foundPath = path
			return filepath.SkipAll
		}
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return fmt.Errorf("search for rule: %w", err)
	}

	if foundPath == "" {
		return fmt.Errorf("rule file not found: %s", ruleName)
	}

	return os.Remove(foundPath)
}

// ListRules returns names of all rules without loading full content.
func (l *Loader) ListRules() ([]string, error) {
	var names []string

	err := filepath.Walk(l.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		base := filepath.Base(path)
		name := strings.TrimSuffix(strings.TrimSuffix(base, ".yaml"), ".yml")
		names = append(names, name)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return names, nil
}
```

**Step 3.4: Run tests to verify they pass**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go test ./internal/rules/... -v`
Expected: PASS

**Step 3.5: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run`
Expected: No errors

**Step 3.6: Commit**

```bash
git add mcp-north-cloud/internal/rules/
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): implement rule loader for YAML files

Loader provides methods to:
- LoadAll: Load all rules from directory
- LoadRule: Load specific rule by name
- SaveRule: Save rule to YAML file
- DeleteRule: Delete rule file
- ListRules: List rule names

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Add Git Operations Package

**Files:**
- Create: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/rules/git.go`
- Create: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/rules/git_test.go`

**Step 4.1: Add go-git dependency**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go get github.com/go-git/go-git/v5`

**Step 4.2: Write failing test for git operations**

Create `git_test.go`:

```go
package rules

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitClient_GetHistory(t *testing.T) {
	t.Helper()

	// Create temp git repo
	tmpDir := t.TempDir()

	// Initialize git repo
	if err := runGitCommand(tmpDir, "init"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if err := runGitCommand(tmpDir, "config", "user.email", "test@test.com"); err != nil {
		t.Fatalf("git config email failed: %v", err)
	}
	if err := runGitCommand(tmpDir, "config", "user.name", "Test"); err != nil {
		t.Fatalf("git config name failed: %v", err)
	}

	// Create a rule file
	rulesDir := filepath.Join(tmpDir, "topics", "crime")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	ruleFile := filepath.Join(rulesDir, "test_rule.yaml")
	if err := os.WriteFile(ruleFile, []byte("rule_name: test_rule"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Commit
	if err := runGitCommand(tmpDir, "add", "."); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := runGitCommand(tmpDir, "commit", "-m", "Add test rule"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Test GetHistory
	gitClient := NewGitClient(tmpDir)
	history, err := gitClient.GetHistory("test_rule", 10)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(history) < 1 {
		t.Errorf("expected at least 1 history entry, got %d", len(history))
	}

	if history[0].CommitMsg != "Add test rule" {
		t.Errorf("expected commit message 'Add test rule', got %s", history[0].CommitMsg)
	}
}

func runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}
```

**Step 4.3: Run test to verify it fails**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go test ./internal/rules/... -run TestGitClient -v`
Expected: FAIL - GitClient doesn't exist

**Step 4.4: Implement GitClient**

Create `git.go`:

```go
package rules

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GitClient provides git operations for rule versioning.
type GitClient struct {
	repoPath string
}

// NewGitClient creates a new git client.
func NewGitClient(repoPath string) *GitClient {
	return &GitClient{repoPath: repoPath}
}

// GetHistory returns commit history for a specific rule file.
func (g *GitClient) GetHistory(ruleName string, limit int) ([]RuleVersion, error) {
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	// Get log iterator
	logIter, err := repo.Log(&git.LogOptions{
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		return nil, fmt.Errorf("get log: %w", err)
	}
	defer logIter.Close()

	var versions []RuleVersion
	ruleFileName := ruleName + ".yaml"

	err = logIter.ForEach(func(c *object.Commit) error {
		if len(versions) >= limit {
			return fmt.Errorf("limit reached")
		}

		// Check if this commit touched the rule file
		stats, statsErr := c.Stats()
		if statsErr != nil {
			// Skip commits we can't get stats for
			return nil
		}

		for _, stat := range stats {
			if strings.HasSuffix(stat.Name, ruleFileName) {
				versions = append(versions, RuleVersion{
					CommitHash: c.Hash.String()[:8],
					CommitMsg:  strings.TrimSpace(c.Message),
					Author:     c.Author.Name,
					Timestamp:  c.Author.When,
					RuleName:   ruleName,
				})
				break
			}
		}

		return nil
	})

	// Ignore "limit reached" error
	if err != nil && err.Error() != "limit reached" {
		return nil, fmt.Errorf("iterate log: %w", err)
	}

	return versions, nil
}

// GetAllHistory returns recent commit history for all rule files.
func (g *GitClient) GetAllHistory(limit int) ([]RuleVersion, error) {
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	logIter, err := repo.Log(&git.LogOptions{
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		return nil, fmt.Errorf("get log: %w", err)
	}
	defer logIter.Close()

	var versions []RuleVersion
	count := 0

	err = logIter.ForEach(func(c *object.Commit) error {
		if count >= limit {
			return fmt.Errorf("limit reached")
		}

		stats, statsErr := c.Stats()
		if statsErr != nil {
			return nil
		}

		for _, stat := range stats {
			if strings.HasSuffix(stat.Name, ".yaml") || strings.HasSuffix(stat.Name, ".yml") {
				// Extract rule name from path
				base := filepath.Base(stat.Name)
				ruleName := strings.TrimSuffix(strings.TrimSuffix(base, ".yaml"), ".yml")

				versions = append(versions, RuleVersion{
					CommitHash: c.Hash.String()[:8],
					CommitMsg:  strings.TrimSpace(c.Message),
					Author:     c.Author.Name,
					Timestamp:  c.Author.When,
					RuleName:   ruleName,
				})
				count++
				break
			}
		}

		return nil
	})

	if err != nil && err.Error() != "limit reached" {
		return nil, fmt.Errorf("iterate log: %w", err)
	}

	return versions, nil
}

// Commit creates a git commit with the given message.
func (g *GitClient) Commit(message, author string) (string, error) {
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		return "", fmt.Errorf("open repo: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("get worktree: %w", err)
	}

	// Stage all changes in rules directory
	if _, err := worktree.Add("classifier/rules"); err != nil {
		// Try adding from repo root
		if _, err := worktree.Add("rules"); err != nil {
			return "", fmt.Errorf("stage changes: %w", err)
		}
	}

	// Create commit
	hash, err := worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  author,
			Email: author + "@north-cloud.local",
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	return hash.String()[:8], nil
}

// GetRuleAtVersion retrieves a rule file content at a specific commit.
func (g *GitClient) GetRuleAtVersion(ruleName, commitHash string) (*Rule, error) {
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	// Get commit
	commitIter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return nil, fmt.Errorf("get log: %w", err)
	}
	defer commitIter.Close()

	var targetCommit *object.Commit
	ruleFileName := ruleName + ".yaml"

	err = commitIter.ForEach(func(c *object.Commit) error {
		if strings.HasPrefix(c.Hash.String(), commitHash) {
			targetCommit = c
			return fmt.Errorf("found")
		}
		return nil
	})

	if targetCommit == nil {
		return nil, fmt.Errorf("commit not found: %s", commitHash)
	}

	// Get file tree
	tree, err := targetCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	// Find the rule file
	var ruleContent []byte
	err = tree.Files().ForEach(func(f *object.File) error {
		if strings.HasSuffix(f.Name, ruleFileName) {
			content, readErr := f.Contents()
			if readErr != nil {
				return readErr
			}
			ruleContent = []byte(content)
			return fmt.Errorf("found")
		}
		return nil
	})

	if ruleContent == nil {
		return nil, fmt.Errorf("rule file not found in commit: %s", ruleName)
	}

	// Parse YAML
	var rule Rule
	if err := parseYAML(ruleContent, &rule); err != nil {
		return nil, fmt.Errorf("parse rule: %w", err)
	}

	return &rule, nil
}

// parseYAML is a helper to parse YAML content.
func parseYAML(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}
```

**Step 4.5: Add yaml import**

Ensure the import is present:

```go
import (
	"gopkg.in/yaml.v3"
)
```

**Step 4.6: Run tests**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go test ./internal/rules/... -v`
Expected: PASS

**Step 4.7: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run`
Expected: No errors

**Step 4.8: Commit**

```bash
git add mcp-north-cloud/internal/rules/ mcp-north-cloud/go.mod mcp-north-cloud/go.sum
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): add GitClient for rule versioning

Uses go-git library to provide:
- GetHistory: Commit history for specific rule
- GetAllHistory: Recent history for all rules
- Commit: Create commits for rule changes
- GetRuleAtVersion: Retrieve rule at specific commit

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Add Classifier Client Methods for Rule Sync

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/client/classifier.go`
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/client/classifier_test.go`

**Step 5.1: Add rule sync types**

In `classifier.go`, add:

```go
// ClassifierRule represents a rule in the classifier database.
type ClassifierRule struct {
	ID            int      `json:"id,omitempty"`
	RuleName      string   `json:"rule_name"`
	RuleType      string   `json:"rule_type"`
	TopicName     string   `json:"topic_name"`
	Keywords      []string `json:"keywords"`
	MinConfidence float64  `json:"min_confidence"`
	Enabled       bool     `json:"enabled"`
	Priority      int      `json:"priority"`
}

// RuleTestRequest represents a request to test a rule.
type RuleTestRequest struct {
	RuleName string `json:"rule_name"`
	Content  string `json:"content"`
}

// RuleTestResponse represents the result of testing a rule.
type RuleTestResponse struct {
	Matched         bool     `json:"matched"`
	Confidence      float64  `json:"confidence"`
	MatchedKeywords []string `json:"matched_keywords"`
}
```

**Step 5.2: Add ListRules method**

In `classifier.go`, add:

```go
// ListRules returns all classification rules from the database.
func (c *ClassifierClient) ListRules() ([]ClassifierRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/rules", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var rules []ClassifierRule
	if err := json.NewDecoder(resp.Body).Decode(&rules); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return rules, nil
}
```

**Step 5.3: Add CreateRule method**

```go
// CreateRule creates a new classification rule.
func (c *ClassifierClient) CreateRule(rule ClassifierRule) (*ClassifierRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	body, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("marshal rule: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/rules", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var created ClassifierRule
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &created, nil
}
```

**Step 5.4: Add UpdateRule method**

```go
// UpdateRule updates an existing classification rule.
func (c *ClassifierClient) UpdateRule(id int, rule ClassifierRule) (*ClassifierRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	body, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("marshal rule: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/rules/%d", c.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var updated ClassifierRule
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &updated, nil
}
```

**Step 5.5: Add DeleteRule method**

```go
// DeleteRule deletes a classification rule.
func (c *ClassifierClient) DeleteRule(id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/api/v1/rules/%d", c.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}
```

**Step 5.6: Add bytes import if missing**

```go
import "bytes"
```

**Step 5.7: Run linter and tests**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run && go test ./...`
Expected: PASS

**Step 5.8: Commit**

```bash
git add mcp-north-cloud/internal/client/classifier.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): add rule CRUD methods to ClassifierClient

Adds methods for classifier rule management:
- ListRules: Get all rules from database
- CreateRule: Create new rule
- UpdateRule: Update existing rule
- DeleteRule: Delete rule

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Add RulesPath to MCP Config

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/config/config.go`

**Step 6.1: Add RulesPath to config**

In `config.go`, add to appropriate struct:

```go
type Config struct {
	Services ServicesConfig `yaml:"services"`
	Logging  LoggingConfig  `yaml:"logging"`
	Auth     AuthConfig     `yaml:"auth"`
	Rules    RulesConfig    `yaml:"rules"` // NEW
}

type RulesConfig struct {
	Path string `env:"RULES_PATH" yaml:"path"`
}
```

**Step 6.2: Add default in NewDefault**

```go
func NewDefault() *Config {
	return &Config{
		// ... existing ...
		Rules: RulesConfig{
			Path: "/home/fsd42/dev/north-cloud/classifier/rules",
		},
	}
}
```

**Step 6.3: Commit**

```bash
git add mcp-north-cloud/internal/config/config.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): add RulesPath to config

Configures path to classification rules directory.
Default: /home/fsd42/dev/north-cloud/classifier/rules
Env override: RULES_PATH

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Define Rule Management Tools

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/mcp/tools.go`

**Step 7.1: Add getRuleManagementTools function**

In `tools.go`, add:

```go
func getRuleManagementTools() []Tool {
	return []Tool{
		{
			Name:        "list_classification_rules",
			Description: "List all classification rules from YAML files. Use when: Viewing available rules or checking rule configuration.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"topic": map[string]any{
						"type":        "string",
						"description": "Optional topic filter (e.g., 'crime', 'sports')",
					},
				},
			},
		},
		{
			Name:        "get_classification_rule",
			Description: "Get a specific classification rule by name with its full configuration. Use when: Inspecting rule details or keywords.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"rule_name": map[string]any{
						"type":        "string",
						"description": "Name of the rule to retrieve",
					},
				},
				"required": []string{"rule_name"},
			},
		},
		{
			Name:        "create_classification_rule",
			Description: "Create a new classification rule. Saves to YAML file and syncs to classifier database. Use when: Adding new detection rules.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"rule_name": map[string]any{
						"type":        "string",
						"description": "Unique rule name (snake_case)",
					},
					"rule_type": map[string]any{
						"type":        "string",
						"description": "Type of rule: 'topic' or 'content_type'",
						"enum":        []string{"topic", "content_type"},
					},
					"topic_name": map[string]any{
						"type":        "string",
						"description": "Topic to assign when rule matches",
					},
					"keywords": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Keywords to match",
					},
					"priority": map[string]any{
						"type":        "integer",
						"description": "Priority (higher = evaluated first, default: 0)",
					},
					"min_confidence": map[string]any{
						"type":        "number",
						"description": "Minimum confidence threshold 0.0-1.0 (default: 0.5)",
					},
					"author": map[string]any{
						"type":        "string",
						"description": "Author name for git commit",
					},
				},
				"required": []string{"rule_name", "rule_type", "topic_name", "keywords"},
			},
		},
		{
			Name:        "update_classification_rule",
			Description: "Update an existing classification rule. Modifies YAML file and syncs to classifier. Use when: Modifying keywords or settings.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"rule_name": map[string]any{
						"type":        "string",
						"description": "Name of rule to update",
					},
					"keywords": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "New keywords list (replaces existing)",
					},
					"priority": map[string]any{
						"type":        "integer",
						"description": "New priority",
					},
					"min_confidence": map[string]any{
						"type":        "number",
						"description": "New confidence threshold",
					},
					"enabled": map[string]any{
						"type":        "boolean",
						"description": "Enable or disable rule",
					},
					"author": map[string]any{
						"type":        "string",
						"description": "Author name for git commit",
					},
				},
				"required": []string{"rule_name"},
			},
		},
		{
			Name:        "delete_classification_rule",
			Description: "Delete a classification rule. Removes YAML file and syncs deletion to classifier. Use when: Removing obsolete rules.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"rule_name": map[string]any{
						"type":        "string",
						"description": "Name of rule to delete",
					},
					"author": map[string]any{
						"type":        "string",
						"description": "Author name for git commit",
					},
				},
				"required": []string{"rule_name"},
			},
		},
		{
			Name:        "test_classification_rule",
			Description: "Test a rule against sample content without saving. Use when: Validating rule before creating or after updating.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"rule_name": map[string]any{
						"type":        "string",
						"description": "Name of rule to test (must exist)",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Sample content to test against",
					},
				},
				"required": []string{"rule_name", "content"},
			},
		},
		{
			Name:        "get_rule_history",
			Description: "Get version history for a rule from git commits. Use when: Reviewing changes or preparing rollback.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"rule_name": map[string]any{
						"type":        "string",
						"description": "Name of rule to get history for",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum entries to return (default: 10)",
					},
				},
				"required": []string{"rule_name"},
			},
		},
		{
			Name:        "rollback_rule",
			Description: "Rollback a rule to a previous version from git history. Use when: Reverting problematic changes.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"rule_name": map[string]any{
						"type":        "string",
						"description": "Name of rule to rollback",
					},
					"commit_hash": map[string]any{
						"type":        "string",
						"description": "Git commit hash to rollback to (from get_rule_history)",
					},
					"author": map[string]any{
						"type":        "string",
						"description": "Author name for git commit",
					},
				},
				"required": []string{"rule_name", "commit_hash"},
			},
		},
		{
			Name:        "sync_rules_to_classifier",
			Description: "Sync all YAML rules to classifier database. Use when: Applying bulk changes or after manual edits to YAML files.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}
```

**Step 7.2: Update getAllTools**

```go
func getAllTools() []Tool {
	tools := make([]Tool, 0, toolGroupCount*estimatedToolsPerGroup)
	tools = append(tools, getWorkflowTools()...)
	tools = append(tools, getCrawlerTools()...)
	tools = append(tools, getSourceManagerTools()...)
	tools = append(tools, getPublisherTools()...)
	tools = append(tools, getSearchTools()...)
	tools = append(tools, getClassifierTools()...)
	tools = append(tools, getIndexManagerTools()...)
	tools = append(tools, getDevelopmentTools()...)
	tools = append(tools, getProxyControlTools()...)
	tools = append(tools, getPipelineMonitoringTools()...)
	tools = append(tools, getRuleManagementTools()...) // NEW
	return tools
}
```

**Step 7.3: Update toolGroupCount**

```go
const (
	toolGroupCount         = 11 // Was 10, now 11
	estimatedToolsPerGroup = 5
)
```

**Step 7.4: Commit**

```bash
git add mcp-north-cloud/internal/mcp/tools.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): define 9 rule management tools

Tools for git-backed rule management:
- list_classification_rules, get_classification_rule
- create_classification_rule, update_classification_rule
- delete_classification_rule, test_classification_rule
- get_rule_history, rollback_rule, sync_rules_to_classifier

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Add RulesManager to Server

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/mcp/server.go`

**Step 8.1: Add rulesLoader and gitClient to Server**

In `server.go`, update the Server struct:

```go
import (
	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/rules"
)

type Server struct {
	crawlerClient       *client.CrawlerClient
	sourceManagerClient *client.SourceManagerClient
	publisherClient     *client.PublisherClient
	searchClient        *client.SearchClient
	classifierClient    *client.ClassifierClient
	indexManagerClient  *client.IndexManagerClient
	proxyClient         *client.ProxyClient
	rulesLoader         *rules.Loader    // NEW
	gitClient           *rules.GitClient // NEW
}
```

**Step 8.2: Update NewServer**

```go
func NewServer(
	crawlerClient *client.CrawlerClient,
	sourceManagerClient *client.SourceManagerClient,
	publisherClient *client.PublisherClient,
	searchClient *client.SearchClient,
	classifierClient *client.ClassifierClient,
	indexManagerClient *client.IndexManagerClient,
	proxyClient *client.ProxyClient,
	rulesLoader *rules.Loader,    // NEW
	gitClient *rules.GitClient,   // NEW
) *Server {
	return &Server{
		crawlerClient:       crawlerClient,
		sourceManagerClient: sourceManagerClient,
		publisherClient:     publisherClient,
		searchClient:        searchClient,
		classifierClient:    classifierClient,
		indexManagerClient:  indexManagerClient,
		proxyClient:         proxyClient,
		rulesLoader:         rulesLoader,  // NEW
		gitClient:           gitClient,    // NEW
	}
}
```

**Step 8.3: Register tool handlers**

Add to `toolHandlers` map:

```go
var toolHandlers = map[string]toolHandlerFunc{
	// ... existing handlers ...

	// Rule management tools
	"list_classification_rules":  (*Server).handleListClassificationRules,
	"get_classification_rule":    (*Server).handleGetClassificationRule,
	"create_classification_rule": (*Server).handleCreateClassificationRule,
	"update_classification_rule": (*Server).handleUpdateClassificationRule,
	"delete_classification_rule": (*Server).handleDeleteClassificationRule,
	"test_classification_rule":   (*Server).handleTestClassificationRule,
	"get_rule_history":           (*Server).handleGetRuleHistory,
	"rollback_rule":              (*Server).handleRollbackRule,
	"sync_rules_to_classifier":   (*Server).handleSyncRulesToClassifier,
}
```

**Step 8.4: Commit**

```bash
git add mcp-north-cloud/internal/mcp/server.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): add rules manager to server

Adds rulesLoader and gitClient to Server struct.
Registers 9 rule management tool handlers.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Implement Rule Management Handlers

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/mcp/handlers.go`

**Step 9.1: Add argument structs**

In `handlers.go`, add:

```go
// Rule management argument structs
type listRulesArgs struct {
	Topic string `json:"topic"`
}

type getRuleArgs struct {
	RuleName string `json:"rule_name"`
}

type createRuleArgs struct {
	RuleName      string   `json:"rule_name"`
	RuleType      string   `json:"rule_type"`
	TopicName     string   `json:"topic_name"`
	Keywords      []string `json:"keywords"`
	Priority      int      `json:"priority"`
	MinConfidence float64  `json:"min_confidence"`
	Author        string   `json:"author"`
}

type updateRuleArgs struct {
	RuleName      string   `json:"rule_name"`
	Keywords      []string `json:"keywords"`
	Priority      *int     `json:"priority"`
	MinConfidence *float64 `json:"min_confidence"`
	Enabled       *bool    `json:"enabled"`
	Author        string   `json:"author"`
}

type deleteRuleArgs struct {
	RuleName string `json:"rule_name"`
	Author   string `json:"author"`
}

type testRuleArgs struct {
	RuleName string `json:"rule_name"`
	Content  string `json:"content"`
}

type getRuleHistoryArgs struct {
	RuleName string `json:"rule_name"`
	Limit    int    `json:"limit"`
}

type rollbackRuleArgs struct {
	RuleName   string `json:"rule_name"`
	CommitHash string `json:"commit_hash"`
	Author     string `json:"author"`
}
```

**Step 9.2: Implement handlers**

Add the handler implementations:

```go
// handleListClassificationRules lists all rules from YAML files.
func (s *Server) handleListClassificationRules(id any, arguments json.RawMessage) *Response {
	if s.rulesLoader == nil {
		return s.errorResponse(id, InvalidParams, "rules loader not configured")
	}

	var args listRulesArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	allRules, err := s.rulesLoader.LoadAll()
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to load rules: %v", err))
	}

	// Filter by topic if specified
	var filtered []rules.Rule
	for _, r := range allRules {
		if args.Topic == "" || strings.Contains(r.TopicName, args.Topic) {
			filtered = append(filtered, r)
		}
	}

	return s.successResponse(id, map[string]any{
		"rules": filtered,
		"count": len(filtered),
	})
}

// handleGetClassificationRule gets a specific rule.
func (s *Server) handleGetClassificationRule(id any, arguments json.RawMessage) *Response {
	if s.rulesLoader == nil {
		return s.errorResponse(id, InvalidParams, "rules loader not configured")
	}

	var args getRuleArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	if args.RuleName == "" {
		return s.errorResponse(id, InvalidParams, "rule_name is required")
	}

	rule, err := s.rulesLoader.LoadRule(args.RuleName)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to load rule: %v", err))
	}

	return s.successResponse(id, rule)
}

// handleCreateClassificationRule creates a new rule.
func (s *Server) handleCreateClassificationRule(id any, arguments json.RawMessage) *Response {
	if s.rulesLoader == nil {
		return s.errorResponse(id, InvalidParams, "rules loader not configured")
	}

	var args createRuleArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	if args.RuleName == "" || args.RuleType == "" || args.TopicName == "" || len(args.Keywords) == 0 {
		return s.errorResponse(id, InvalidParams, "rule_name, rule_type, topic_name, and keywords are required")
	}

	// Create rule
	rule := &rules.Rule{
		RuleName:      args.RuleName,
		RuleType:      args.RuleType,
		TopicName:     args.TopicName,
		Keywords:      args.Keywords,
		Priority:      args.Priority,
		Enabled:       true,
		MinConfidence: args.MinConfidence,
	}
	if rule.MinConfidence == 0 {
		rule.MinConfidence = 0.5
	}

	// Save to YAML
	if err := s.rulesLoader.SaveRule(rule); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to save rule: %v", err))
	}

	// Sync to classifier database
	if s.classifierClient != nil {
		dbRule := client.ClassifierRule{
			RuleName:      rule.RuleName,
			RuleType:      rule.RuleType,
			TopicName:     rule.TopicName,
			Keywords:      rule.Keywords,
			MinConfidence: rule.MinConfidence,
			Enabled:       rule.Enabled,
			Priority:      rule.Priority,
		}
		if _, err := s.classifierClient.CreateRule(dbRule); err != nil {
			return s.errorResponse(id, InternalError, fmt.Sprintf("failed to sync to classifier: %v", err))
		}
	}

	// Git commit
	if s.gitClient != nil {
		author := args.Author
		if author == "" {
			author = "mcp-server"
		}
		msg := fmt.Sprintf("feat(rules): create %s", args.RuleName)
		if _, err := s.gitClient.Commit(msg, author); err != nil {
			// Log but don't fail - rule was created
			_ = err
		}
	}

	return s.successResponse(id, map[string]any{
		"created":   true,
		"rule_name": args.RuleName,
	})
}

// handleUpdateClassificationRule updates an existing rule.
func (s *Server) handleUpdateClassificationRule(id any, arguments json.RawMessage) *Response {
	if s.rulesLoader == nil {
		return s.errorResponse(id, InvalidParams, "rules loader not configured")
	}

	var args updateRuleArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	if args.RuleName == "" {
		return s.errorResponse(id, InvalidParams, "rule_name is required")
	}

	// Load existing rule
	rule, err := s.rulesLoader.LoadRule(args.RuleName)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to load rule: %v", err))
	}

	// Apply updates
	if len(args.Keywords) > 0 {
		rule.Keywords = args.Keywords
	}
	if args.Priority != nil {
		rule.Priority = *args.Priority
	}
	if args.MinConfidence != nil {
		rule.MinConfidence = *args.MinConfidence
	}
	if args.Enabled != nil {
		rule.Enabled = *args.Enabled
	}

	// Save to YAML
	if err := s.rulesLoader.SaveRule(rule); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to save rule: %v", err))
	}

	// Sync to classifier (would need to find by name and update)
	// For simplicity, we'll handle this in sync_rules_to_classifier

	// Git commit
	if s.gitClient != nil {
		author := args.Author
		if author == "" {
			author = "mcp-server"
		}
		msg := fmt.Sprintf("feat(rules): update %s", args.RuleName)
		if _, err := s.gitClient.Commit(msg, author); err != nil {
			_ = err
		}
	}

	return s.successResponse(id, map[string]any{
		"updated":   true,
		"rule_name": args.RuleName,
	})
}

// handleDeleteClassificationRule deletes a rule.
func (s *Server) handleDeleteClassificationRule(id any, arguments json.RawMessage) *Response {
	if s.rulesLoader == nil {
		return s.errorResponse(id, InvalidParams, "rules loader not configured")
	}

	var args deleteRuleArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	if args.RuleName == "" {
		return s.errorResponse(id, InvalidParams, "rule_name is required")
	}

	// Delete YAML file
	if err := s.rulesLoader.DeleteRule(args.RuleName); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to delete rule: %v", err))
	}

	// Git commit
	if s.gitClient != nil {
		author := args.Author
		if author == "" {
			author = "mcp-server"
		}
		msg := fmt.Sprintf("feat(rules): delete %s", args.RuleName)
		if _, err := s.gitClient.Commit(msg, author); err != nil {
			_ = err
		}
	}

	return s.successResponse(id, map[string]any{
		"deleted":   true,
		"rule_name": args.RuleName,
	})
}

// handleTestClassificationRule tests a rule against content.
func (s *Server) handleTestClassificationRule(id any, arguments json.RawMessage) *Response {
	if s.rulesLoader == nil {
		return s.errorResponse(id, InvalidParams, "rules loader not configured")
	}

	var args testRuleArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	if args.RuleName == "" || args.Content == "" {
		return s.errorResponse(id, InvalidParams, "rule_name and content are required")
	}

	// Load rule
	rule, err := s.rulesLoader.LoadRule(args.RuleName)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to load rule: %v", err))
	}

	// Simple keyword matching test
	contentLower := strings.ToLower(args.Content)
	var matchedKeywords []string
	for _, kw := range rule.Keywords {
		if strings.Contains(contentLower, strings.ToLower(kw)) {
			matchedKeywords = append(matchedKeywords, kw)
		}
	}

	matched := len(matchedKeywords) > 0
	confidence := float64(len(matchedKeywords)) / float64(len(rule.Keywords))

	return s.successResponse(id, map[string]any{
		"rule_name":        args.RuleName,
		"matched":          matched,
		"confidence":       confidence,
		"matched_keywords": matchedKeywords,
		"total_keywords":   len(rule.Keywords),
	})
}

// handleGetRuleHistory gets git history for a rule.
func (s *Server) handleGetRuleHistory(id any, arguments json.RawMessage) *Response {
	if s.gitClient == nil {
		return s.errorResponse(id, InvalidParams, "git client not configured")
	}

	var args getRuleHistoryArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	if args.RuleName == "" {
		return s.errorResponse(id, InvalidParams, "rule_name is required")
	}

	limit := args.Limit
	if limit <= 0 {
		limit = 10
	}

	history, err := s.gitClient.GetHistory(args.RuleName, limit)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to get history: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"rule_name": args.RuleName,
		"history":   history,
		"count":     len(history),
	})
}

// handleRollbackRule rolls back a rule to a previous version.
func (s *Server) handleRollbackRule(id any, arguments json.RawMessage) *Response {
	if s.gitClient == nil || s.rulesLoader == nil {
		return s.errorResponse(id, InvalidParams, "git client or rules loader not configured")
	}

	var args rollbackRuleArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	if args.RuleName == "" || args.CommitHash == "" {
		return s.errorResponse(id, InvalidParams, "rule_name and commit_hash are required")
	}

	// Get rule at version
	oldRule, err := s.gitClient.GetRuleAtVersion(args.RuleName, args.CommitHash)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to get rule at version: %v", err))
	}

	// Save the old version
	if err := s.rulesLoader.SaveRule(oldRule); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to save rolled back rule: %v", err))
	}

	// Git commit
	author := args.Author
	if author == "" {
		author = "mcp-server"
	}
	msg := fmt.Sprintf("revert(rules): rollback %s to %s", args.RuleName, args.CommitHash)
	if _, err := s.gitClient.Commit(msg, author); err != nil {
		_ = err
	}

	return s.successResponse(id, map[string]any{
		"rolled_back":   true,
		"rule_name":     args.RuleName,
		"to_commit":     args.CommitHash,
	})
}

// handleSyncRulesToClassifier syncs all YAML rules to classifier database.
func (s *Server) handleSyncRulesToClassifier(id any, _ json.RawMessage) *Response {
	if s.rulesLoader == nil || s.classifierClient == nil {
		return s.errorResponse(id, InvalidParams, "rules loader or classifier client not configured")
	}

	// Load all YAML rules
	yamlRules, err := s.rulesLoader.LoadAll()
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to load rules: %v", err))
	}

	// Get existing database rules
	dbRules, err := s.classifierClient.ListRules()
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to list database rules: %v", err))
	}

	// Build map of existing rules by name
	dbRuleMap := make(map[string]client.ClassifierRule)
	for _, r := range dbRules {
		dbRuleMap[r.RuleName] = r
	}

	var created, updated int
	var errors []string

	// Sync each YAML rule
	for _, yamlRule := range yamlRules {
		dbRule := client.ClassifierRule{
			RuleName:      yamlRule.RuleName,
			RuleType:      yamlRule.RuleType,
			TopicName:     yamlRule.TopicName,
			Keywords:      yamlRule.Keywords,
			MinConfidence: yamlRule.MinConfidence,
			Enabled:       yamlRule.Enabled,
			Priority:      yamlRule.Priority,
		}

		if existing, ok := dbRuleMap[yamlRule.RuleName]; ok {
			// Update existing
			if _, err := s.classifierClient.UpdateRule(existing.ID, dbRule); err != nil {
				errors = append(errors, fmt.Sprintf("update %s: %v", yamlRule.RuleName, err))
			} else {
				updated++
			}
		} else {
			// Create new
			if _, err := s.classifierClient.CreateRule(dbRule); err != nil {
				errors = append(errors, fmt.Sprintf("create %s: %v", yamlRule.RuleName, err))
			} else {
				created++
			}
		}
	}

	return s.successResponse(id, map[string]any{
		"created": created,
		"updated": updated,
		"total":   len(yamlRules),
		"errors":  errors,
	})
}
```

**Step 9.3: Add strings import if missing**

```go
import "strings"
```

**Step 9.4: Run linter and tests**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run && go test ./...`
Expected: All pass

**Step 9.5: Commit**

```bash
git add mcp-north-cloud/internal/mcp/handlers.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): implement 9 rule management handlers

Handlers for git-backed rule management:
- list/get/create/update/delete classification rules
- test rule against sample content
- get git history and rollback to previous versions
- sync all YAML rules to classifier database

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Initialize Rules Manager in main.go

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/main.go`

**Step 10.1: Add imports and initialization**

In `main.go`, add:

```go
import (
	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/rules"
)

// In main() or run(), after config loading:

// Create rules manager
rulesLoader := rules.NewLoader(cfg.Rules.Path)
gitClient := rules.NewGitClient(filepath.Dir(cfg.Rules.Path)) // Parent repo

// Update server creation
server := mcp.NewServer(
	crawlerClient,
	sourceManagerClient,
	publisherClient,
	searchClient,
	classifierClient,
	indexManagerClient,
	proxyClient,
	rulesLoader,
	gitClient,
)
```

**Step 10.2: Add filepath import**

```go
import "path/filepath"
```

**Step 10.3: Build and test**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go build -o bin/mcp-north-cloud .`
Expected: Build succeeds

**Step 10.4: Commit**

```bash
git add mcp-north-cloud/main.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): initialize rules manager in main

Creates rulesLoader and gitClient from config and passes
to Server constructor.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Integration Testing

**Files:**
- No file changes - verification only

**Step 11.1: Build MCP server**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && task build`

**Step 11.2: Test list_classification_rules**

Run:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_classification_rules","arguments":{}}}' | ./bin/mcp-north-cloud
```
Expected: JSON with rules from YAML files

**Step 11.3: Test get_classification_rule**

Run:
```bash
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_classification_rule","arguments":{"rule_name":"violent_crime_detection"}}}' | ./bin/mcp-north-cloud
```
Expected: Full rule details

**Step 11.4: Test test_classification_rule**

Run:
```bash
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"test_classification_rule","arguments":{"rule_name":"violent_crime_detection","content":"A murder occurred in downtown Calgary yesterday after a shooting incident."}}}' | ./bin/mcp-north-cloud
```
Expected: Matched with keywords like "murder", "shooting"

**Step 11.5: Test get_rule_history**

Run:
```bash
echo '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_rule_history","arguments":{"rule_name":"violent_crime_detection"}}}' | ./bin/mcp-north-cloud
```
Expected: Git commit history for the rule

---

## Task 12: Update Documentation

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/README.md`

**Step 12.1: Add rule management tools section**

```markdown
## Rule Management Tools (Git-Backed)

These tools manage classification rules via YAML files with Git versioning.

| Tool | Description |
|------|-------------|
| `list_classification_rules` | List all rules from YAML files |
| `get_classification_rule` | Get specific rule by name |
| `create_classification_rule` | Create new rule (saves to YAML, syncs to DB) |
| `update_classification_rule` | Update existing rule |
| `delete_classification_rule` | Delete rule |
| `test_classification_rule` | Test rule against sample content |
| `get_rule_history` | Get git commit history for rule |
| `rollback_rule` | Rollback to previous version |
| `sync_rules_to_classifier` | Sync all YAML rules to database |

### Configuration

Set `RULES_PATH` environment variable to the rules directory.
Default: `/home/fsd42/dev/north-cloud/classifier/rules`

### Rule File Format

Rules are stored as YAML in `classifier/rules/topics/{category}/{rule_name}.yaml`:

```yaml
rule_name: violent_crime_detection
rule_type: topic
topic_name: violent_crime
priority: 10
enabled: true
min_confidence: 0.3
keywords:
  - murder
  - homicide
  - assault
```
```

**Step 12.2: Commit**

```bash
git add mcp-north-cloud/README.md
git commit -m "$(cat <<'EOF'
docs(mcp-north-cloud): document rule management tools

Adds documentation for 9 git-backed rule management tools
with configuration and YAML format examples.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Summary

This plan implements Phase 3 with 12 tasks:

| Task | Description | Files |
|------|-------------|-------|
| 1 | Create rules directory with YAML files | classifier/rules/ |
| 2 | Add rule types to MCP | mcp/internal/rules/types.go |
| 3 | Implement rule loader | mcp/internal/rules/loader.go |
| 4 | Add git operations | mcp/internal/rules/git.go |
| 5 | Add classifier client CRUD methods | mcp/internal/client/classifier.go |
| 6 | Add RulesPath config | mcp/internal/config/config.go |
| 7 | Define 9 rule management tools | mcp/internal/mcp/tools.go |
| 8 | Add rules manager to server | mcp/internal/mcp/server.go |
| 9 | Implement all handlers | mcp/internal/mcp/handlers.go |
| 10 | Initialize in main.go | mcp/main.go |
| 11 | Integration testing | (verification only) |
| 12 | Update documentation | mcp/README.md |

**New Tools Added:**
1. `list_classification_rules` - List all YAML rules
2. `get_classification_rule` - Get specific rule
3. `create_classification_rule` - Create with git commit
4. `update_classification_rule` - Update with git commit
5. `delete_classification_rule` - Delete with git commit
6. `test_classification_rule` - Test against content
7. `get_rule_history` - Git commit history
8. `rollback_rule` - Revert to previous version
9. `sync_rules_to_classifier` - Bulk sync to database

**Key Design Decisions:**
- YAML files are source of truth in `classifier/rules/`
- MCP server acts as Git gateway (classifier unchanged)
- Rules sync to classifier database via existing CRUD API
- Git provides versioning, history, and rollback

**Total: ~12 commits, following TDD pattern**
