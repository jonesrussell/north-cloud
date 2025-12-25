#!/bin/sh
# This script is executed by certbot after successful certificate renewal
# It reloads nginx to pick up the new certificates

set -e

echo "$(date): Certificate renewed, reloading nginx..."

# Send reload signal to nginx container
# Using docker exec to send reload signal to nginx
docker exec north-cloud-nginx nginx -s reload

echo "$(date): Nginx reloaded successfully"
