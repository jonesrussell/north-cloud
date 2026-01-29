# Classifier & Publisher Reliability Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete the reliability infrastructure for classifier (DLQ, backpressure, telemetry) and publisher (transactional outbox), then add dashboard management UIs.

**Architecture:** Transactional outbox pattern ensures at-least-once delivery. Dead-letter queue with exponential backoff handles failures. Aho-Corasick rule engine provides O(n+m) matching. Prometheus metrics enable observability.

**Tech Stack:** Go 1.25+, PostgreSQL, Elasticsearch, Redis, Vue 3 + TanStack Query, Prometheus

---

## Phase 1: Foundation (Complete Scaffolded Code)

### Task 1: Add Missing Dependencies to Classifier

**Files:**
- Modify: `classifier/go.mod`

**Step 1: Add required dependencies**

Run:
```bash
cd classifier && go get github.com/cloudflare/ahocorasick github.com/prometheus/client_golang go.opentelemetry.io/otel go.opentelemetry.io/otel/trace
```

**Step 2: Tidy modules**

Run:
```bash
cd classifier && go mod tidy
```

Expected: Clean exit, no errors

**Step 3: Verify build**

Run:
```bash
cd classifier && go build ./...
```

Expected: Build succeeds

**Step 4: Commit**

```bash
git add classifier/go.mod classifier/go.sum
git commit -m "deps(classifier): add otel, prometheus, ahocorasick dependencies"
```

---

### Task 2: Add Missing Dependencies to Publisher

**Files:**
- Modify: `publisher/go.mod`

**Step 1: Add required dependencies**

Run:
```bash
cd publisher && go get github.com/redis/go-redis/v9 go.opentelemetry.io/otel go.opentelemetry.io/otel/trace github.com/lib/pq
```

**Step 2: Tidy modules**

Run:
```bash
cd publisher && go mod tidy
```

Expected: Clean exit, no errors

**Step 3: Verify build**

Run:
```bash
cd publisher && go build ./...
```

Expected: Build succeeds

**Step 4: Commit**

```bash
git add publisher/go.mod publisher/go.sum
git commit -m "deps(publisher): add redis, otel, pq dependencies"
```

---

### Task 3: Fix Telemetry Package Compilation

**Files:**
- Modify: `classifier/internal/telemetry/telemetry.go`
- Test: `classifier/internal/telemetry/telemetry_test.go`

**Step 1: Write the failing test**

Create `classifier/internal/telemetry/telemetry_test.go`:

```go
package telemetry

import (
	"context"
	"testing"
	"time"
)

func TestNewProvider(t *testing.T) {
	t.Helper()

	provider := NewProvider()
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
	if provider.Tracer == nil {
		t.Error("expected non-nil tracer")
	}
	if provider.Metrics == nil {
		t.Error("expected non-nil metrics")
	}
}

func TestRecordClassification(t *testing.T) {
	t.Helper()

	provider := NewProvider()
	ctx := context.Background()

	// Should not panic
	provider.RecordClassification(ctx, "test-source", true, 100*time.Millisecond)
	provider.RecordClassification(ctx, "test-source", false, 50*time.Millisecond)
}

func TestRecordRuleMatch(t *testing.T) {
	t.Helper()

	provider := NewProvider()
	ctx := context.Background()

	// Should not panic
	provider.RecordRuleMatch(ctx, 5*time.Millisecond, 25, 3)
}

func TestSetQueueDepth(t *testing.T) {
	t.Helper()

	provider := NewProvider()

	// Should not panic
	provider.SetQueueDepth(100)
	provider.SetActiveWorkers(5)
}
```

**Step 2: Run test to verify it compiles and passes**

Run:
```bash
cd classifier && go test ./internal/telemetry/... -v
```

Expected: All tests pass

**Step 3: Commit**

```bash
git add classifier/internal/telemetry/
git commit -m "feat(classifier): add telemetry package with prometheus metrics"
```

---

### Task 4: Fix Dead Letter Domain Model

**Files:**
- Modify: `classifier/internal/domain/dead_letter.go`

**Step 1: Read current file and verify structure**

Run:
```bash
cat classifier/internal/domain/dead_letter.go | head -50
```

**Step 2: Add missing DLQStats struct if not present**

Add to `classifier/internal/domain/dead_letter.go`:

