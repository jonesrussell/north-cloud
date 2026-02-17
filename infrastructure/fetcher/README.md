# Fetcher Worker Droplet

Runs fetcher workers on a separate DigitalOcean droplet with a fresh IP address, keeping crawling traffic isolated from the main server.

## Prerequisites

- `doctl` CLI authenticated
- SSH key registered with DigitalOcean
- VPC created in the target region

## Provisioning

```bash
# Create VPC (if not exists)
doctl vpcs create --name north-cloud-vpc --region tor1 --ip-range 10.132.0.0/20

# Create droplet
doctl compute droplet create nc-fetcher-01 \
  --region tor1 \
  --size s-1vcpu-1gb \
  --image docker-24-04 \
  --vpc-uuid <vpc-id> \
  --ssh-keys <key-fingerprint> \
  --tag-names north-cloud,fetcher \
  --user-data-file infrastructure/fetcher/cloud-init.yml

# Create firewall (SSH only inbound, all outbound)
doctl compute firewall create \
  --name nc-fetcher-fw \
  --tag-names fetcher \
  --inbound-rules "protocol:tcp,ports:22,address:YOUR_IP/32" \
  --outbound-rules "protocol:tcp,ports:all,address:0.0.0.0/0 protocol:udp,ports:all,address:0.0.0.0/0"
```

## Configuration

After provisioning, SSH in and edit `/opt/fetcher/.env`:

```bash
ssh root@<droplet-ip>
vi /opt/fetcher/.env
```

Set the VPC-internal IPs for:
- `FETCHER_DATABASE_URL` - crawler PostgreSQL (via VPC)
- `FETCHER_ELASTICSEARCH_URL` - Elasticsearch (via VPC)
- `FETCHER_SOURCE_MANAGER_URL` - source-manager API (via VPC)

## Starting

```bash
cd /opt/fetcher
docker compose up -d
docker compose logs -f
```

## Updating

```bash
cd /opt/fetcher
docker compose pull
docker compose up -d
```
