# Classification Dashboard Phase 1: Operator Essentials — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Give operators immediate visibility into crime-ml and mining-ml services: health status, mining content breakdowns, hybrid decision audit on documents, and pipeline mode indicators.

**Architecture:** Four features across two services (index-manager for mining aggregations, dashboard for all UI). The classifier service already exposes ML health via its `/health` endpoint — we add a new `/api/v1/metrics/ml-health` endpoint for richer data. The dashboard gets a new MiningBreakdownView mirroring CrimeBreakdownView, a Classifier Health widget, a Hybrid Decision Audit panel on document detail, and a Pipeline Mode Indicator.

**Tech Stack:** Go 1.25+ (classifier, index-manager), Vue 3 + TypeScript + Tailwind CSS 4 (dashboard), Elasticsearch aggregations, Gin HTTP framework.

**Design doc:** `docs/plans/2026-02-05-classification-dashboard-redesign.md`

---

## Task 1: Add mining object to Elasticsearch mapping

The ES mapping for `classified_content` indexes is missing the `mining` object. The classifier writes mining data but ES currently maps it dynamically. We need explicit mapping.

**Files:**
- Modify: `index-manager/internal/elasticsearch/mappings/classified_content.go:77-170`

**Step 1: Add mining mapping to `getClassificationFields()`**

After the `"location"` block (line 147), add the mining object:

```go
// Nested mining object
"mining": map[string]any{
    "type": "object",
    "properties": map[string]any{
        "relevance": map[string]any{
            "type": "keyword",
        },
        "mining_stage": map[string]any{
            "type": "keyword",
        },
        "commodities": map[string]any{
            "type": "keyword",
        },
        "location": map[string]any{
            "type": "keyword",
        },
        "final_confidence": map[string]any{
            "type": "float",
        },
        "review_required": map[string]any{
            "type": "boolean",
        },
        "model_version": map[string]any{
            "type": "keyword",
        },
    },
},
```

**Step 2: Verify lint passes**

Run: `cd index-manager && golangci-lint run`
Expected: PASS (no errors)

**Step 3: Commit**

```bash
git add index-manager/internal/elasticsearch/mappings/classified_content.go
git commit -m "feat(index-manager): add mining object to classified_content ES mapping"
```

**Note:** Existing indexes won't pick up the new mapping automatically. New indexes created after this will have it. For existing indexes, ES dynamic mapping will still work — this just makes it explicit.

---

## Task 2: Add MiningAggregation domain type (index-manager)

**Files:**
- Modify: `index-manager/internal/domain/aggregation.go`

**Step 1: Add MiningAggregation type**

After the `CrimeAggregation` struct (line 10), add:

```go
// MiningAggregation represents mining distribution statistics
type MiningAggregation struct {
	ByRelevance    map[string]int64 `json:"by_relevance"`
	ByMiningStage  map[string]int64 `json:"by_mining_stage"`
	ByCommodity    map[string]int64 `json:"by_commodity"`
	ByLocation     map[string]int64 `json:"by_location"`
	TotalMining    int64            `json:"total_mining"`
	TotalDocuments int64            `json:"total_documents"`
}
```

**Step 2: Verify lint passes**

Run: `cd index-manager && golangci-lint run`
Expected: PASS

**Step 3: Commit**

```bash
git add index-manager/internal/domain/aggregation.go
git commit -m "feat(index-manager): add MiningAggregation domain type"
```

---

## Task 3: Add GetMiningAggregation service method (index-manager)

**Files:**
- Modify: `index-manager/internal/service/aggregation_service.go`

**Step 1: Add constants for mining aggregation limits**

After line 18 (existing constants block), add:

```go
const (
	topMiningTypesLimit    = 10
	topCommoditiesLimit    = 10
)
```

Wait — these can reuse existing constants. The `topCitiesLimit = 10` and `topCrimeTypesLimit = 10` are already defined. Use those.

**Step 1: Add GetMiningAggregation method**

After the `GetOverviewAggregation` method (after line 210), add:

