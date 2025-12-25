# SSL Certificate Management with Let's Encrypt

This directory contains scripts and documentation for managing SSL/TLS certificates using Let's Encrypt and Certbot in the North Cloud production environment.

## Overview

- **Domain**: northcloud.biz
- **Certificate Authority**: Let's Encrypt
- **Validation Method**: HTTP-01 (webroot)
- **Renewal**: Automatic (every 12 hours)
- **Certificate Validity**: 90 days

## Certificate Status

Current certificate details:
- **Issued**: December 25, 2025
- **Expires**: March 25, 2026
- **Days Remaining**: Check with `./scripts/check-cert-expiry.sh`

## Directory Structure

```
infrastructure/certbot/
├── README.md                    # This file
└── scripts/
    ├── check-cert-expiry.sh     # Monitor certificate expiration
    ├── renew-and-reload.sh      # Manual renewal with nginx reload
    └── reload-nginx.sh          # Simple nginx reload script
```

## Initial Certificate Setup

The initial certificate was obtained using:

```bash
docker run --rm \
  --network north-cloud_north-cloud-network \
  -v north-cloud_certbot_etc:/etc/letsencrypt \
  -v north-cloud_certbot_www:/var/www/certbot \
  certbot/certbot certonly \
  --webroot -w /var/www/certbot \
  -d northcloud.biz \
  --email jonesrussell42@gmail.com \
  --agree-tos \
  --non-interactive
```

## Automatic Renewal

The `certbot` service (defined in `docker-compose.prod.yml`) automatically checks for certificate renewal every 12 hours. The service:

1. Runs `certbot renew` to check if renewal is needed
2. Certificates are renewed when they have 30 days or less until expiration
3. Logs renewal status with timestamps
4. Certificates are automatically updated in the shared volume

### Viewing Renewal Logs

```bash
docker logs north-cloud-certbot
```

### Certbot Service Status

```bash
docker ps | grep certbot
```

## Manual Operations

### Check Certificate Expiration

Use the monitoring script to check days until expiration:

```bash
bash infrastructure/certbot/scripts/check-cert-expiry.sh
```

Output example:
```
=== SSL Certificate Expiry Check ===
Domain: northcloud.biz
Date: Thu Dec 25 18:01:31 UTC 2025

Certificate expires: Mar 25 16:22:59 2026 GMT
Days remaining: 89

✅ Certificate is valid for 89 more days
```

### Manual Certificate Renewal

If you need to manually renew the certificate:

```bash
bash infrastructure/certbot/scripts/renew-and-reload.sh
```

This script will:
1. Run certbot renewal
2. Check if renewal occurred
3. Reload nginx if certificates were renewed
4. Log all actions to `/tmp/certbot-renewal.log`

### Force Certificate Renewal (Testing)

To force renewal even if not due:

```bash
docker run --rm \
  --network north-cloud_north-cloud-network \
  -v north-cloud_certbot_etc:/etc/letsencrypt \
  -v north-cloud_certbot_www:/var/www/certbot \
  certbot/certbot renew --force-renewal
```

Then reload nginx:
```bash
docker exec north-cloud-nginx nginx -s reload
```

### Test Renewal (Dry Run)

To test renewal without actually renewing:

```bash
docker run --rm \
  --network north-cloud_north-cloud-network \
  -v north-cloud_certbot_etc:/etc/letsencrypt \
  -v north-cloud_certbot_www:/var/www/certbot \
  certbot/certbot renew --dry-run
```

### Reload Nginx After Renewal

After certificates are renewed, reload nginx to use the new certificates:

```bash
docker exec north-cloud-nginx nginx -s reload
```

Or use the helper script:
```bash
bash infrastructure/certbot/scripts/reload-nginx.sh
```

## Certificate Files

Certificates are stored in Docker volumes and mounted to nginx:

- **Volume**: `north-cloud_certbot_etc`
- **Nginx Mount**: `/etc/letsencrypt` (read-only)
- **Certificate Path**: `/etc/letsencrypt/live/northcloud.biz/fullchain.pem`
- **Private Key Path**: `/etc/letsencrypt/live/northcloud.biz/privkey.pem`

### Viewing Certificate Details

From the host:
```bash
docker run --rm \
  -v north-cloud_certbot_etc:/etc/letsencrypt \
  certbot/certbot certificates
```

