#!/bin/bash
# SSL Certificate Renewal and Nginx Reload Script
# This script renews SSL certificates and reloads nginx if successful

set -e

DOMAIN="northcloud.biz"
LOG_FILE="/tmp/certbot-renewal.log"

echo "=== SSL Certificate Renewal ===" | tee -a $LOG_FILE
echo "Date: $(date)" | tee -a $LOG_FILE
echo "Domain: $DOMAIN" | tee -a $LOG_FILE
echo "" | tee -a $LOG_FILE

# Run certbot renewal
echo "Running certbot renewal..." | tee -a $LOG_FILE
if docker run --rm --network north-cloud_north-cloud-network \
  -v north-cloud_certbot_etc:/etc/letsencrypt \
  -v north-cloud_certbot_www:/var/www/certbot \
  certbot/certbot renew 2>&1 | tee -a $LOG_FILE; then

    echo "" | tee -a $LOG_FILE
    echo "✅ Certbot renewal check completed successfully" | tee -a $LOG_FILE

    # Check if renewal actually happened (certbot outputs this)
    if grep -q "Certificate not yet due for renewal" $LOG_FILE; then
        echo "ℹ️  No renewal needed at this time" | tee -a $LOG_FILE
    else
        echo "🔄 Certificate renewed! Reloading nginx..." | tee -a $LOG_FILE

        # Reload nginx
        if docker exec north-cloud-nginx nginx -s reload 2>&1 | tee -a $LOG_FILE; then
            echo "✅ Nginx reloaded successfully" | tee -a $LOG_FILE
        else
            echo "❌ ERROR: Failed to reload nginx" | tee -a $LOG_FILE
            exit 1
        fi
    fi
else
    echo "❌ ERROR: Certbot renewal failed" | tee -a $LOG_FILE
    exit 1
fi

echo "" | tee -a $LOG_FILE
echo "=== Renewal Complete ===" | tee -a $LOG_FILE
echo "$(date)" | tee -a $LOG_FILE