```go
// GetMiningAggregation returns mining distribution statistics
func (s *AggregationService) GetMiningAggregation(
	ctx context.Context,
	req *domain.AggregationRequest,
) (*domain.MiningAggregation, error) {
	query := s.buildAggregationQuery(req, map[string]any{
		"by_relevance": map[string]any{
			"terms": map[string]any{
				"field": "mining.relevance",
				"size":  topCitiesLimit,
			},
		},
		"by_mining_stage": map[string]any{
			"terms": map[string]any{
				"field": "mining.mining_stage",
				"size":  topCitiesLimit,
			},
		},
		"by_commodity": map[string]any{
			"terms": map[string]any{
				"field": "mining.commodities",
				"size":  topCrimeTypesLimit,
			},
		},
		"by_location": map[string]any{
			"terms": map[string]any{
				"field": "mining.location",
				"size":  topCitiesLimit,
			},
		},
		"mining_related": map[string]any{
			"filter": map[string]any{
				"terms": map[string]any{
					"mining.relevance": []string{"core_mining", "peripheral_mining"},
				},
			},
		},
	})

	res, err := s.esClient.SearchAllClassifiedContent(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute mining aggregation: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	var esResp aggregationResponse
	if decodeErr := json.NewDecoder(res.Body).Decode(&esResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode mining aggregation response: %w", decodeErr)
	}

	return &domain.MiningAggregation{
		ByRelevance:    extractBuckets(esResp.Aggregations["by_relevance"]),
		ByMiningStage:  extractBuckets(esResp.Aggregations["by_mining_stage"]),
		ByCommodity:    extractBuckets(esResp.Aggregations["by_commodity"]),
		ByLocation:     extractBuckets(esResp.Aggregations["by_location"]),
		TotalMining:    extractFilterCount(esResp.Aggregations["mining_related"]),
		TotalDocuments: esResp.Hits.Total.Value,
	}, nil
}
```

**Step 2: Verify lint passes**

Run: `cd index-manager && golangci-lint run`
Expected: PASS

**Step 3: Commit**

```bash
git add index-manager/internal/service/aggregation_service.go
git commit -m "feat(index-manager): add GetMiningAggregation service method"
```

---

## Task 4: Add mining aggregation handler and route (index-manager)

**Files:**
- Modify: `index-manager/internal/api/handlers.go` (after line 644)
- Modify: `index-manager/internal/api/routes.go` (line 47)

**Step 1: Add handler method**

After `GetOverviewAggregation` handler (line 644 in handlers.go), add:

```go
// GetMiningAggregation handles GET /api/v1/aggregations/mining
func (h *Handler) GetMiningAggregation(c *gin.Context) {
	req := h.parseAggregationRequest(c)

	result, err := h.aggregationService.GetMiningAggregation(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to get mining aggregation", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
```

**Step 2: Register route**

In `routes.go`, after line 47 (`aggregations.GET("/overview", ...)`), add:

```go
	aggregations.GET("/mining", handler.GetMiningAggregation)  // GET /api/v1/aggregations/mining
```

**Step 3: Verify lint passes**

Run: `cd index-manager && golangci-lint run`
Expected: PASS

**Step 4: Verify tests pass**

Run: `cd index-manager && go test ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add index-manager/internal/api/handlers.go index-manager/internal/api/routes.go
git commit -m "feat(index-manager): add mining aggregation endpoint"
```

---

## Task 5: Add ML health endpoint to classifier

The classifier needs a new endpoint that returns detailed ML service health information: reachability, model versions, latency, and pipeline mode.

**Files:**
- Modify: `classifier/internal/api/handlers.go`
- Modify: `classifier/internal/api/server.go`
- Modify: `classifier/internal/config/config.go` (read existing, may not need changes)

**Step 1: Read the classifier's ML client interfaces to understand health check patterns**

Check files:
- `classifier/internal/mlclient/client.go` — Crime ML client, look for Health method
- `classifier/internal/miningmlclient/client.go` — Mining ML client, look for Health method
- `classifier/internal/classifier/classifier.go` — Main orchestrator, understand how it holds ML references

**Step 2: Add MLHealthResponse type and handler**

In `handlers.go`, after the existing handler methods, add:

```go
// MLServiceHealth represents the health status of a single ML service
type MLServiceHealth struct {
	Reachable    bool   `json:"reachable"`
	ModelVersion string `json:"model_version,omitempty"`
	LatencyMs    int64  `json:"latency_ms,omitempty"`
	LastChecked  string `json:"last_checked_at"`
	Error        string `json:"error,omitempty"`
}

// MLHealthResponse represents the overall ML health status
type MLHealthResponse struct {
	CrimeML      *MLServiceHealth `json:"crime_ml,omitempty"`
	MiningML     *MLServiceHealth `json:"mining_ml,omitempty"`
	PipelineMode PipelineMode     `json:"pipeline_mode"`
}

// PipelineMode represents the current classifier pipeline configuration
type PipelineMode struct {
	Crime  string `json:"crime"`  // "hybrid", "rules-only", "disabled"
	Mining string `json:"mining"` // "hybrid", "rules-only", "disabled"
}
```

The handler needs access to the classifier config and ML clients. The Handler struct already has the `classifier` field which holds the orchestrator. We need to either:
- Pass config to the handler (add to Handler struct), or
- Access ML clients through the classifier

**Implementation approach:** Add `config` to the Handler struct so we can check `config.Classification.Crime.Enabled` etc. The handler will ping ML services directly via HTTP to check health.