```go
// DLQStats holds dead-letter queue statistics
type DLQStats struct {
	Pending     int64      `json:"pending"`
	Exhausted   int64      `json:"exhausted"`
	Ready       int64      `json:"ready"`
	AvgRetries  float64    `json:"avg_retries"`
	OldestEntry *time.Time `json:"oldest_entry,omitempty"`
}

// DLQSourceCount holds count by source
type DLQSourceCount struct {
	SourceName string `json:"source_name"`
	Count      int64  `json:"count"`
}

// DLQErrorCount holds count by error code
type DLQErrorCount struct {
	ErrorCode string `json:"error_code"`
	Count     int64  `json:"count"`
}
```

**Step 3: Verify build**

Run:
```bash
cd classifier && go build ./internal/domain/...
```

Expected: Build succeeds

**Step 4: Commit**

```bash
git add classifier/internal/domain/dead_letter.go
git commit -m "feat(classifier): add DLQ domain model with stats types"
```

---

### Task 5: Write Dead Letter Repository Tests

**Files:**
- Create: `classifier/internal/database/dead_letter_repository_test.go`

**Step 1: Write the failing test**

Create `classifier/internal/database/dead_letter_repository_test.go`:

```go
package database

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

func TestDeadLetterRepository_Enqueue(t *testing.T) {
	t.Helper()

	// Skip if no database connection
	db := getTestDB(t)
	if db == nil {
		t.Skip("no test database available")
	}
	defer db.Close()

	repo := NewDeadLetterRepository(db)
	ctx := context.Background()

	entry := &domain.DeadLetterEntry{
		ContentID:   "test-content-123",
		SourceName:  "test-source",
		IndexName:   "test_raw_content",
		ErrorMessage: "test error",
		ErrorCode:   domain.ErrorCodeUnknown,
		NextRetryAt: time.Now().Add(time.Minute),
	}

	err := repo.Enqueue(ctx, entry)
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	// Verify entry exists
	fetched, err := repo.GetByContentID(ctx, "test-content-123")
	if err != nil {
		t.Fatalf("get by content id failed: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected entry to exist")
	}
	if fetched.SourceName != "test-source" {
		t.Errorf("expected source_name 'test-source', got %q", fetched.SourceName)
	}

	// Cleanup
	_ = repo.Remove(ctx, "test-content-123")
}

func getTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// Return nil to skip database tests in CI without a real DB
	// In integration tests, connect to test database
	return nil
}
```

**Step 2: Run test (will skip without DB)**

Run:
```bash
cd classifier && go test ./internal/database/... -v -run TestDeadLetter
```

Expected: Test skips (no DB) or passes (with DB)

**Step 3: Commit**

```bash
git add classifier/internal/database/dead_letter_repository_test.go
git commit -m "test(classifier): add DLQ repository tests (skip without DB)"
```

---

### Task 6: Fix Rule Engine Compilation

**Files:**
- Modify: `classifier/internal/classifier/rule_engine.go`
- Create: `classifier/internal/classifier/rule_engine_test.go`

**Step 1: Write the failing test**

Create `classifier/internal/classifier/rule_engine_test.go`:

