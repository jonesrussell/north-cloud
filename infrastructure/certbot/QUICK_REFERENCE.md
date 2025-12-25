# SSL Certificate Quick Reference

## Common Commands

### Check Certificate Status
```bash
# Quick expiry check
bash infrastructure/certbot/scripts/check-cert-expiry.sh

# Detailed certificate info
docker run --rm -v north-cloud_certbot_etc:/etc/letsencrypt \
  certbot/certbot certificates
```

### Manual Renewal
```bash
# Renew and reload nginx (recommended)
bash infrastructure/certbot/scripts/renew-and-reload.sh

# Just renew (manual nginx reload needed)
docker run --rm --network north-cloud_north-cloud-network \
  -v north-cloud_certbot_etc:/etc/letsencrypt \
  -v north-cloud_certbot_www:/var/www/certbot \
  certbot/certbot renew

# Reload nginx after renewal
docker exec north-cloud-nginx nginx -s reload
```

### Testing
```bash
# Test renewal without actually renewing (dry-run)
docker run --rm --network north-cloud_north-cloud-network \
  -v north-cloud_certbot_etc:/etc/letsencrypt \
  -v north-cloud_certbot_www:/var/www/certbot \
  certbot/certbot renew --dry-run
```

### Monitoring
```bash
# View certbot service logs
docker logs north-cloud-certbot

# Check certbot service status
docker ps | grep certbot

# Test HTTPS
curl -I https://northcloud.biz/
```

## Automatic Renewal

The certbot service runs every 12 hours and:
- Checks if renewal is needed (30 days before expiry)
- Renews certificate automatically
- Logs the result

**Note**: After automatic renewal, you must manually reload nginx:
```bash
docker exec north-cloud-nginx nginx -s reload
```

## Emergency Contact

If certificates expire or renewal fails:

1. Check logs: `docker logs north-cloud-certbot`
2. Force renewal: `bash infrastructure/certbot/scripts/renew-and-reload.sh`
3. Contact: jonesrussell42@gmail.com

## Next Renewal Due

- **Current Expiry**: March 25, 2026
- **Auto-Renewal Start**: ~February 23, 2026 (30 days before)
- **Email Alerts**: 20 days before expiry