Add to Handler struct:
```go
config *config.Config
```

Update NewHandler to accept config as last parameter before logger.

Then add the handler:
```go
// GetMLHealth handles GET /api/v1/metrics/ml-health
func (h *Handler) GetMLHealth(c *gin.Context) {
	resp := MLHealthResponse{
		PipelineMode: h.getPipelineMode(),
	}

	if h.config.Classification.Crime.Enabled {
		resp.CrimeML = h.checkMLServiceHealth(
			c.Request.Context(),
			h.config.Classification.Crime.MLServiceURL,
		)
	}

	if h.config.Classification.Mining.Enabled {
		resp.MiningML = h.checkMLServiceHealth(
			c.Request.Context(),
			h.config.Classification.Mining.MLServiceURL,
		)
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) getPipelineMode() PipelineMode {
	mode := PipelineMode{Crime: "disabled", Mining: "disabled"}

	if h.config.Classification.Crime.Enabled {
		if h.config.Classification.Crime.MLServiceURL != "" {
			mode.Crime = "hybrid"
		} else {
			mode.Crime = "rules-only"
		}
	}

	if h.config.Classification.Mining.Enabled {
		if h.config.Classification.Mining.MLServiceURL != "" {
			mode.Mining = "hybrid"
		} else {
			mode.Mining = "rules-only"
		}
	}

	return mode
}

func (h *Handler) checkMLServiceHealth(ctx context.Context, baseURL string) *MLServiceHealth {
	start := time.Now()
	health := &MLServiceHealth{
		LastChecked: time.Now().UTC().Format(time.RFC3339),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		health.Error = fmt.Sprintf("failed to create request: %v", err)
		return health
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		health.Error = fmt.Sprintf("service unreachable: %v", err)
		return health
	}
	defer func() { _ = resp.Body.Close() }()

	health.LatencyMs = time.Since(start).Milliseconds()

	if resp.StatusCode == http.StatusOK {
		health.Reachable = true
		// Parse model version from health response if available
		var healthResp struct {
			ModelVersion string `json:"model_version"`
		}
		if decodeErr := json.NewDecoder(resp.Body).Decode(&healthResp); decodeErr == nil {
			health.ModelVersion = healthResp.ModelVersion
		}
	} else {
		health.Error = fmt.Sprintf("unhealthy status: %d", resp.StatusCode)
	}

	return health
}
```

Note: You'll need to add `"encoding/json"` to imports if not already present.

**Step 3: Register route**

In `server.go`, after the `stats` group (line 85), add:

```go
	// Metrics endpoints
	metrics := v1.Group("/metrics")
	metrics.GET("/ml-health", handler.GetMLHealth) // GET /api/v1/metrics/ml-health
```

**Step 4: Update Handler constructor**

Update `NewHandler` signature to accept `cfg *config.Config` and store it in the struct. Update all callers (in bootstrap).

Check: `classifier/internal/bootstrap/` for where NewHandler is called.

**Step 5: Verify lint passes**

Run: `cd classifier && golangci-lint run`
Expected: PASS

**Step 6: Verify tests pass**

Run: `cd classifier && go test ./...`
Expected: PASS

**Step 7: Commit**

```bash
git add classifier/internal/api/handlers.go classifier/internal/api/server.go
git commit -m "feat(classifier): add ML health metrics endpoint"
```

---

## Task 6: Add MiningAggregation type to dashboard

**Files:**
- Modify: `dashboard/src/types/aggregation.ts`

**Step 1: Add MiningAggregation interface**

After `CrimeAggregation` (line 7), add:

```typescript
export interface MiningAggregation {
  by_relevance: Record<string, number>
  by_mining_stage: Record<string, number>
  by_commodity: Record<string, number>
  by_location: Record<string, number>
  total_mining: number
  total_documents: number
}
```

**Step 2: Add MLHealthResponse and PipelineMode types**

At the end of the file, add:

```typescript
export interface MLServiceHealth {
  reachable: boolean
  model_version?: string
  latency_ms?: number
  last_checked_at: string
  error?: string
}

export interface MLHealthResponse {
  crime_ml?: MLServiceHealth
  mining_ml?: MLServiceHealth
  pipeline_mode: PipelineMode
}

export interface PipelineMode {
  crime: 'hybrid' | 'rules-only' | 'disabled'
  mining: 'hybrid' | 'rules-only' | 'disabled'
}
```

**Step 3: Verify lint passes**

Run: `cd dashboard && npm run lint`
Expected: PASS

**Step 4: Commit**

```bash
git add dashboard/src/types/aggregation.ts
git commit -m "feat(dashboard): add MiningAggregation and MLHealth types"
```

---

