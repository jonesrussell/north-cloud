#!/bin/bash
# Certificate expiry monitoring script
# Checks SSL certificate expiration and alerts if renewal is needed

set -e

DOMAIN="northcloud.biz"
DAYS_WARNING=30

echo "=== SSL Certificate Expiry Check ==="
echo "Domain: $DOMAIN"
echo "Date: $(date)"
echo ""

# Get certificate expiry date
EXPIRY=$(echo | openssl s_client -connect $DOMAIN:443 -servername $DOMAIN 2>/dev/null | openssl x509 -noout -enddate 2>/dev/null | cut -d= -f2)

if [ -z "$EXPIRY" ]; then
    echo "❌ ERROR: Could not retrieve certificate information"
    exit 1
fi

echo "Certificate expires: $EXPIRY"

# Calculate days until expiry
EXPIRY_EPOCH=$(date -d "$EXPIRY" +%s)
CURRENT_EPOCH=$(date +%s)
DAYS_REMAINING=$(( ($EXPIRY_EPOCH - $CURRENT_EPOCH) / 86400 ))

echo "Days remaining: $DAYS_REMAINING"
echo ""

if [ $DAYS_REMAINING -lt 0 ]; then
    echo "🚨 CRITICAL: Certificate has EXPIRED!"
    exit 2
elif [ $DAYS_REMAINING -lt $DAYS_WARNING ]; then
    echo "⚠️  WARNING: Certificate expires in less than $DAYS_WARNING days"
    echo "Run certificate renewal: docker run --rm --network north-cloud_north-cloud-network -v north-cloud_certbot_etc:/etc/letsencrypt -v north-cloud_certbot_www:/var/www/certbot certbot/certbot renew --force-renewal"
    exit 1
else
    echo "✅ Certificate is valid for $DAYS_REMAINING more days"
    exit 0
fi