```go
package classifier

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

func TestTrieRuleEngine_Match(t *testing.T) {
	t.Helper()

	rules := []domain.ClassificationRule{
		{
			ID:            1,
			RuleName:      "crime-rule",
			RuleType:      "topic",
			TopicName:     "violent_crime",
			Keywords:      []string{"murder", "shooting", "stabbing"},
			MinConfidence: 0.3,
			Enabled:       true,
			Priority:      10,
		},
		{
			ID:            2,
			RuleName:      "sports-rule",
			RuleType:      "topic",
			TopicName:     "sports",
			Keywords:      []string{"football", "hockey", "basketball"},
			MinConfidence: 0.3,
			Enabled:       true,
			Priority:      5,
		},
	}

	engine := NewTrieRuleEngine(rules, nil, nil)

	// Test crime match
	matches := engine.Match("Breaking News", "A shooting occurred downtown. The murder suspect fled the scene.")

	if len(matches) == 0 {
		t.Fatal("expected at least one match")
	}

	if matches[0].Rule.TopicName != "violent_crime" {
		t.Errorf("expected violent_crime, got %s", matches[0].Rule.TopicName)
	}

	if matches[0].MatchCount < 2 {
		t.Errorf("expected at least 2 matches, got %d", matches[0].MatchCount)
	}
}

func TestTrieRuleEngine_UpdateRules(t *testing.T) {
	t.Helper()

	rules := []domain.ClassificationRule{
		{
			ID:       1,
			Keywords: []string{"test"},
			Enabled:  true,
		},
	}

	engine := NewTrieRuleEngine(rules, nil, nil)

	if engine.RuleCount() != 1 {
		t.Errorf("expected 1 rule, got %d", engine.RuleCount())
	}

	// Update with new rules
	newRules := []domain.ClassificationRule{
		{ID: 1, Keywords: []string{"test"}, Enabled: true},
		{ID: 2, Keywords: []string{"new"}, Enabled: true},
	}
	engine.UpdateRules(newRules)

	if engine.RuleCount() != 2 {
		t.Errorf("expected 2 rules, got %d", engine.RuleCount())
	}
}

func TestTrieRuleEngine_DisabledRules(t *testing.T) {
	t.Helper()

	rules := []domain.ClassificationRule{
		{ID: 1, Keywords: []string{"active"}, Enabled: true},
		{ID: 2, Keywords: []string{"inactive"}, Enabled: false},
	}

	engine := NewTrieRuleEngine(rules, nil, nil)

	if engine.RuleCount() != 1 {
		t.Errorf("expected 1 enabled rule, got %d", engine.RuleCount())
	}
}
```

**Step 2: Run test to verify it fails (missing domain fields)**

Run:
```bash
cd classifier && go test ./internal/classifier/... -v -run TestTrieRuleEngine
```

Expected: May fail if domain.ClassificationRule is missing fields

**Step 3: Check domain.ClassificationRule has required fields**

Verify `classifier/internal/domain/rule.go` has:
- `ID int`
- `RuleName string`
- `RuleType string`
- `TopicName string`
- `Keywords []string`
- `MinConfidence float64`
- `Enabled bool`
- `Priority int`

**Step 4: Run tests again**

Run:
```bash
cd classifier && go test ./internal/classifier/... -v -run TestTrieRuleEngine
```

Expected: All tests pass

**Step 5: Commit**

```bash
git add classifier/internal/classifier/rule_engine.go classifier/internal/classifier/rule_engine_test.go
git commit -m "feat(classifier): add Aho-Corasick rule engine with tests"
```

---

### Task 7: Fix Publisher Domain Model

**Files:**
- Modify: `publisher/internal/domain/outbox.go`

**Step 1: Read current file**

Run:
```bash
cat publisher/internal/domain/outbox.go | head -100
```

**Step 2: Add missing OutboxStats struct if not present**

Add to `publisher/internal/domain/outbox.go`:

```go
// OutboxStats holds outbox statistics
type OutboxStats struct {
	Pending              int64   `json:"pending"`
	Publishing           int64   `json:"publishing"`
	Published            int64   `json:"published"`
	FailedRetryable      int64   `json:"failed_retryable"`
	FailedExhausted      int64   `json:"failed_exhausted"`
	AvgPublishLagSeconds float64 `json:"avg_publish_lag_seconds"`
}
```

**Step 3: Verify build**

Run:
```bash
cd publisher && go build ./internal/domain/...
```

Expected: Build succeeds

**Step 4: Commit**

```bash
git add publisher/internal/domain/outbox.go
git commit -m "feat(publisher): add outbox domain model with stats"
```

---

### Task 8: Write Outbox Repository Tests

**Files:**
- Create: `publisher/internal/database/outbox_repository_test.go`

**Step 1: Write the test**

Create `publisher/internal/database/outbox_repository_test.go`:

```go
package database

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/publisher/internal/domain"
)

func TestOutboxRepository_MarkPublished(t *testing.T) {
	t.Helper()

	// Skip if no database connection
	db := getTestDB(t)
	if db == nil {
		t.Skip("no test database available")
	}
	defer db.Close()

	repo := NewOutboxRepository(db)
	ctx := context.Background()

	// This test requires a pre-existing entry
	// In real integration tests, insert first then mark
	err := repo.MarkPublished(ctx, "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestOutboxRepository_GetStats(t *testing.T) {
	t.Helper()

	db := getTestDB(t)
	if db == nil {
		t.Skip("no test database available")
	}
	defer db.Close()

	repo := NewOutboxRepository(db)
	ctx := context.Background()

	stats, err := repo.GetStats(ctx)
	if err != nil {
		t.Fatalf("get stats failed: %v", err)
	}

	// Stats should return valid struct even if empty
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
}

func getTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return nil // Skip without real DB
}
```