## Task 7: Add mining aggregation and ML health API methods to dashboard client

**Files:**
- Modify: `dashboard/src/api/client.ts`

**Step 1: Update imports**

Add `MiningAggregation` and `MLHealthResponse` to the aggregation type imports (line 34-38):

```typescript
import type {
  CrimeAggregation,
  LocationAggregation,
  OverviewAggregation,
  AggregationFilters,
  MiningAggregation,
  MLHealthResponse,
} from '../types/aggregation'
```

**Step 2: Add getMining to indexManagerApi.aggregations**

After `getOverview` (line 504-507), add:

```typescript
    getMining: (filters?: AggregationFilters): Promise<AxiosResponse<MiningAggregation>> => {
      const params = buildAggregationParams(filters)
      return indexManagerClient.get('/api/v1/aggregations/mining', { params })
    },
```

**Step 3: Add metrics to classifierApi**

After the `stats` group in `classifierApi` (line 370), add:

```typescript
  // Metrics
  metrics: {
    mlHealth: (): Promise<AxiosResponse<MLHealthResponse>> =>
      classifierClient.get('/metrics/ml-health'),
  },
```

**Step 4: Verify lint passes**

Run: `cd dashboard && npm run lint`
Expected: PASS

**Step 5: Commit**

```bash
git add dashboard/src/api/client.ts
git commit -m "feat(dashboard): add mining aggregation and ML health API methods"
```

---

## Task 8: Create MiningBreakdownView

**Files:**
- Create: `dashboard/src/views/intelligence/MiningBreakdownView.vue`

**Step 1: Create the view**

Mirror `CrimeBreakdownView.vue` exactly, replacing crime-specific content with mining-specific content:

