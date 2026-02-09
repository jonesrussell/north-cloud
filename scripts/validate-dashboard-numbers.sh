#!/usr/bin/env bash
# Validate Pipeline Monitor numbers (Content Flow Today) against production DBs.
# Run on prod: ssh jones@northcloud.biz 'cd /opt/north-cloud && ./scripts/validate-dashboard-numbers.sh'
# Uses same logic as: crawler GetTodayStats, classifier GetStats(date=today), publisher GetPublishStats(today).

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

# Compose files (prod: base + prod; dev: base + dev)
COMPOSE_FILES="-f docker-compose.base.yml"
if [ -f "docker-compose.prod.yml" ]; then
  COMPOSE_FILES="$COMPOSE_FILES -f docker-compose.prod.yml"
elif [ -f "docker-compose.dev.yml" ]; then
  COMPOSE_FILES="$COMPOSE_FILES -f docker-compose.dev.yml"
fi

run_psql() {
  local service=$1
  local db=$2
  local query=$3
  docker compose $COMPOSE_FILES exec -T "postgres-${service}" psql -U postgres -d "$db" -t -A -c "$query" 2>/dev/null
}

echo "=== Dashboard validation ($(date -Iseconds)) ==="
echo ""

# 1) Crawler: crawled_today, indexed_today (job_executions where started_at >= CURRENT_DATE, status = completed)
echo "--- Crawler (crawled_today, indexed_today) ---"
CRAWLER_RESULT=$(run_psql "crawler" "crawler" "
  SELECT COALESCE(SUM(items_crawled), 0), COALESCE(SUM(items_indexed), 0)
  FROM job_executions
  WHERE started_at >= CURRENT_DATE AND status = 'completed';
" 2>/dev/null || echo "0	0")
CRAWLED=$(echo "$CRAWLER_RESULT" | cut -f1)
INDEXED=$(echo "$CRAWLER_RESULT" | cut -f2)
echo "Crawled (from DB): $CRAWLED"
echo "Indexed (from DB): $INDEXED"
echo ""

# 2) Classifier: total_classified (classification_history where classified_at >= start of today, server TZ)
echo "--- Classifier (total_classified today) ---"
# Use server local date; Postgres CURRENT_DATE is in server TZ
CLASSIFIED=$(run_psql "classifier" "classifier" "
  SELECT COUNT(*) FROM classification_history WHERE classified_at >= CURRENT_DATE;
" 2>/dev/null || echo "0")
echo "Classified (from DB): $CLASSIFIED"
echo ""

# 3) Publisher: total_articles today = sum of per-channel counts = rows in publish_history
echo "--- Publisher (total_articles today = Routed/Published) ---"
PUB_TOTAL=$(run_psql "publisher" "publisher" "
  SELECT COUNT(*) FROM publish_history WHERE published_at >= CURRENT_DATE;
" 2>/dev/null || echo "0")
echo "Published (from DB): $PUB_TOTAL"
echo ""

echo "=== Summary (expected vs dashboard) ==="
echo "Dashboard shows: 7,697 Crawled | 6,522 Indexed | 4,745 Classified | 6,226 Routed | 6,226 Published"
echo "DB validation:   $CRAWLED Crawled | $INDEXED Indexed | $CLASSIFIED Classified | $PUB_TOTAL Routed/Published"
