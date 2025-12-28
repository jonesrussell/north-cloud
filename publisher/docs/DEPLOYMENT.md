# Publisher Service Deployment Guide

This guide provides step-by-step deployment instructions for the Publisher service in development and production environments.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Development Deployment](#development-deployment)
3. [Production Deployment](#production-deployment)
4. [Database Migration](#database-migration)
5. [Service Verification](#service-verification)
6. [Rollback Procedures](#rollback-procedures)
7. [Monitoring & Maintenance](#monitoring--maintenance)
8. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Required Infrastructure

Before deploying the Publisher service, ensure the following infrastructure is running:

- **PostgreSQL 16+**: Publisher database
- **Elasticsearch 8.x**: Source of classified articles
- **Redis 7.x**: Pub/sub messaging
- **Nginx**: Reverse proxy (production)
- **Docker & Docker Compose**: Container orchestration

### Required Environment Variables

Create or update `.env` file with the following variables:

```bash
# Publisher Database
POSTGRES_PUBLISHER_HOST=postgres-publisher
POSTGRES_PUBLISHER_PORT=5432
POSTGRES_PUBLISHER_USER=postgres
POSTGRES_PUBLISHER_PASSWORD=<strong-password>
POSTGRES_PUBLISHER_DB=publisher
POSTGRES_PUBLISHER_SSLMODE=disable  # Use 'require' in production

# Publisher API
PUBLISHER_PORT=8070

# Publisher Router
PUBLISHER_ROUTER_CHECK_INTERVAL=5m
PUBLISHER_ROUTER_BATCH_SIZE=100

# Authentication (required)
AUTH_JWT_SECRET=<your-jwt-secret>  # Generate with: openssl rand -hex 32

# Elasticsearch
ELASTICSEARCH_URL=http://elasticsearch:9200

# Redis
REDIS_URL=redis://redis:6379
```

**Security Notes**:
- Generate strong `POSTGRES_PUBLISHER_PASSWORD`: `openssl rand -base64 32`
- Use same `AUTH_JWT_SECRET` across all services
- In production, use `POSTGRES_PUBLISHER_SSLMODE=require`

---

## Development Deployment

### 1. Start Infrastructure Services

```bash
# Start base infrastructure (PostgreSQL, Elasticsearch, Redis)
docker-compose -f docker-compose.base.yml up -d postgres-publisher elasticsearch redis

# Wait for services to be healthy (check status)
docker-compose -f docker-compose.base.yml ps

# Expected: All services showing "Up" and "healthy"
```

### 2. Run Database Migrations

```bash
# Connect to publisher database
docker exec -it north-cloud-postgres-publisher psql -U postgres -d publisher

# Run init script (creates extensions and functions)
\i /migrations/init.sql

# Run schema migration
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

# Exit psql
\q
```

### 3. Build and Start Publisher Services

```bash
# Build all publisher services
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml build publisher-api publisher-router

# Start services
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d publisher-api publisher-router

# Check logs
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f publisher-api
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f publisher-router
```

### 4. Verify Services

```bash
# API health check
curl http://localhost:8070/health

# Expected: {"status":"healthy","service":"publisher","version":"1.0.0"}

# Frontend is served via unified dashboard
# Access at http://localhost/dashboard/publisher (via nginx)
# Or directly at http://localhost:3002/dashboard/publisher (development)
```

### 5. Access Frontend Dashboard

```bash
# Access via unified dashboard (development)
open http://localhost:3002/dashboard/publisher

# Via nginx (production)
open http://localhost/dashboard/publisher
```

---

## Production Deployment

### 1. Pre-Deployment Checklist

- [ ] Environment variables configured in `.env`
- [ ] Strong passwords generated for database
- [ ] JWT secret generated and shared across services
- [ ] SSL/TLS certificates obtained (if using HTTPS)
- [ ] Database backup strategy defined
- [ ] Monitoring alerts configured
- [ ] Rollback plan documented

### 2. Build Production Images

```bash
# Build production images (no source mounts, optimized)
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml build publisher-api publisher-router

# Verify images created
docker images | grep publisher
```

### 3. Start Infrastructure

```bash
# Start infrastructure services
docker-compose -f docker-compose.base.yml up -d postgres-publisher elasticsearch redis

# Wait for health checks to pass
docker-compose -f docker-compose.base.yml ps
```

### 4. Run Database Migrations

```bash
# Run migrations (same as development)
docker exec -it north-cloud-postgres-publisher psql -U postgres -d publisher

# Run init and schema
\i /migrations/init.sql
\i /migrations/001_initial_schema.sql

# Verify tables
\dt

# Exit
\q
```

### 5. Seed Initial Configuration (Optional)

If migrating from YAML config, create initial sources, channels, and routes:

```bash
# Get JWT token for API access
TOKEN=$(curl -s -X POST http://localhost:8040/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' \
  | jq -r '.token')

# Create a source
curl -X POST http://localhost:8070/api/v1/sources \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "example_com",
    "index_pattern": "example_com_classified_content",
    "enabled": true
  }'

# Create a channel
curl -X POST http://localhost:8070/api/v1/channels \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "articles:crime",
    "description": "Crime-related articles",
    "enabled": true
  }'

# Create routes as needed (see TESTING.md for examples)
```

### 6. Start Publisher Services

```bash
# Start publisher services
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d publisher-api publisher-router

# Verify startup
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml ps

# Check logs for errors
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml logs publisher-api
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml logs publisher-router
```

### 7. Start Nginx (Reverse Proxy)

```bash
# Start nginx (serves frontend and proxies API)
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d nginx

# Verify nginx configuration
docker exec north-cloud-nginx nginx -t

# Expected: nginx: configuration file /etc/nginx/nginx.conf test is successful
```

### 8. Verify Production Deployment

```bash
# Health check via nginx
curl https://northcloud.biz/api/health/publisher

# Frontend access (via unified dashboard)
open https://northcloud.biz/dashboard/publisher

# API access (with JWT)
curl https://northcloud.biz/api/publisher/v1/sources \
  -H "Authorization: Bearer $TOKEN"
```

---

## Database Migration

### Initial Schema Migration

The initial schema creates four tables: `sources`, `channels`, `routes`, and `publish_history`.

**Migration Files**:
- `/publisher/internal/database/init.sql` - Extensions and functions
- `/publisher/internal/database/migrations/001_initial_schema.sql` - Table creation

**Manual Migration**:
```bash
# Connect to database
docker exec -it north-cloud-postgres-publisher psql -U postgres -d publisher

# Run scripts
\i /migrations/init.sql
\i /migrations/001_initial_schema.sql

# Verify
\dt
\df update_updated_at_column
```

**Automated Migration** (future):
```bash
# Using golang-migrate (to be implemented)
migrate -path publisher/internal/database/migrations \
        -database "postgres://user:pass@host:5432/publisher?sslmode=disable" \
        up
```

### Future Schema Changes

When adding new migrations:

1. Create migration file: `002_add_feature.sql`
2. Apply manually or via migration tool
3. Update schema version in database
4. Test rollback with down migration

---

## Service Verification

### API Service

**Health Check**:
```bash
curl http://localhost:8070/health

# Expected:
# {"status":"healthy","service":"publisher","version":"1.0.0"}
```

**API Endpoints**:
```bash
# Get JWT token
TOKEN=$(curl -s -X POST http://localhost:8040/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' \
  | jq -r '.token')

# List sources
curl http://localhost:8070/api/v1/sources \
  -H "Authorization: Bearer $TOKEN"

# List channels
curl http://localhost:8070/api/v1/channels \
  -H "Authorization: Bearer $TOKEN"

# List routes
curl http://localhost:8070/api/v1/routes \
  -H "Authorization: Bearer $TOKEN"
```

### Router Service

**Logs Check**:
```bash
# Watch router logs
docker logs -f north-cloud-publisher-router-dev  # or north-cloud-publisher-router-prod

# Expected log messages:
# - "Starting publisher router service"
# - "Processing routes..." (every 5 minutes by default)
# - "Found X articles for route..." (when articles match filters)
# - "Published article..." (when article published to Redis)
```

**Redis Verification**:
```bash
# Subscribe to a channel
redis-cli SUBSCRIBE articles:crime

# Wait for router to publish (or trigger manually by inserting test article)
# Expected: Receive JSON message with article data
```

### Frontend Service

The publisher frontend is part of the unified dashboard service. Access it via:

**Development**:
```bash
# Access via dashboard dev server
curl http://localhost:3002/dashboard/publisher

# Expected: HTML with <div id="app">
```

**Production**:
```bash
# Access nginx-served dashboard
curl http://localhost/dashboard/publisher

# Via domain
curl https://northcloud.biz/dashboard/publisher
```

**Browser Verification**:
1. Open `http://localhost/dashboard/publisher` (or `https://northcloud.biz/dashboard/publisher`)
2. Login page should appear (if not authenticated)
3. After login, see Publisher Dashboard with stats
4. Navigate to Sources, Channels, Routes pages
5. Verify CRUD operations work

---

## Rollback Procedures

### Scenario 1: Router Service Issues

If the router service fails to publish or has errors:

**Step 1**: Stop router service
```bash
docker stop north-cloud-publisher-router-dev  # or -prod
```

**Step 2**: Review logs
```bash
docker logs north-cloud-publisher-router-dev
```

**Step 3**: Fix issue
- Update configuration
- Fix database data (routes, sources, channels)
- Rebuild image if code changes needed

**Step 4**: Restart router
```bash
docker restart north-cloud-publisher-router-dev
```

**Step 5**: Verify with logs
```bash
docker logs -f north-cloud-publisher-router-dev
```

### Scenario 2: API Service Issues

If API endpoints fail or authentication breaks:

**Step 1**: Stop API service
```bash
docker stop north-cloud-publisher-api-dev  # or -prod
```

**Step 2**: Check configuration
- Verify `AUTH_JWT_SECRET` matches across services
- Verify database connection string
- Check port availability

**Step 3**: Restart with corrected config
```bash
# Update .env file
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d publisher-api
```

**Step 4**: Verify health
```bash
curl http://localhost:8070/health
```

### Scenario 3: Database Corruption

If database data is corrupted or migration fails:

**Step 1**: Stop all publisher services
```bash
docker stop north-cloud-publisher-api-dev north-cloud-publisher-router-dev
```

**Step 2**: Backup current database
```bash
docker exec north-cloud-postgres-publisher pg_dump -U postgres publisher > backup_$(date +%Y%m%d_%H%M%S).sql
```

**Step 3**: Restore from backup (if available)
```bash
# Drop and recreate database
docker exec -it north-cloud-postgres-publisher psql -U postgres -c "DROP DATABASE publisher;"
docker exec -it north-cloud-postgres-publisher psql -U postgres -c "CREATE DATABASE publisher;"

# Restore backup
docker exec -i north-cloud-postgres-publisher psql -U postgres publisher < backup_YYYYMMDD_HHMMSS.sql
```

**Step 4**: Re-run migrations (if starting fresh)
```bash
docker exec -it north-cloud-postgres-publisher psql -U postgres -d publisher

\i /migrations/init.sql
\i /migrations/001_initial_schema.sql
```

**Step 5**: Restart services
```bash
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d publisher-api publisher-router
```

### Scenario 4: Complete Rollback to Previous Version

If the new publisher architecture fails entirely:

**Step 1**: Stop new publisher services
```bash
docker stop north-cloud-publisher-api-dev north-cloud-publisher-router-dev
```

**Step 2**: Revert to previous docker-compose configuration
```bash
git checkout <previous-commit> docker-compose.prod.yml
```

**Step 3**: Start old publisher service
```bash
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d publisher
```

**Step 4**: Verify system working
- Check router service logs
- Verify articles are published to Redis channels
- Monitor consumer services receiving messages

**Rollback Window**: Recommended 1 hour decision window after cutover

---

## Monitoring & Maintenance

### Health Checks

**Automated Health Monitoring**:
```bash
# Create health check script
cat > /usr/local/bin/check_publisher_health.sh << 'EOF'
#!/bin/bash

# API health
API_HEALTH=$(curl -s http://localhost:8070/health | jq -r '.status')
if [ "$API_HEALTH" != "healthy" ]; then
    echo "ALERT: Publisher API unhealthy"
    # Send alert (email, Slack, PagerDuty, etc.)
fi

# Router activity (check logs for recent processing)
ROUTER_ACTIVE=$(docker logs --since 10m north-cloud-publisher-router-dev 2>&1 | grep -c "Processing routes")
if [ "$ROUTER_ACTIVE" -eq 0 ]; then
    echo "WARNING: Router hasn't processed in 10 minutes"
fi

# Database connectivity
DB_CHECK=$(docker exec north-cloud-postgres-publisher psql -U postgres -d publisher -c "SELECT 1" 2>&1)
if [ $? -ne 0 ]; then
    echo "ALERT: Publisher database unreachable"
fi

# Redis connectivity
REDIS_CHECK=$(redis-cli PING 2>&1)
if [ "$REDIS_CHECK" != "PONG" ]; then
    echo "ALERT: Redis unreachable"
fi

echo "All health checks passed"
EOF

chmod +x /usr/local/bin/check_publisher_health.sh

# Run via cron (every 5 minutes)
echo "*/5 * * * * /usr/local/bin/check_publisher_health.sh" | crontab -
```

### Metrics to Monitor

**Database Metrics**:
```bash
# Active routes
docker exec north-cloud-postgres-publisher psql -U postgres -d publisher -c \
  "SELECT COUNT(*) FROM routes WHERE enabled = true;"

# Publish history (last 24 hours)
docker exec north-cloud-postgres-publisher psql -U postgres -d publisher -c \
  "SELECT COUNT(*) FROM publish_history WHERE published_at > NOW() - INTERVAL '24 hours';"

# Per-channel stats
docker exec north-cloud-postgres-publisher psql -U postgres -d publisher -c \
  "SELECT channel_name, COUNT(*) FROM publish_history
   WHERE published_at > NOW() - INTERVAL '24 hours'
   GROUP BY channel_name ORDER BY COUNT(*) DESC;"
```

**Service Metrics**:
```bash
# API request rate (via nginx logs)
tail -f /var/log/nginx/access.log | grep "/api/publisher"

# Router processing rate (articles/hour)
docker logs --since 1h north-cloud-publisher-router-dev 2>&1 | grep -c "Published article"

# Redis pub/sub activity
redis-cli PUBSUB NUMSUB articles:crime articles:news
```

**Resource Usage**:
```bash
# Container stats
docker stats north-cloud-publisher-api-dev north-cloud-publisher-router-dev

# Database size
docker exec north-cloud-postgres-publisher psql -U postgres -d publisher -c \
  "SELECT pg_size_pretty(pg_database_size('publisher'));"
```

### Log Management

**Centralized Logging** (recommended for production):
```yaml
# Add to docker-compose.prod.yml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

**Log Rotation**:
```bash
# Docker log rotation (set in /etc/docker/daemon.json)
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "5"
  }
}

# Restart Docker daemon
sudo systemctl restart docker
```

**Log Aggregation** (for multiple services):
- Use ELK stack (Elasticsearch, Logstash, Kibana)
- Use Grafana Loki
- Use Datadog or New Relic

### Database Maintenance

**Backup Strategy**:
```bash
# Daily automated backup script
cat > /usr/local/bin/backup_publisher_db.sh << 'EOF'
#!/bin/bash
BACKUP_DIR="/var/backups/publisher"
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p $BACKUP_DIR

docker exec north-cloud-postgres-publisher pg_dump -U postgres publisher | \
  gzip > $BACKUP_DIR/publisher_$DATE.sql.gz

# Keep last 7 days
find $BACKUP_DIR -name "publisher_*.sql.gz" -mtime +7 -delete

echo "Backup completed: publisher_$DATE.sql.gz"
EOF

chmod +x /usr/local/bin/backup_publisher_db.sh

# Schedule daily at 2 AM
echo "0 2 * * * /usr/local/bin/backup_publisher_db.sh" | crontab -
```

**Vacuum and Analyze** (weekly):
```bash
# Optimize database performance
docker exec north-cloud-postgres-publisher psql -U postgres -d publisher -c "VACUUM ANALYZE;"

# Schedule weekly (Sundays at 3 AM)
echo "0 3 * * 0 docker exec north-cloud-postgres-publisher psql -U postgres -d publisher -c 'VACUUM ANALYZE;'" | crontab -
```

**Publish History Archival** (monthly):
```bash
# Archive old publish history (keep last 90 days)
# Create archive table
docker exec north-cloud-postgres-publisher psql -U postgres -d publisher -c \
  "CREATE TABLE IF NOT EXISTS publish_history_archive (LIKE publish_history INCLUDING ALL);"

# Move old records to archive
docker exec north-cloud-postgres-publisher psql -U postgres -d publisher -c \
  "INSERT INTO publish_history_archive SELECT * FROM publish_history
   WHERE published_at < NOW() - INTERVAL '90 days';"

# Delete archived records from main table
docker exec north-cloud-postgres-publisher psql -U postgres -d publisher -c \
  "DELETE FROM publish_history WHERE published_at < NOW() - INTERVAL '90 days';"
```

---

## Troubleshooting

### API Service Won't Start

**Symptoms**: Container exits immediately or health check fails

**Diagnosis**:
```bash
# Check logs
docker logs north-cloud-publisher-api-dev

# Common errors:
# - "failed to connect to database" - Database not ready or wrong credentials
# - "bind: address already in use" - Port 8070 in use
# - "JWT secret not configured" - Missing AUTH_JWT_SECRET
```

**Solutions**:
1. **Database connection**:
   ```bash
   # Verify database is running
   docker ps | grep postgres-publisher

   # Test connection
   docker exec -it north-cloud-postgres-publisher psql -U postgres -d publisher

   # Check credentials in .env
   grep POSTGRES_PUBLISHER .env
   ```

2. **Port conflict**:
   ```bash
   # Find process using port 8070
   lsof -i :8070

   # Change PUBLISHER_PORT in .env or kill conflicting process
   ```

3. **Missing JWT secret**:
   ```bash
   # Generate and add to .env
   echo "AUTH_JWT_SECRET=$(openssl rand -hex 32)" >> .env

   # Restart service
   docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d publisher-api
   ```

### Router Not Publishing Articles

**Symptoms**: Router runs but no articles published to Redis

**Diagnosis**:
```bash
# Check router logs
docker logs -f north-cloud-publisher-router-dev

# Look for:
# - "Processing routes..." - Router is active
# - "Found 0 articles for route..." - No articles match filters
# - Errors: Elasticsearch connection, Redis connection
```

**Solutions**:
1. **Verify routes exist**:
   ```bash
   # Check routes in database
   docker exec north-cloud-postgres-publisher psql -U postgres -d publisher -c \
     "SELECT * FROM routes WHERE enabled = true;"

   # If no routes, create via API (see TESTING.md)
   ```

2. **Check Elasticsearch articles**:
   ```bash
   # Verify articles exist in classified_content indexes
   curl http://localhost:9200/{source}_classified_content/_search?pretty

   # Check quality scores and topics match route filters
   ```

3. **Verify Redis connection**:
   ```bash
   # Test Redis connectivity
   docker exec north-cloud-publisher-router-dev redis-cli -h redis PING

   # Expected: PONG
   ```

4. **Check deduplication**:
   ```bash
   # Articles may already be published (check publish_history)
   docker exec north-cloud-postgres-publisher psql -U postgres -d publisher -c \
     "SELECT COUNT(*) FROM publish_history WHERE article_id = 'your-article-id';"

   # If >0, article already published
   ```

### Frontend Not Loading

**Symptoms**: Blank page or 404 errors

**Diagnosis**:
```bash
# Check dashboard logs (development)
docker logs north-cloud-dashboard-dev

# Check nginx logs (production)
docker logs north-cloud-nginx
```

**Solutions**:
1. **Development**: Verify dashboard service running
   ```bash
   # Check container status
   docker ps | grep dashboard-dev

   # Check dashboard port
   curl http://localhost:3002/dashboard/publisher
   ```

2. **Production**: Verify nginx routing
   ```bash
   # Test nginx config
   docker exec north-cloud-nginx nginx -t

   # Check dashboard routing
   docker exec north-cloud-nginx cat /etc/nginx/nginx.conf | grep dashboard
   ```

3. **API proxy issues**:
   ```bash
   # Test API via nginx
   curl http://localhost/api/publisher/v1/sources

   # Expected: 401 Unauthorized (no token) or valid JSON response
   ```

### Database Migration Failures

**Symptoms**: Tables not created or schema errors

**Diagnosis**:
```bash
# Connect to database
docker exec -it north-cloud-postgres-publisher psql -U postgres -d publisher

# Check tables
\dt

# Check extensions
\dx

# Check error logs
docker logs north-cloud-postgres-publisher
```

**Solutions**:
1. **Extensions not installed**:
   ```sql
   -- Manually create extensions
   CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
   CREATE EXTENSION IF NOT EXISTS "pgcrypto";
   ```

2. **Re-run migrations**:
   ```bash
   # Drop and recreate (⚠️ destroys data)
   docker exec -it north-cloud-postgres-publisher psql -U postgres -c "DROP DATABASE publisher;"
   docker exec -it north-cloud-postgres-publisher psql -U postgres -c "CREATE DATABASE publisher;"

   # Run migrations
   docker exec -it north-cloud-postgres-publisher psql -U postgres -d publisher -f /migrations/init.sql
   docker exec -it north-cloud-postgres-publisher psql -U postgres -d publisher -f /migrations/001_initial_schema.sql
   ```

### High Memory or CPU Usage

**Symptoms**: Slow performance or container restarts

**Diagnosis**:
```bash
# Check resource usage
docker stats north-cloud-publisher-api-dev north-cloud-publisher-router-dev

# Check logs for errors
docker logs north-cloud-publisher-router-dev | grep -i error
```

**Solutions**:
1. **Router batch size too large**:
   ```bash
   # Reduce PUBLISHER_ROUTER_BATCH_SIZE in .env
   PUBLISHER_ROUTER_BATCH_SIZE=50  # Default is 100

   # Restart router
   docker restart north-cloud-publisher-router-dev
   ```

2. **Check interval too frequent**:
   ```bash
   # Increase PUBLISHER_ROUTER_CHECK_INTERVAL
   PUBLISHER_ROUTER_CHECK_INTERVAL=10m  # Default is 5m
   ```

3. **Database query optimization**:
   ```sql
   -- Add indexes if needed (already in schema, but verify)
   CREATE INDEX IF NOT EXISTS idx_routes_enabled ON routes(enabled);
   CREATE INDEX IF NOT EXISTS idx_sources_enabled ON sources(enabled);
   CREATE INDEX IF NOT EXISTS idx_channels_enabled ON channels(enabled);
   ```

---

## Post-Deployment Checklist

After completing deployment, verify:

### Day 1 (First 24 Hours)

- [ ] All services running and healthy
- [ ] API endpoints responding correctly
- [ ] Router processing routes (check logs)
- [ ] Articles being published to Redis (subscribe to channels)
- [ ] Publish history being recorded
- [ ] Frontend dashboard accessible
- [ ] Authentication working (login, JWT validation)
- [ ] No error spikes in logs
- [ ] Resource usage within normal ranges (CPU <50%, Memory <1GB)

### Week 1

- [ ] Publish history growing consistently
- [ ] No missed articles (compare to expected volume)
- [ ] Consumer services receiving and processing messages
- [ ] Deduplication working (no duplicate posts)
- [ ] Database backups running daily
- [ ] Health checks passing
- [ ] Performance metrics stable

### Month 1

- [ ] Review and archive old publish_history (>90 days)
- [ ] Database vacuum and analyze completed
- [ ] No performance degradation
- [ ] All routes active and publishing
- [ ] Monitoring alerts configured and tested
- [ ] Documentation updated with any operational notes

---

## Summary

This deployment guide covers:
- ✅ Development and production deployment procedures
- ✅ Database migration and verification
- ✅ Service verification and health checks
- ✅ Comprehensive rollback procedures
- ✅ Monitoring and maintenance strategies
- ✅ Troubleshooting common issues
- ✅ Post-deployment validation checklists

For detailed testing procedures, see [TESTING.md](TESTING.md).

For consumer integration, see [CONSUMER_GUIDE.md](CONSUMER_GUIDE.md).

For Redis message format, see [REDIS_MESSAGE_FORMAT.md](REDIS_MESSAGE_FORMAT.md).