```vue
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Loader2, Pickaxe, RefreshCw, BarChart3, MapPin } from 'lucide-vue-next'
import { indexManagerApi } from '@/api/client'
import type { MiningAggregation } from '@/types/aggregation'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

const loading = ref(true)
const error = ref<string | null>(null)
const aggregation = ref<MiningAggregation | null>(null)

const loadAggregation = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await indexManagerApi.aggregations.getMining()
    aggregation.value = response.data
  } catch (err) {
    error.value = 'Unable to load mining aggregation data.'
    console.error('Failed to load mining aggregation:', err)
  } finally {
    loading.value = false
  }
}

const miningPercentage = computed(() => {
  if (!aggregation.value) return 0
  const { total_mining, total_documents } = aggregation.value
  if (total_documents === 0) return 0
  return Math.round((total_mining / total_documents) * 100)
})

const relevanceData = computed(() => {
  if (!aggregation.value?.by_relevance) return []
  return Object.entries(aggregation.value.by_relevance)
    .map(([name, count]) => ({ name: formatLabel(name), count }))
    .sort((a, b) => b.count - a.count)
})

const stageData = computed(() => {
  if (!aggregation.value?.by_mining_stage) return []
  return Object.entries(aggregation.value.by_mining_stage)
    .map(([name, count]) => ({ name: formatLabel(name), count }))
    .sort((a, b) => b.count - a.count)
})

const commodityData = computed(() => {
  if (!aggregation.value?.by_commodity) return []
  return Object.entries(aggregation.value.by_commodity)
    .map(([name, count]) => ({ name: formatLabel(name), count }))
    .sort((a, b) => b.count - a.count)
})

const locationData = computed(() => {
  if (!aggregation.value?.by_location) return []
  return Object.entries(aggregation.value.by_location)
    .map(([name, count]) => ({ name: formatLabel(name), count }))
    .sort((a, b) => b.count - a.count)
})

const formatLabel = (label: string) => {
  return label
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase())
}

const formatNumber = (num: number) => {
  return num.toLocaleString()
}

const getBarWidth = (count: number, data: { count: number }[]) => {
  if (data.length === 0) return 0
  const max = Math.max(...data.map((d) => d.count))
  if (max === 0) return 0
  return (count / max) * 100
}

const getRelevanceColor = (relevance: string) => {
  const lower = relevance.toLowerCase()
  if (lower.includes('core')) return 'bg-amber-500'
  if (lower.includes('peripheral')) return 'bg-yellow-400'
  if (lower.includes('not')) return 'bg-gray-400'
  return 'bg-gray-400'
}

const getStageColor = (stage: string) => {
  const lower = stage.toLowerCase()
  if (lower.includes('exploration')) return 'bg-sky-500'
  if (lower.includes('development')) return 'bg-blue-600'
  if (lower.includes('production')) return 'bg-green-500'
  if (lower.includes('unspecified')) return 'bg-gray-400'
  return 'bg-gray-400'
}

const getCommodityColor = (commodity: string) => {
  const lower = commodity.toLowerCase()
  if (lower.includes('gold')) return 'bg-yellow-500'
  if (lower.includes('copper')) return 'bg-orange-500'
  if (lower.includes('lithium')) return 'bg-cyan-500'
  if (lower.includes('nickel')) return 'bg-slate-500'
  if (lower.includes('uranium')) return 'bg-lime-500'
  if (lower.includes('iron')) return 'bg-red-700'
  if (lower.includes('rare')) return 'bg-purple-500'
  return 'bg-gray-400'
}

onMounted(loadAggregation)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Mining Breakdown
        </h1>
        <p class="text-muted-foreground">
          Distribution of mining-related content across all indexes
        </p>
      </div>
      <Button
        variant="outline"
        :disabled="loading"
        @click="loadAggregation"
      >
        <RefreshCw
          class="mr-2 h-4 w-4"
          :class="{ 'animate-spin': loading }"
        />
        Refresh
      </Button>
    </div>

    <div
      v-if="loading"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card
      v-else-if="error"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ error }}
        </p>
      </CardContent>
    </Card>

    <template v-else-if="aggregation">
      <!-- Summary Cards -->
      <div class="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader class="pb-2">
            <CardDescription>Total Documents</CardDescription>
            <CardTitle class="text-3xl">
              {{ formatNumber(aggregation.total_documents) }}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader class="pb-2">
            <CardDescription>Mining Related</CardDescription>
            <CardTitle class="text-3xl text-amber-500">
              {{ formatNumber(aggregation.total_mining) }}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader class="pb-2">
            <CardDescription>Mining Percentage</CardDescription>
            <CardTitle class="text-3xl">
              {{ miningPercentage }}%
            </CardTitle>
          </CardHeader>
        </Card>
      </div>

      <!-- Charts Grid -->
      <div class="grid gap-6 lg:grid-cols-2">
        <!-- By Relevance -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <Pickaxe class="h-5 w-5" />
              By Relevance
            </CardTitle>
            <CardDescription>
              Mining content relevance classification
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              v-if="relevanceData.length === 0"
              class="text-center py-8 text-muted-foreground"
            >
              No data available
            </div>
            <div
              v-else
              class="space-y-3"
            >
              <div
                v-for="item in relevanceData"
                :key="item.name"
                class="space-y-1"
              >
                <div class="flex justify-between text-sm">
                  <span>{{ item.name }}</span>
                  <span class="font-medium">{{ formatNumber(item.count) }}</span>
                </div>
                <div class="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    class="h-full rounded-full transition-all"
                    :class="getRelevanceColor(item.name)"
                    :style="{ width: `${getBarWidth(item.count, relevanceData)}%` }"
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- By Mining Stage -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <BarChart3 class="h-5 w-5" />
              By Mining Stage
            </CardTitle>
            <CardDescription>
              Lifecycle stage of mining content
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              v-if="stageData.length === 0"
              class="text-center py-8 text-muted-foreground"
            >
              No data available
            </div>
            <div
              v-else
              class="space-y-3"
            >
              <div
                v-for="item in stageData"
                :key="item.name"
                class="space-y-1"
              >
                <div class="flex justify-between text-sm">
                  <span>{{ item.name }}</span>
                  <span class="font-medium">{{ formatNumber(item.count) }}</span>
                </div>
                <div class="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    class="h-full rounded-full transition-all"
                    :class="getStageColor(item.name)"
                    :style="{ width: `${getBarWidth(item.count, stageData)}%` }"
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- By Commodity -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <Pickaxe class="h-5 w-5" />
              By Commodity
            </CardTitle>
            <CardDescription>
              Commodities mentioned in mining content
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              v-if="commodityData.length === 0"
              class="text-center py-8 text-muted-foreground"
            >
              No data available
            </div>
            <div
              v-else
              class="space-y-3"
            >
              <div
                v-for="item in commodityData"
                :key="item.name"
                class="space-y-1"
              >
                <div class="flex justify-between text-sm">
                  <span>{{ item.name }}</span>
                  <span class="font-medium">{{ formatNumber(item.count) }}</span>
                </div>
                <div class="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    class="h-full rounded-full transition-all"
                    :class="getCommodityColor(item.name)"
                    :style="{ width: `${getBarWidth(item.count, commodityData)}%` }"
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- By Location -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <MapPin class="h-5 w-5" />
              By Location
            </CardTitle>
            <CardDescription>
              Geographic distribution of mining content
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div
              v-if="locationData.length === 0"
              class="text-center py-8 text-muted-foreground"
            >
              No data available
            </div>
            <div
              v-else
              class="space-y-3"
            >
              <div
                v-for="item in locationData"
                :key="item.name"
                class="space-y-1"
              >
                <div class="flex justify-between text-sm">
                  <span>{{ item.name }}</span>
                  <span class="font-medium">{{ formatNumber(item.count) }}</span>
                </div>
                <div class="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    class="h-full rounded-full transition-all bg-emerald-500"
                    :style="{ width: `${getBarWidth(item.count, locationData)}%` }"
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </template>
  </div>
</template>
```

