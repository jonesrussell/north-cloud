# Publisher Service Testing Guide

This document provides comprehensive testing procedures for the Publisher service.

## Table of Contents

1. [Test Overview](#test-overview)
2. [Local Testing](#local-testing)
3. [Docker Testing](#docker-testing)
4. [Integration Testing](#integration-testing)
5. [Performance Testing](#performance-testing)
6. [Production Readiness Checklist](#production-readiness-checklist)

## Test Overview

### Test Levels

1. **Unit Tests**: Individual function/method testing
2. **Integration Tests**: API endpoints and database operations
3. **System Tests**: End-to-end publisher → Redis → consumer flow
4. **Performance Tests**: Load testing and scalability validation

### Test Environment Setup

```bash
# Start test infrastructure
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d postgres-publisher elasticsearch redis

# Wait for services to be healthy
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml ps
```

## Local Testing

### 1. Database Migration Test

```bash
# Connect to database
docker exec -it north-cloud-postgres-publisher psql -U postgres -d publisher

# Run migration
\i /migrations/001_initial_schema.sql

# Verify tables created
\dt

# Expected output:
#              List of relations
#  Schema |      Name        | Type  |  Owner
# --------+------------------+-------+----------
#  public | channels         | table | postgres
#  public | publish_history  | table | postgres
#  public | routes           | table | postgres
#  public | sources          | table | postgres
```

### 2. API Server Test

```bash
# Start API server
cd /home/jones/north-cloud/publisher
go run . api

# In another terminal, test health endpoint
curl http://localhost:8070/health

# Expected: {"status":"healthy","service":"publisher","version":"1.0.0"}
```

### 3. API Endpoints Test

**Create Source**:
```bash
curl -X POST http://localhost:8070/api/v1/sources \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{
    "name": "test_source",
    "index_pattern": "test_source_classified_content",
    "enabled": true
  }'

# Expected: 201 Created with source object
```

**List Sources**:
```bash
curl http://localhost:8070/api/v1/sources \
  -H "Authorization: Bearer $JWT_TOKEN"

# Expected: {"sources":[...], "count":1}
```

**Create Channel**:
```bash
curl -X POST http://localhost:8070/api/v1/channels \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{
    "name": "articles:test",
    "description": "Test channel",
    "enabled": true
  }'

# Expected: 201 Created with channel object
```

**Create Route**:
```bash
# First, get source and channel IDs from previous responses
SOURCE_ID="<uuid-from-create-source>"
CHANNEL_ID="<uuid-from-create-channel>"

curl -X POST http://localhost:8070/api/v1/routes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{
    "source_id": "'$SOURCE_ID'",
    "channel_id": "'$CHANNEL_ID'",
    "min_quality_score": 50,
    "topics": ["test"],
    "enabled": true
  }'

# Expected: 201 Created with route object including source/channel details
```

### 4. Router Service Test

**Prepare Test Data**:
```bash
# Insert test article into Elasticsearch
curl -X POST "http://localhost:9200/test_source_classified_content/_doc/test-article-1" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Test Crime Article",
    "body": "Test article content",
    "raw_text": "Test article content",
    "canonical_url": "https://example.com/test-1",
    "source": "https://example.com/test-1",
    "published_date": "2025-12-28T10:00:00Z",
    "quality_score": 85,
    "topics": ["test", "crime"],
    "content_type": "article",
    "is_crime_related": true,
    "source_reputation": 75,
    "confidence": 0.9
  }'
```

**Start Router**:
```bash
# Terminal 1: Start router
go run . router

# Terminal 2: Monitor Redis
redis-cli SUBSCRIBE articles:test

# Expected: Router should pick up the article and publish to Redis
# Redis subscriber should receive the message
```

**Verify Publish History**:
```bash
curl http://localhost:8070/api/v1/publish-history \
  -H "Authorization: Bearer $JWT_TOKEN"

# Expected: Article appears in history with channel "articles:test"
```

## Docker Testing

### 1. Build and Start Services

```bash
# Build images
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml build publisher-api publisher-router publisher-frontend

# Start services
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d publisher-api publisher-router publisher-frontend

# Check logs
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f publisher-api
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f publisher-router
```

### 2. Test API Connectivity

```bash
# Test from host
curl http://localhost:8070/health

# Test from another container
docker exec north-cloud-nginx-dev curl http://publisher-api:8070/health
```

### 3. Test Frontend

```bash
# Access frontend
open http://localhost:3003

# Should see Publisher Dashboard with login page
```

### 4. Test Router Processing

```bash
# Insert test article (same as local test)
curl -X POST "http://localhost:9200/test_source_classified_content/_doc/test-article-2" \
  -H "Content-Type: application/json" \
  -d '{...}'  # Same JSON as above

# Wait for router poll interval (5 minutes) or restart router
docker restart north-cloud-publisher-router-dev

# Monitor router logs
docker logs -f north-cloud-publisher-router-dev

# Should see: "Processing routes...", "Found X articles for route...", "Published article..."
```

## Integration Testing

### End-to-End Flow Test

This test validates the complete flow: API → Database → Router → Elasticsearch → Redis

```bash
#!/bin/bash
set -e

echo "=== Publisher Integration Test ==="

# Get JWT token (adjust for your auth setup)
TOKEN=$(curl -s -X POST http://localhost:8040/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' \
  | jq -r '.token')

echo "✓ Obtained JWT token"

# 1. Create source
SOURCE_RESPONSE=$(curl -s -X POST http://localhost:8070/api/v1/sources \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "integration_test_source",
    "index_pattern": "integration_test_classified_content",
    "enabled": true
  }')

SOURCE_ID=$(echo $SOURCE_RESPONSE | jq -r '.id')
echo "✓ Created source: $SOURCE_ID"

# 2. Create channel
CHANNEL_RESPONSE=$(curl -s -X POST http://localhost:8070/api/v1/channels \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "articles:integration_test",
    "description": "Integration test channel",
    "enabled": true
  }')

CHANNEL_ID=$(echo $CHANNEL_RESPONSE | jq -r '.id')
echo "✓ Created channel: $CHANNEL_ID"

# 3. Create route
ROUTE_RESPONSE=$(curl -s -X POST http://localhost:8070/api/v1/routes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"source_id\": \"$SOURCE_ID\",
    \"channel_id\": \"$CHANNEL_ID\",
    \"min_quality_score\": 50,
    \"topics\": [\"integration_test\"],
    \"enabled\": true
  }")

ROUTE_ID=$(echo $ROUTE_RESPONSE | jq -r '.id')
echo "✓ Created route: $ROUTE_ID"

# 4. Insert test article into Elasticsearch
curl -s -X POST "http://localhost:9200/integration_test_classified_content/_doc/integration-article-1" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Integration Test Article",
    "body": "This is a test article for integration testing",
    "raw_text": "This is a test article for integration testing",
    "canonical_url": "https://example.com/integration-test-1",
    "source": "https://example.com/integration-test-1",
    "published_date": "2025-12-28T10:00:00Z",
    "quality_score": 85,
    "topics": ["integration_test"],
    "content_type": "article",
    "is_crime_related": true,
    "source_reputation": 75,
    "confidence": 0.9
  }' > /dev/null

echo "✓ Inserted test article into Elasticsearch"

# 5. Subscribe to Redis channel in background
(redis-cli SUBSCRIBE articles:integration_test | grep -m 1 "Integration Test Article") &
REDIS_PID=$!

# 6. Trigger router processing (restart to force immediate run)
docker restart north-cloud-publisher-router-dev > /dev/null
echo "✓ Triggered router processing"

# 7. Wait for message
echo "⏳ Waiting for Redis message..."
sleep 30

# Check if Redis received message
kill $REDIS_PID 2>/dev/null || true

# 8. Verify publish history
HISTORY=$(curl -s "http://localhost:8070/api/v1/publish-history?limit=1" \
  -H "Authorization: Bearer $TOKEN")

if echo "$HISTORY" | jq -e '.history[0].article_id == "integration-article-1"' > /dev/null; then
  echo "✓ Article found in publish history"
else
  echo "✗ Article NOT found in publish history"
  exit 1
fi

# Cleanup
echo "🧹 Cleaning up..."
curl -s -X DELETE "http://localhost:8070/api/v1/routes/$ROUTE_ID" \
  -H "Authorization: Bearer $TOKEN" > /dev/null
curl -s -X DELETE "http://localhost:8070/api/v1/channels/$CHANNEL_ID" \
  -H "Authorization: Bearer $TOKEN" > /dev/null
curl -s -X DELETE "http://localhost:8070/api/v1/sources/$SOURCE_ID" \
  -H "Authorization: Bearer $TOKEN" > /dev/null
curl -s -X DELETE "http://localhost:9200/integration_test_classified_content" > /dev/null

echo "✅ Integration test completed successfully!"
```

Save as `test-integration.sh` and run:
```bash
chmod +x test-integration.sh
./test-integration.sh
```

## Performance Testing

### Load Testing with Apache Bench

**Test API Endpoints**:
```bash
# Install Apache Bench
apt-get install apache2-utils

# Test health endpoint (1000 requests, 10 concurrent)
ab -n 1000 -c 10 http://localhost:8070/health

# Test list sources (with authentication)
ab -n 100 -c 5 -H "Authorization: Bearer $TOKEN" http://localhost:8070/api/v1/sources

# Expected: <100ms average response time, 0% failed requests
```

### Router Performance Test

**Setup**:
```bash
# Insert 100 test articles
for i in {1..100}; do
  curl -s -X POST "http://localhost:9200/load_test_classified_content/_doc/load-test-$i" \
    -H "Content-Type: application/json" \
    -d "{
      \"title\": \"Load Test Article $i\",
      \"body\": \"Test content $i\",
      \"raw_text\": \"Test content $i\",
      \"canonical_url\": \"https://example.com/load-test-$i\",
      \"source\": \"https://example.com/load-test-$i\",
      \"published_date\": \"2025-12-28T10:00:00Z\",
      \"quality_score\": 85,
      \"topics\": [\"load_test\"],
      \"content_type\": \"article\",
      \"is_crime_related\": true
    }" > /dev/null
done

echo "Inserted 100 test articles"
```

**Measure Router Performance**:
```bash
# Restart router and measure time
START_TIME=$(date +%s)
docker restart north-cloud-publisher-router-dev

# Wait for processing
sleep 60

# Check publish history count
PUBLISHED_COUNT=$(curl -s "http://localhost:8070/api/v1/publish-history?limit=1000" \
  -H "Authorization: Bearer $TOKEN" | jq '.count')

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo "Published: $PUBLISHED_COUNT articles"
echo "Duration: $DURATION seconds"
echo "Throughput: $((PUBLISHED_COUNT / DURATION)) articles/second"

# Expected: >10 articles/second
```

### Redis Throughput Test

```bash
# Test Redis pub/sub throughput
redis-benchmark -t publish -c 10 -n 10000 -d 10000

# Expected: >10000 requests/second
```

## Production Readiness Checklist

### Pre-Deployment

- [ ] All unit tests pass
- [ ] Integration tests pass
- [ ] Performance tests meet requirements
- [ ] Database migrations tested
- [ ] Docker images build successfully
- [ ] Environment variables configured
- [ ] JWT secrets generated (strong, unique)
- [ ] Redis password configured
- [ ] SSL/TLS certificates obtained
- [ ] Monitoring configured
- [ ] Logging configured
- [ ] Backup strategy defined

### Deployment Validation

- [ ] Database initialized and migrated
- [ ] API server starts successfully
- [ ] Router service starts successfully
- [ ] Frontend accessible via nginx
- [ ] Health endpoints return 200
- [ ] Can create sources, channels, routes via API
- [ ] Router processes articles and publishes to Redis
- [ ] Publish history recorded correctly
- [ ] Consumer can subscribe and receive messages
- [ ] Graceful shutdown works (SIGTERM)
- [ ] Services restart automatically
- [ ] Logs are being collected
- [ ] Metrics are being reported

### Post-Deployment

- [ ] Monitor logs for errors (first 24 hours)
- [ ] Check publish history growth
- [ ] Verify article throughput matches expected volume
- [ ] Test consumer integration
- [ ] Monitor resource usage (CPU, memory, disk)
- [ ] Verify deduplication working
- [ ] Test rollback procedures
- [ ] Document any issues

## Troubleshooting Tests

### API Server Not Starting

```bash
# Check logs
docker logs north-cloud-publisher-api-dev

# Common issues:
# - Database not ready: Wait for postgres health check
# - Port conflict: Check if 8070 is in use
# - Missing environment variables: Check .env file
```

### Router Not Publishing

```bash
# Check router logs
docker logs north-cloud-publisher-router-dev

# Verify:
# 1. Routes exist and are enabled
curl http://localhost:8070/api/v1/routes -H "Authorization: Bearer $TOKEN"

# 2. Articles exist in Elasticsearch
curl http://localhost:9200/test_source_classified_content/_search

# 3. Redis is accessible
docker exec north-cloud-publisher-router-dev redis-cli -h redis PING
```

### Frontend Not Loading

```bash
# Check frontend logs
docker logs north-cloud-publisher-frontend-dev

# Check nginx routing
curl -I http://localhost/publisher

# Common issues:
# - Frontend container not started
# - Nginx misconfigured
# - API proxy not working
```

## Automated Test Suite

Create `run-tests.sh`:

```bash
#!/bin/bash
set -e

echo "🧪 Running Publisher Test Suite"
echo "================================"

# Unit tests
echo "1️⃣ Running unit tests..."
cd /home/jones/north-cloud/publisher
go test ./... -v

# Integration tests
echo "2️⃣ Running integration tests..."
./test-integration.sh

# Performance tests
echo "3️⃣ Running performance tests..."
ab -n 100 -c 10 http://localhost:8070/health > /dev/null
echo "✓ Performance tests passed"

# Frontend tests
echo "4️⃣ Running frontend tests..."
cd frontend
npm test
cd ..

echo "✅ All tests passed!"
```

## Continuous Integration

Example GitHub Actions workflow:

```yaml
name: Publisher Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_PASSWORD: test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

      redis:
        image: redis:7-alpine
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
    - uses: actions/checkout@v3

    - name: Setup Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.25'

    - name: Run tests
      run: |
        cd publisher
        go test ./... -v

    - name: Build
      run: |
        cd publisher
        go build .
```

## Summary

This testing guide covers:
- ✅ Local development testing
- ✅ Docker container testing
- ✅ End-to-end integration testing
- ✅ Performance and load testing
- ✅ Production readiness validation

For production deployment, ensure all checklist items are completed and all tests pass consistently.