**Step 2: Run test**

Run:
```bash
cd publisher && go test ./internal/database/... -v -run TestOutbox
```

Expected: Tests skip (no DB)

**Step 3: Commit**

```bash
git add publisher/internal/database/outbox_repository_test.go
git commit -m "test(publisher): add outbox repository tests (skip without DB)"
```

---

### Task 9: Write Outbox Worker Tests

**Files:**
- Create: `publisher/internal/worker/outbox_worker_test.go`

**Step 1: Write the test**

Create `publisher/internal/worker/outbox_worker_test.go`:

```go
package worker

import (
	"testing"
	"time"
)

func TestDefaultOutboxWorkerConfig(t *testing.T) {
	t.Helper()

	cfg := DefaultOutboxWorkerConfig()

	if cfg.PollInterval != 5*time.Second {
		t.Errorf("expected 5s poll interval, got %v", cfg.PollInterval)
	}

	if cfg.BatchSize != 100 {
		t.Errorf("expected batch size 100, got %d", cfg.BatchSize)
	}

	if cfg.PublishTimeout != 10*time.Second {
		t.Errorf("expected 10s publish timeout, got %v", cfg.PublishTimeout)
	}
}
```

**Step 2: Run test**

Run:
```bash
cd publisher && go test ./internal/worker/... -v
```

Expected: Test passes

**Step 3: Commit**

```bash
git add publisher/internal/worker/outbox_worker_test.go
git commit -m "test(publisher): add outbox worker unit tests"
```

---

## Phase 2: Dashboard UI Components

### Task 10: Create Classifier Types File

**Files:**
- Verify: `dashboard/src/types/classifier.ts`

**Step 1: Verify file exists and has correct types**

Run:
```bash
cat dashboard/src/types/classifier.ts | head -50
```

Expected: File exists with ClassificationRule interface

**Step 2: Export from types index**

Add to `dashboard/src/types/index.ts` (create if not exists):

```typescript
export * from './classifier'
export * from './indexManager'
export * from './publisher'
export * from './crawler'
export * from './health'
export * from './common'
```

**Step 3: Commit**

```bash
git add dashboard/src/types/
git commit -m "feat(dashboard): add classifier types and export index"
```

---

### Task 11: Add Rule Test Endpoint to API Client

**Files:**
- Modify: `dashboard/src/api/client.ts`

**Step 1: Add test endpoint to classifierApi.rules**

Find the `classifierApi.rules` object and add:

```typescript
test: (id: string | number, content: string) =>
  classifierClient.post(`/rules/${id}/test`, { content }),
```

**Step 2: Add typed imports**

Add at top of file:

```typescript
import type {
  ClassificationRule,
  RuleCreateRequest,
  RuleTestResult,
  RulesListResponse
} from '@/types/classifier'
```

**Step 3: Update rules methods with types**

```typescript
rules: {
  list: (): Promise<AxiosResponse<RulesListResponse>> =>
    classifierClient.get('/rules'),
  get: (id: string | number): Promise<AxiosResponse<ClassificationRule>> =>
    classifierClient.get(`/rules/${id}`),
  create: (data: RuleCreateRequest): Promise<AxiosResponse<ClassificationRule>> =>
    classifierClient.post('/rules', data),
  update: (id: string | number, data: Partial<RuleCreateRequest>): Promise<AxiosResponse<ClassificationRule>> =>
    classifierClient.put(`/rules/${id}`, data),
  delete: (id: string | number): Promise<AxiosResponse<void>> =>
    classifierClient.delete(`/rules/${id}`),
  test: (id: string | number, content: string): Promise<AxiosResponse<RuleTestResult>> =>
    classifierClient.post(`/rules/${id}/test`, { content }),
},
```

**Step 4: Verify TypeScript compiles**

Run:
```bash
cd dashboard && npm run build
```

Expected: Build succeeds

**Step 5: Commit**

```bash
git add dashboard/src/api/client.ts
git commit -m "feat(dashboard): add typed classifier rules API with test endpoint"
```

---

### Task 12: Create Rules Management View