**Step 2: Verify lint passes**

Run: `cd dashboard && npm run lint`
Expected: PASS

**Step 3: Commit**

```bash
git add dashboard/src/views/intelligence/MiningBreakdownView.vue
git commit -m "feat(dashboard): add MiningBreakdownView"
```

---

## Task 9: Create ClassifierHealthWidget component

**Files:**
- Create: `dashboard/src/components/domain/classifier/ClassifierHealthWidget.vue`

**Step 1: Create the widget component**

```vue
<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { Activity, CircleCheck, CircleX } from 'lucide-vue-next'
import { classifierApi } from '@/api/client'
import type { MLHealthResponse } from '@/types/aggregation'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

const healthPollIntervalMs = 30000

const health = ref<MLHealthResponse | null>(null)
const loading = ref(true)
let pollTimer: ReturnType<typeof setInterval> | null = null

const loadHealth = async () => {
  try {
    const response = await classifierApi.metrics.mlHealth()
    health.value = response.data
  } catch (err) {
    console.error('Failed to load ML health:', err)
  } finally {
    loading.value = false
  }
}

const getPipelineModeVariant = (mode: string) => {
  if (mode === 'hybrid') return 'default'
  if (mode === 'rules-only') return 'secondary'
  return 'outline'
}

onMounted(() => {
  loadHealth()
  pollTimer = setInterval(loadHealth, healthPollIntervalMs)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <Card v-if="!loading && health">
    <CardHeader class="pb-3">
      <CardTitle class="flex items-center gap-2 text-sm font-medium">
        <Activity class="h-4 w-4" />
        Classifier Health
      </CardTitle>
    </CardHeader>
    <CardContent class="space-y-3">
      <!-- Pipeline Mode -->
      <div class="flex items-center gap-2 flex-wrap">
        <span class="text-xs text-muted-foreground">Pipeline:</span>
        <Badge :variant="getPipelineModeVariant(health.pipeline_mode.crime)" class="text-xs">
          Crime {{ health.pipeline_mode.crime }}
        </Badge>
        <Badge :variant="getPipelineModeVariant(health.pipeline_mode.mining)" class="text-xs">
          Mining {{ health.pipeline_mode.mining }}
        </Badge>
      </div>

      <!-- Crime ML -->
      <div
        v-if="health.crime_ml"
        class="flex items-center justify-between text-xs"
      >
        <div class="flex items-center gap-1.5">
          <CircleCheck
            v-if="health.crime_ml.reachable"
            class="h-3.5 w-3.5 text-green-500"
          />
          <CircleX
            v-else
            class="h-3.5 w-3.5 text-red-500"
          />
          <span>crime-ml</span>
        </div>
        <div class="flex items-center gap-2 text-muted-foreground">
          <span v-if="health.crime_ml.model_version">{{ health.crime_ml.model_version }}</span>
          <span v-if="health.crime_ml.latency_ms">{{ health.crime_ml.latency_ms }}ms</span>
        </div>
      </div>

      <!-- Mining ML -->
      <div
        v-if="health.mining_ml"
        class="flex items-center justify-between text-xs"
      >
        <div class="flex items-center gap-1.5">
          <CircleCheck
            v-if="health.mining_ml.reachable"
            class="h-3.5 w-3.5 text-green-500"
          />
          <CircleX
            v-else
            class="h-3.5 w-3.5 text-red-500"
          />
          <span>mining-ml</span>
        </div>
        <div class="flex items-center gap-2 text-muted-foreground">
          <span v-if="health.mining_ml.model_version">{{ health.mining_ml.model_version }}</span>
          <span v-if="health.mining_ml.latency_ms">{{ health.mining_ml.latency_ms }}ms</span>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
```

**Step 2: Create index export**

Create `dashboard/src/components/domain/classifier/index.ts`:

```typescript
export { default as ClassifierHealthWidget } from './ClassifierHealthWidget.vue'
```

**Step 3: Verify lint passes**

Run: `cd dashboard && npm run lint`
Expected: PASS

**Step 4: Commit**

```bash
git add dashboard/src/components/domain/classifier/
git commit -m "feat(dashboard): add ClassifierHealthWidget component"
```

---

## Task 10: Add Hybrid Decision Audit panel to DocumentDetailView

**Files:**
- Modify: `dashboard/src/views/intelligence/DocumentDetailView.vue`

**Step 1: Add MiningInfo type and update Document type if needed**

