# SendGrid + Mailpit Email Setup

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable Grafana email alerts via SendGrid in production and Mailpit for local dev.

**Architecture:** Config-only changes — set env vars for SendGrid on prod, add Mailpit container for dev SMTP testing. No code changes.

**Tech Stack:** SendGrid SMTP relay, Mailpit (MailHog successor), Docker Compose

---

### Task 1: Update docker-compose.prod.yml — add from address overrides

**Files:**
- Modify: `docker-compose.prod.yml:578-580`

**Step 1: Add explicit from address and from name to prod Grafana env**

After the existing `GF_SMTP_PASSWORD` line, add:
```yaml
      GF_SMTP_FROM_ADDRESS: "noreply@northcloud.biz"
      GF_SMTP_FROM_NAME: "North Cloud Alerts"
```

**Step 2: Verify YAML is valid**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.prod.yml config --quiet`

---

### Task 2: Add Mailpit to docker-compose.dev.yml

**Files:**
- Modify: `docker-compose.dev.yml` (add mailpit service + grafana SMTP overrides)

**Step 1: Add mailpit service before the volumes section**

```yaml
  mailpit:
    image: axllent/mailpit:v1.21
    profiles:
      - observability
    ports:
      - "8025:8025"   # Web UI
      - "1025:1025"   # SMTP
    deploy:
      resources:
        limits:
          cpus: "0.1"
          memory: 64M
    networks:
      - north-cloud-network
```

**Step 2: Add Grafana SMTP overrides pointing to mailpit**

In the existing `grafana:` section, add to environment:
```yaml
      GF_SMTP_ENABLED: "true"
      GF_SMTP_HOST: "mailpit:1025"
      GF_SMTP_USER: ""
      GF_SMTP_PASSWORD: ""
      GF_SMTP_FROM_ADDRESS: "noreply@northcloud.biz"
      GF_SMTP_FROM_NAME: "North Cloud Alerts (Dev)"
      GRAFANA_ALERT_EMAIL: "dev@localhost"
```

---

### Task 3: Update .env.example

**Files:**
- Modify: `.env.example:291-295`

**Step 1: Update SMTP defaults and add Mailpit comment**

```
# Grafana SMTP (SendGrid relay for email alerts)
# Production: set SENDGRID_API_KEY and GRAFANA_ALERT_EMAIL
# Development: Mailpit catches all mail at http://localhost:8025 (no config needed)
GF_SMTP_ENABLED=false
SENDGRID_API_KEY=
GF_SMTP_FROM_ADDRESS=noreply@northcloud.biz
GRAFANA_ALERT_EMAIL=alerts@example.com
```

---

### Task 4: Set production env vars and restart Grafana

**Step 1: Add SENDGRID_API_KEY to prod .env**

```bash
ssh jones@northcloud.biz
echo 'SENDGRID_API_KEY=<your-sendgrid-api-key>' >> /opt/north-cloud/.env
echo 'GRAFANA_ALERT_EMAIL=russell@web.net' >> /opt/north-cloud/.env
```

**Step 2: Copy updated docker-compose.prod.yml to server**

**Step 3: Restart Grafana**

```bash
cd /opt/north-cloud
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d grafana
```

**Step 4: Verify SMTP env vars are set**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml exec grafana env | grep -E 'SMTP|SENDGRID|ALERT_EMAIL'
```

---

### Task 5: Send test email

**Step 1: Use Grafana API to send a test notification**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml exec grafana \
  wget -qO- --post-data='{}' \
  --header='Content-Type: application/json' \
  --header='Authorization: Basic BASE64_CREDS' \
  http://localhost:3000/grafana/api/admin/provisioning/notifications
```

Or trigger via Grafana UI: Alerting > Contact Points > Email > Test.

---

### Task 6: Commit all changes

```bash
git add docker-compose.prod.yml docker-compose.dev.yml .env.example
git commit -m "feat(grafana): SendGrid email alerts + Mailpit for dev"
```