From nginx container:
```bash
docker exec north-cloud-nginx \
  openssl x509 -in /etc/letsencrypt/live/northcloud.biz/fullchain.pem \
  -noout -dates -subject -issuer
```

## Monitoring and Alerts

### Daily Certificate Checks

Add this to your crontab for daily certificate monitoring:

```bash
# Check SSL certificate expiry daily at 9 AM
0 9 * * * /home/jones/north-cloud/infrastructure/certbot/scripts/check-cert-expiry.sh
```

### Email Alerts

Let's Encrypt will send email notifications to `jonesrussell42@gmail.com` when:
- Certificate is about to expire (20 days before)
- Certificate expires (if renewal fails)

## Troubleshooting

### Certificate Renewal Failed

1. **Check certbot logs**:
   ```bash
   docker logs north-cloud-certbot
   ```

2. **Verify webroot is accessible**:
   ```bash
   curl http://northcloud.biz/.well-known/acme-challenge/test
   ```

3. **Test renewal manually**:
   ```bash
   docker run --rm \
     --network north-cloud_north-cloud-network \
     -v north-cloud_certbot_etc:/etc/letsencrypt \
     -v north-cloud_certbot_www:/var/www/certbot \
     certbot/certbot renew --dry-run --verbose
   ```

### HTTPS Not Working After Renewal

1. **Check nginx is using the certificates**:
   ```bash
   docker exec north-cloud-nginx nginx -t
   ```

2. **Reload nginx**:
   ```bash
   docker exec north-cloud-nginx nginx -s reload
   ```

3. **Verify certificate files exist**:
   ```bash
   docker exec north-cloud-nginx ls -la /etc/letsencrypt/live/northcloud.biz/
   ```

### Rate Limiting

Let's Encrypt has rate limits:
- **50 certificates per registered domain per week**
- **5 duplicate certificates per week**

Use `--dry-run` for testing to avoid hitting limits.

If rate-limited, use the staging server:
```bash
docker run --rm \
  --network north-cloud_north-cloud-network \
  -v north-cloud_certbot_etc:/etc/letsencrypt \
  -v north-cloud_certbot_www:/var/www/certbot \
  certbot/certbot certonly \
  --webroot -w /var/www/certbot \
  -d northcloud.biz \
  --email jonesrussell42@gmail.com \
  --agree-tos \
  --non-interactive \
  --staging
```

## Best Practices

1. **Monitor Expiration**: Run `check-cert-expiry.sh` regularly or set up a cron job
2. **Test Renewals**: Use `--dry-run` before production changes
3. **Backup Certificates**: The certbot volume should be included in backups
4. **Keep Email Updated**: Ensure contact email is monitored for Let's Encrypt notifications
5. **Check Logs**: Review certbot logs periodically for issues

## Security Considerations

- Certificates are mounted read-only in nginx for security
- Private keys are never exposed outside the Docker volumes
- ACME challenge directory is publicly accessible (required for validation)
- HTTPS enforced with HTTP to HTTPS redirects (except ACME challenges)
- HSTS headers configured for security

## Useful Commands

```bash
# Check certificate status
bash infrastructure/certbot/scripts/check-cert-expiry.sh

# Manual renewal
bash infrastructure/certbot/scripts/renew-and-reload.sh

# View certbot logs
docker logs north-cloud-certbot --tail 50

# Test renewal
docker run --rm --network north-cloud_north-cloud-network \
  -v north-cloud_certbot_etc:/etc/letsencrypt \
  -v north-cloud_certbot_www:/var/www/certbot \
  certbot/certbot renew --dry-run

# Reload nginx
docker exec north-cloud-nginx nginx -s reload

# View certificate details
docker run --rm -v north-cloud_certbot_etc:/etc/letsencrypt \
  certbot/certbot certificates
```

## References

- [Let's Encrypt Documentation](https://letsencrypt.org/docs/)
- [Certbot Docker Documentation](https://eff-certbot.readthedocs.io/en/stable/install.html#running-with-docker)
- [Certbot Renewal Guide](https://eff-certbot.readthedocs.io/en/stable/using.html#renewing-certificates)
- [Let's Encrypt Rate Limits](https://letsencrypt.org/docs/rate-limits/)

---

**Last Updated**: December 25, 2025
**Maintainer**: jonesrussell42@gmail.com