Check `dashboard/src/types/indexManager.ts` — if `MiningInfo` doesn't exist, add it:

```typescript
export interface MiningInfo {
  relevance?: string
  mining_stage?: string
  commodities?: string[]
  location?: string
  final_confidence?: number
  review_required?: boolean
  model_version?: string
}
```

And add `mining?: MiningInfo` to the `Document` interface.

**Step 2: Add Hybrid Decision Audit section to template**

After the existing document info Card (line 171), before the Raw JSON Card, add two new collapsible panels:

```vue
      <!-- Crime Decision Audit -->
      <Card v-if="(document as Record<string, unknown>).crime">
        <CardHeader>
          <CardTitle class="text-lg">Crime Classification Audit</CardTitle>
        </CardHeader>
        <CardContent>
          <dl class="grid grid-cols-2 gap-4">
            <div>
              <dt class="text-sm text-muted-foreground">Relevance</dt>
              <dd class="mt-1">
                <Badge variant="destructive">
                  {{ ((document as Record<string, unknown>).crime as Record<string, unknown>).street_crime_relevance }}
                </Badge>
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">Confidence</dt>
              <dd class="mt-1 font-mono text-sm">
                {{ (((document as Record<string, unknown>).crime as Record<string, unknown>).final_confidence as number)?.toFixed(2) ?? 'N/A' }}
              </dd>
            </div>
            <div v-if="((document as Record<string, unknown>).crime as Record<string, unknown>).sub_label">
              <dt class="text-sm text-muted-foreground">Sub Label</dt>
              <dd class="mt-1">
                <Badge variant="secondary">
                  {{ ((document as Record<string, unknown>).crime as Record<string, unknown>).sub_label }}
                </Badge>
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">Homepage Eligible</dt>
              <dd class="mt-1">
                <Badge :variant="((document as Record<string, unknown>).crime as Record<string, unknown>).homepage_eligible ? 'default' : 'secondary'">
                  {{ ((document as Record<string, unknown>).crime as Record<string, unknown>).homepage_eligible ? 'Yes' : 'No' }}
                </Badge>
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">Review Required</dt>
              <dd class="mt-1">
                <Badge :variant="((document as Record<string, unknown>).crime as Record<string, unknown>).review_required ? 'destructive' : 'secondary'">
                  {{ ((document as Record<string, unknown>).crime as Record<string, unknown>).review_required ? 'Yes' : 'No' }}
                </Badge>
              </dd>
            </div>
            <div v-if="((document as Record<string, unknown>).crime as Record<string, unknown>).crime_types">
              <dt class="text-sm text-muted-foreground">Crime Types</dt>
              <dd class="mt-1 flex flex-wrap gap-1">
                <Badge
                  v-for="ct in ((document as Record<string, unknown>).crime as Record<string, unknown>).crime_types as string[]"
                  :key="ct"
                  variant="outline"
                  class="text-xs"
                >
                  {{ ct }}
                </Badge>
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <!-- Mining Decision Audit -->
      <Card v-if="(document as Record<string, unknown>).mining">
        <CardHeader>
          <CardTitle class="text-lg">Mining Classification Audit</CardTitle>
        </CardHeader>
        <CardContent>
          <dl class="grid grid-cols-2 gap-4">
            <div>
              <dt class="text-sm text-muted-foreground">Relevance</dt>
              <dd class="mt-1">
                <Badge variant="default" class="bg-amber-500">
                  {{ ((document as Record<string, unknown>).mining as Record<string, unknown>).relevance }}
                </Badge>
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">Confidence</dt>
              <dd class="mt-1 font-mono text-sm">
                {{ (((document as Record<string, unknown>).mining as Record<string, unknown>).final_confidence as number)?.toFixed(2) ?? 'N/A' }}
              </dd>
            </div>
            <div v-if="((document as Record<string, unknown>).mining as Record<string, unknown>).mining_stage">
              <dt class="text-sm text-muted-foreground">Mining Stage</dt>
              <dd class="mt-1">
                <Badge variant="secondary">
                  {{ ((document as Record<string, unknown>).mining as Record<string, unknown>).mining_stage }}
                </Badge>
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">Review Required</dt>
              <dd class="mt-1">
                <Badge :variant="((document as Record<string, unknown>).mining as Record<string, unknown>).review_required ? 'destructive' : 'secondary'">
                  {{ ((document as Record<string, unknown>).mining as Record<string, unknown>).review_required ? 'Yes' : 'No' }}
                </Badge>
              </dd>
            </div>
            <div v-if="((document as Record<string, unknown>).mining as Record<string, unknown>).commodities">
              <dt class="text-sm text-muted-foreground">Commodities</dt>
              <dd class="mt-1 flex flex-wrap gap-1">
                <Badge
                  v-for="c in ((document as Record<string, unknown>).mining as Record<string, unknown>).commodities as string[]"
                  :key="c"
                  variant="outline"
                  class="text-xs"
                >
                  {{ c }}
                </Badge>
              </dd>
            </div>
            <div v-if="((document as Record<string, unknown>).mining as Record<string, unknown>).model_version">
              <dt class="text-sm text-muted-foreground">Model Version</dt>
              <dd class="mt-1 font-mono text-xs text-muted-foreground">
                {{ ((document as Record<string, unknown>).mining as Record<string, unknown>).model_version }}
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>
```