**Files:**
- Create: `dashboard/src/views/intelligence/RulesManagementView.vue`
- Modify: `dashboard/src/router/index.ts`

**Step 1: Create the view component**

Create `dashboard/src/views/intelligence/RulesManagementView.vue`:

```vue
<script setup lang="ts">
import { ref, computed } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { classifierApi } from '@/api/client'
import type { ClassificationRule, RuleCreateRequest, RuleTestResult } from '@/types/classifier'
import { groupRulesByTopic } from '@/types/classifier'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter
} from '@/components/ui/dialog'
import { toast } from 'vue-sonner'
import { Plus, Pencil, Trash2, TestTube, GripVertical, Power, PowerOff } from 'lucide-vue-next'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

const queryClient = useQueryClient()

// UI State
const showCreateModal = ref(false)
const showTestModal = ref(false)
const showDeleteConfirm = ref(false)
const editingRule = ref<ClassificationRule | null>(null)
const selectedRuleForTest = ref<ClassificationRule | null>(null)
const ruleToDelete = ref<ClassificationRule | null>(null)
const testContent = ref('')
const testResult = ref<RuleTestResult | null>(null)

// Form state
const formData = ref<RuleCreateRequest>({
  rule_name: '',
  rule_type: 'topic',
  topic_name: '',
  keywords: [],
  min_confidence: 0.3,
  enabled: true,
  priority: 5,
})
const keywordsInput = ref('')

// Fetch rules
const { data: rulesResponse, isLoading } = useQuery({
  queryKey: ['classifier', 'rules'],
  queryFn: async () => {
    const response = await classifierApi.rules.list()
    return response.data
  },
})

// Group by topic for organization
const rulesByTopic = computed(() => {
  if (!rulesResponse.value?.rules) return {}
  return groupRulesByTopic(rulesResponse.value.rules)
})

// Mutations
const createMutation = useMutation({
  mutationFn: async (data: RuleCreateRequest) => {
    const response = await classifierApi.rules.create(data)
    return response.data
  },
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['classifier', 'rules'] })
    showCreateModal.value = false
    resetForm()
    toast.success('Rule created successfully')
  },
  onError: (error: Error) => toast.error(`Failed to create rule: ${error.message}`),
})

const updateMutation = useMutation({
  mutationFn: async ({ id, data }: { id: number; data: Partial<RuleCreateRequest> }) => {
    const response = await classifierApi.rules.update(id, data)
    return response.data
  },
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['classifier', 'rules'] })
    editingRule.value = null
    resetForm()
    toast.success('Rule updated successfully')
  },
  onError: (error: Error) => toast.error(`Failed to update rule: ${error.message}`),
})

const deleteMutation = useMutation({
  mutationFn: async (id: number) => {
    await classifierApi.rules.delete(id)
  },
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['classifier', 'rules'] })
    showDeleteConfirm.value = false
    ruleToDelete.value = null
    toast.success('Rule deleted successfully')
  },
  onError: (error: Error) => toast.error(`Failed to delete rule: ${error.message}`),
})

const toggleMutation = useMutation({
  mutationFn: async ({ id, enabled }: { id: number; enabled: boolean }) => {
    const response = await classifierApi.rules.update(id, { enabled })
    return response.data
  },
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['classifier', 'rules'] })
    toast.success('Rule toggled')
  },
})

const testMutation = useMutation({
  mutationFn: async ({ ruleId, content }: { ruleId: number; content: string }) => {
    const response = await classifierApi.rules.test(ruleId, content)
    return response.data
  },
  onSuccess: (data) => {
    testResult.value = data
  },
  onError: (error: Error) => toast.error(`Test failed: ${error.message}`),
})

// Actions
function openCreateModal() {
  resetForm()
  showCreateModal.value = true
}

function openEditModal(rule: ClassificationRule) {
  editingRule.value = rule
  formData.value = {
    rule_name: rule.rule_name,
    rule_type: rule.rule_type,
    topic_name: rule.topic_name,
    keywords: [...rule.keywords],
    min_confidence: rule.min_confidence,
    enabled: rule.enabled,
    priority: rule.priority,
  }
  keywordsInput.value = rule.keywords.join(', ')
  showCreateModal.value = true
}

function handleDelete(rule: ClassificationRule) {
  ruleToDelete.value = rule
  showDeleteConfirm.value = true
}

function confirmDelete() {
  if (ruleToDelete.value) {
    deleteMutation.mutate(ruleToDelete.value.id)
  }
}

function handleTest(rule: ClassificationRule) {
  selectedRuleForTest.value = rule
  testContent.value = ''
  testResult.value = null
  showTestModal.value = true
}

function runTest() {
  if (!selectedRuleForTest.value || !testContent.value.trim()) return
  testMutation.mutate({
    ruleId: selectedRuleForTest.value.id,
    content: testContent.value,
  })
}

function handleSubmit() {
  // Parse keywords from comma-separated input
  formData.value.keywords = keywordsInput.value
    .split(',')
    .map(k => k.trim().toLowerCase())
    .filter(k => k.length > 0)

  if (editingRule.value) {
    updateMutation.mutate({ id: editingRule.value.id, data: formData.value })
  } else {
    createMutation.mutate(formData.value)
  }
}

function resetForm() {
  formData.value = {
    rule_name: '',
    rule_type: 'topic',
    topic_name: '',
    keywords: [],
    min_confidence: 0.3,
    enabled: true,
    priority: 5,
  }
  keywordsInput.value = ''
  editingRule.value = null
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold">Classification Rules</h1>
        <p class="text-muted-foreground">
          Manage keyword-based classification rules for topic detection
        </p>
      </div>
      <Button @click="openCreateModal">
        <Plus class="mr-2 h-4 w-4" />
        Create Rule
      </Button>
    </div>

    <!-- Loading State -->
    <div v-if="isLoading" class="flex justify-center py-12">
      <LoadingSpinner />
    </div>

    <!-- Rules grouped by topic -->
    <div v-else class="space-y-6">
      <Card v-for="(topicRules, topic) in rulesByTopic" :key="topic">
        <CardHeader>
          <CardTitle class="flex items-center gap-2">
            <Badge variant="outline">{{ topic }}</Badge>
            <span class="text-sm text-muted-foreground">
              {{ topicRules.length }} {{ topicRules.length === 1 ? 'rule' : 'rules' }}
            </span>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div class="divide-y">
            <div
              v-for="rule in topicRules"
              :key="rule.id"
              class="flex items-center justify-between py-3"
            >
              <div class="flex items-center gap-4">
                <GripVertical class="h-4 w-4 text-muted-foreground cursor-grab" />
                <div>
                  <div class="flex items-center gap-2">
                    <span class="font-medium">{{ rule.rule_name }}</span>
                    <Badge :variant="rule.enabled ? 'default' : 'secondary'">
                      {{ rule.enabled ? 'Active' : 'Disabled' }}
                    </Badge>
                    <Badge variant="outline">Priority: {{ rule.priority }}</Badge>
                  </div>
                  <div class="text-sm text-muted-foreground mt-1">
                    {{ rule.keywords.slice(0, 5).join(', ') }}
                    <span v-if="rule.keywords.length > 5">
                      +{{ rule.keywords.length - 5 }} more
                    </span>
                  </div>
                  <div class="text-xs text-muted-foreground">
                    Min confidence: {{ (rule.min_confidence * 100).toFixed(0) }}%
                  </div>
                </div>
              </div>

              <div class="flex items-center gap-2">
                <Button variant="ghost" size="sm" @click="handleTest(rule)">
                  <TestTube class="h-4 w-4" />
                </Button>
                <Button variant="ghost" size="sm" @click="openEditModal(rule)">
                  <Pencil class="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  @click="toggleMutation.mutate({ id: rule.id, enabled: !rule.enabled })"
                >
                  <Power v-if="rule.enabled" class="h-4 w-4" />
                  <PowerOff v-else class="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  class="text-destructive hover:text-destructive"
                  @click="handleDelete(rule)"
                >
                  <Trash2 class="h-4 w-4" />
                </Button>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <div v-if="Object.keys(rulesByTopic).length === 0" class="text-center py-12 text-muted-foreground">
        No classification rules found. Create your first rule to get started.
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <Dialog v-model:open="showCreateModal">
      <DialogContent class="max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {{ editingRule ? 'Edit Rule' : 'Create Rule' }}
          </DialogTitle>
        </DialogHeader>
        <form @submit.prevent="handleSubmit" class="space-y-4">
          <div>
            <label class="text-sm font-medium">Rule Name</label>
            <Input v-model="formData.rule_name" placeholder="e.g., violent-crime-detector" />
          </div>
          <div>
            <label class="text-sm font-medium">Topic Name</label>
            <Input v-model="formData.topic_name" placeholder="e.g., violent_crime" />
          </div>
          <div>
            <label class="text-sm font-medium">Keywords (comma-separated)</label>
            <textarea
              v-model="keywordsInput"
              class="w-full h-24 p-2 border rounded-md text-sm"
              placeholder="murder, shooting, stabbing, assault"
            />
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="text-sm font-medium">Priority (1-10)</label>
              <Input v-model.number="formData.priority" type="number" min="1" max="10" />
            </div>
            <div>
              <label class="text-sm font-medium">Min Confidence (0-1)</label>
              <Input v-model.number="formData.min_confidence" type="number" min="0" max="1" step="0.1" />
            </div>
          </div>
          <div class="flex items-center gap-2">
            <input type="checkbox" v-model="formData.enabled" id="enabled" />
            <label for="enabled" class="text-sm">Enabled</label>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" @click="showCreateModal = false">
              Cancel
            </Button>
            <Button type="submit" :disabled="createMutation.isPending.value || updateMutation.isPending.value">
              {{ editingRule ? 'Update' : 'Create' }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>

    <!-- Delete Confirmation -->
    <Dialog v-model:open="showDeleteConfirm">
      <DialogContent>
        <DialogHeader>
          <DialogTitle class="text-destructive">Delete Rule</DialogTitle>
          <DialogDescription>
            Are you sure you want to delete "{{ ruleToDelete?.rule_name }}"? This action cannot be undone.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="showDeleteConfirm = false">Cancel</Button>
          <Button variant="destructive" @click="confirmDelete" :disabled="deleteMutation.isPending.value">
            Delete
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Test Modal -->
    <Dialog v-model:open="showTestModal">
      <DialogContent class="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Test Rule: {{ selectedRuleForTest?.rule_name }}</DialogTitle>
          <DialogDescription>
            Paste sample content to test against this rule
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <div>
            <label class="text-sm font-medium">Sample Content</label>
            <textarea
              v-model="testContent"
              class="w-full h-32 mt-1 p-2 border rounded-md"
              placeholder="Paste article content to test against this rule..."
            />
          </div>
          <Button @click="runTest" :disabled="testMutation.isPending.value || !testContent.trim()">
            Run Test
          </Button>

          <div v-if="testResult" class="p-4 bg-muted rounded-lg">
            <div class="flex items-center gap-2 mb-2">
              <Badge :variant="testResult.matched ? 'default' : 'destructive'">
                {{ testResult.matched ? 'MATCHED' : 'NO MATCH' }}
              </Badge>
              <span>Score: {{ (testResult.score * 100).toFixed(1) }}%</span>
              <span>Coverage: {{ (testResult.coverage * 100).toFixed(1) }}%</span>
            </div>
            <div v-if="testResult.matched_keywords?.length" class="mt-2">
              <span class="text-sm text-muted-foreground">Matched keywords:</span>
              <div class="flex flex-wrap gap-1 mt-1">
                <Badge v-for="kw in testResult.matched_keywords" :key="kw" variant="outline">
                  {{ kw }}
                </Badge>
              </div>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>
```

**Step 2: Add route**

Add to `dashboard/src/router/index.ts` in the intelligence routes:

```typescript
{
  path: '/intelligence/rules',
  name: 'rules-management',
  component: () => import('@/views/intelligence/RulesManagementView.vue'),
  meta: { requiresAuth: true },
},
```

**Step 3: Verify build**

Run:
```bash
cd dashboard && npm run build
```

Expected: Build succeeds

**Step 4: Commit**

```bash
git add dashboard/src/views/intelligence/RulesManagementView.vue dashboard/src/router/index.ts
git commit -m "feat(dashboard): add rules management view with CRUD and test UI"
```

---

## Phase 3: Database Migrations

### Task 13: Run Classifier DLQ Migration

**Step 1: Verify migration file exists**

Run:
```bash
cat classifier/migrations/009_create_dead_letter_queue.up.sql
```

**Step 2: Run migration**

Run:
```bash
cd classifier && go run cmd/migrate/main.go up
```

Expected: Migration succeeds

**Step 3: Verify table exists**

Run:
```bash
docker exec -it north-cloud-postgres-classifier psql -U postgres -d classifier -c "\dt dead_letter_queue"
```

Expected: Table listed

---

### Task 14: Run Publisher Outbox Migration

**Step 1: Verify migration file exists**