**Step 3: Verify lint passes**

Run: `cd dashboard && npm run lint`
Expected: PASS

**Step 4: Commit**

```bash
git add dashboard/src/views/intelligence/DocumentDetailView.vue
git commit -m "feat(dashboard): add hybrid decision audit panels to document detail"
```

---

## Task 11: Add routes and navigation for new views

**Files:**
- Modify: `dashboard/src/router/index.ts`
- Modify: `dashboard/src/config/navigation.ts`

**Step 1: Add mining route to router**

In `router/index.ts`, after the crime breakdown route (line 92), add:

```typescript
  {
    path: '/intelligence/mining',
    name: 'intelligence-mining',
    component: () => import('../views/intelligence/MiningBreakdownView.vue'),
    meta: { title: 'Mining Breakdown', section: 'intelligence', requiresAuth: true },
  },
```

**Step 2: Add mining nav item**

In `config/navigation.ts`, add `Pickaxe` to the lucide imports (line 1), then add a new child to the Intelligence section (after Crime Breakdown, line 59):

```typescript
      { title: 'Mining Breakdown', path: '/intelligence/mining', icon: Pickaxe },
```

**Step 3: Add ClassifierHealthWidget to Intelligence views**

The health widget needs to appear on all Intelligence pages. The cleanest approach: add it to `CrimeBreakdownView.vue` and `MiningBreakdownView.vue` in the header area. Import and render:

```vue
import { ClassifierHealthWidget } from '@/components/domain/classifier'
```

Place it in a sidebar layout or as a compact header card above the main content. Given the existing layout pattern (full-width views), place it as a compact card in the header row.

In both `CrimeBreakdownView.vue` and `MiningBreakdownView.vue`, wrap the header in a flex container:

```vue
<div class="flex items-start justify-between gap-4">
  <div>
    <h1 ...>...</h1>
    <p ...>...</p>
  </div>
  <div class="flex items-center gap-2">
    <ClassifierHealthWidget />
    <Button ...>Refresh</Button>
  </div>
</div>
```

**Step 4: Verify lint passes**

Run: `cd dashboard && npm run lint`
Expected: PASS

**Step 5: Verify build succeeds**

Run: `cd dashboard && npm run build`
Expected: PASS (no TypeScript errors)

**Step 6: Commit**

```bash
git add dashboard/src/router/index.ts dashboard/src/config/navigation.ts
git add dashboard/src/views/intelligence/CrimeBreakdownView.vue
git add dashboard/src/views/intelligence/MiningBreakdownView.vue
git commit -m "feat(dashboard): add mining route, navigation, and health widget integration"
```

---

## Task 12: Final verification

**Step 1: Run all index-manager tests**

Run: `cd index-manager && go test ./...`
Expected: PASS

**Step 2: Run index-manager linter**

Run: `cd index-manager && golangci-lint run`
Expected: PASS

**Step 3: Run classifier tests**

Run: `cd classifier && go test ./...`
Expected: PASS

**Step 4: Run classifier linter**

Run: `cd classifier && golangci-lint run`
Expected: PASS

**Step 5: Run dashboard lint**

Run: `cd dashboard && npm run lint`
Expected: PASS

**Step 6: Run dashboard build**

Run: `cd dashboard && npm run build`
Expected: PASS

**Step 7: Verify docker builds**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build index-manager classifier dashboard`
Expected: All three build successfully

---

## Summary

| Task | Service | What |
|------|---------|------|
| 1 | index-manager | Add mining ES mapping |
| 2 | index-manager | Add MiningAggregation domain type |
| 3 | index-manager | Add GetMiningAggregation service method |
| 4 | index-manager | Add mining aggregation handler + route |
| 5 | classifier | Add ML health metrics endpoint |
| 6 | dashboard | Add TypeScript types |
| 7 | dashboard | Add API client methods |
| 8 | dashboard | Create MiningBreakdownView |
| 9 | dashboard | Create ClassifierHealthWidget |
| 10 | dashboard | Add hybrid audit panels to document detail |
| 11 | dashboard | Add routes, navigation, widget integration |
| 12 | all | Final verification |