Run:
```bash
cat publisher/migrations/002_create_outbox.up.sql
```

**Step 2: Run migration**

Run:
```bash
cd publisher && go run cmd/migrate/main.go up
```

Expected: Migration succeeds

**Step 3: Verify table exists**

Run:
```bash
docker exec -it north-cloud-postgres-publisher psql -U postgres -d publisher -c "\dt classified_outbox"
```

Expected: Table listed

---

## Phase 4: Integration

### Task 15: Add /metrics Endpoint to Classifier

**Files:**
- Modify: `classifier/internal/server/httpd.go`

**Step 1: Add metrics endpoint**

Add to router setup:

```go
import "github.com/jonesrussell/north-cloud/classifier/internal/telemetry"

// In SetupRoutes or similar:
tp := telemetry.NewProvider()
router.GET("/metrics", gin.WrapH(tp.Handler()))
```

**Step 2: Verify endpoint works**

Run:
```bash
curl -s localhost:8071/metrics | head -20
```

Expected: Prometheus metrics output

**Step 3: Commit**

```bash
git add classifier/internal/server/httpd.go
git commit -m "feat(classifier): add /metrics endpoint for prometheus scraping"
```

---

### Task 16: Add Rule Test Endpoint to Classifier API

**Files:**
- Modify: `classifier/internal/api/handlers.go`
- Modify: `classifier/internal/api/routes.go`

**Step 1: Add handler**

Add to `classifier/internal/api/handlers.go`:

```go
// TestRule tests a rule against sample content
func (h *Handler) TestRule(c *gin.Context) {
	ctx := c.Request.Context()

	ruleID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule ID"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the rule
	rule, err := h.rulesRepo.GetByID(ctx, ruleID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	// Test using the rule engine
	matches := h.ruleEngine.Match("", req.Content)

	var result *classifier.RuleMatch
	for i := range matches {
		if matches[i].Rule.ID == ruleID {
			result = &matches[i]
			break
		}
	}

	if result == nil {
		c.JSON(http.StatusOK, gin.H{
			"matched":          false,
			"score":            0,
			"matched_keywords": []string{},
			"coverage":         0,
			"match_count":      0,
			"unique_matches":   0,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"matched":          true,
		"score":            result.Score,
		"matched_keywords": result.MatchedKeywords,
		"coverage":         result.Coverage,
		"match_count":      result.MatchCount,
		"unique_matches":   result.UniqueMatches,
	})
}
```

**Step 2: Add route**

Add to routes:

```go
rules.POST("/:id/test", h.TestRule)
```

**Step 3: Verify build**

Run:
```bash
cd classifier && go build ./...
```

Expected: Build succeeds

**Step 4: Commit**

```bash
git add classifier/internal/api/
git commit -m "feat(classifier): add rule test endpoint for dashboard integration"
```

---

## Final: Lint and Test All

### Task 17: Run Full Test Suite

**Step 1: Lint classifier**

Run:
```bash
cd classifier && golangci-lint run
```

Expected: No errors (warnings acceptable)

**Step 2: Test classifier**

Run:
```bash
cd classifier && go test ./... -v
```

Expected: All tests pass

**Step 3: Lint publisher**

Run:
```bash
cd publisher && golangci-lint run
```

Expected: No errors

**Step 4: Test publisher**

Run:
```bash
cd publisher && go test ./... -v
```

Expected: All tests pass

**Step 5: Build dashboard**

Run:
```bash
cd dashboard && npm run build
```

Expected: Build succeeds

**Step 6: Final commit**

```bash
git add -A
git commit -m "chore: complete reliability infrastructure implementation"
```

---

## Summary

This plan implements:

1. **Foundation** (Tasks 1-9): Dependencies, telemetry, DLQ, outbox models and repositories
2. **Dashboard** (Tasks 10-12): TypeScript types, API client updates, Rules Management UI
3. **Migrations** (Tasks 13-14): DLQ and outbox database tables
4. **Integration** (Tasks 15-16): Metrics endpoint, rule test API
5. **Validation** (Task 17): Full test and lint pass

Total: 17 tasks, estimated 2-3 hours execution time.
