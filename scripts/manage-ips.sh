#!/bin/bash
# IP Management Script for Crawler Proxy Rotation
# Manages DigitalOcean Reserved IPs attached to the production droplet.
# Squid runs in a Docker container (network_mode: host) and binds outbound
# connections to these IPs. The crawler connects to Squid via the Docker gateway.
#
# Usage:
#   ./scripts/manage-ips.sh add --region <region>
#   ./scripts/manage-ips.sh remove --ip <ip>
#   ./scripts/manage-ips.sh list
#   ./scripts/manage-ips.sh validate
#   ./scripts/manage-ips.sh regenerate-squid
#
# Environment variables:
#   DIGITALOCEAN_ACCESS_TOKEN - DigitalOcean API token (required for add/remove/validate, used by doctl)
#   INVENTORY_FILE     - Path to IP inventory file (default: /home/deployer/north-cloud/proxy-ips.conf)
#   SQUID_CONF         - Path to Squid config (default: /home/deployer/north-cloud/squid/squid.conf)
#   NETWORK_INTERFACE  - Network interface for IPs (default: eth0)
#
# Prerequisites:
#   - doctl (DigitalOcean CLI, authenticated)
#   - jq
#   - docker compose (Squid runs as a container)
#   - ip (iproute2)

set -euo pipefail

# =============================================================================
# Constants
# =============================================================================

INVENTORY_FILE="${INVENTORY_FILE:-/home/deployer/north-cloud/proxy-ips.conf}"
SQUID_CONF="${SQUID_CONF:-/home/deployer/north-cloud/squid/squid.conf}"
SQUID_LOG_DIR="${SQUID_LOG_DIR:-/home/deployer/north-cloud/squid/logs}"
NETWORK_INTERFACE="${NETWORK_INTERFACE:-eth0}"
BASE_PORT=3128
DO_METADATA_URL="http://169.254.169.254/metadata/v1"
DOCKER_NETWORKS="172.16.0.0/12"  # All Docker subnets (bridge + compose networks)
LOCALHOST_NETWORK="127.0.0.0/8"
NETPLAN_DIR="/etc/netplan"
CLOUD_INIT_NETWORK_CFG="/etc/cloud/cloud.cfg.d/99-disable-network-config.cfg"
COMPOSE_DIR="${COMPOSE_DIR:-/home/deployer/north-cloud}"
SQUID_CONTAINER="north-cloud-squid"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# =============================================================================
# Utility Functions
# =============================================================================

log_info() {
  echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
  echo -e "${GREEN}[OK]${NC} $*"
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $*" >&2
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $*" >&2
}

die() {
  log_error "$@"
  exit 1
}

require_root() {
  if [[ $EUID -ne 0 ]]; then
    die "This command must be run as root (use sudo)"
  fi
}

require_doctl() {
  if ! command -v doctl &>/dev/null; then
    die "doctl is not installed. Install: https://docs.digitalocean.com/reference/doctl/how-to/install/"
  fi
}

require_jq() {
  if ! command -v jq &>/dev/null; then
    die "jq is not installed. Install: apt-get install jq"
  fi
}

require_docker_compose() {
  if ! command -v docker &>/dev/null; then
    die "docker is not installed"
  fi
  if ! docker compose version &>/dev/null; then
    die "docker compose plugin is not installed"
  fi
}

# Get the droplet ID from the DO metadata API.
get_droplet_id() {
  local droplet_id
  droplet_id=$(curl -sf "${DO_METADATA_URL}/id" 2>/dev/null) \
    || die "Failed to query DO metadata API. Are you running on a DigitalOcean droplet?"
  echo "$droplet_id"
}

# Ensure the inventory file exists and is readable.
ensure_inventory() {
  if [[ ! -f "$INVENTORY_FILE" ]]; then
    local inventory_dir
    inventory_dir=$(dirname "$INVENTORY_FILE")
    mkdir -p "$inventory_dir"
    touch "$INVENTORY_FILE"
    log_info "Created inventory file: $INVENTORY_FILE"
  fi
}

# Read raw inventory lines (RESERVED_IP:ANCHOR_IP per line, skip blanks/comments).
read_inventory_raw() {
  if [[ ! -f "$INVENTORY_FILE" ]]; then
    return
  fi
  grep -v '^\s*#' "$INVENTORY_FILE" | grep -v '^\s*$' || true
}

# Read reserved IPs from the inventory file (returns just the reserved IP column).
read_inventory() {
  read_inventory_raw | cut -d: -f1
}

# Look up the anchor IP for a given reserved IP from the inventory file.
get_anchor_for_reserved() {
  local reserved_ip="$1"
  read_inventory_raw | grep "^${reserved_ip}:" | cut -d: -f2 | head -1
}

# Count IPs in the inventory file.
count_inventory() {
  read_inventory_raw | wc -l | tr -d ' '
}

# Disable cloud-init network management to prevent it from overwriting netplan.
disable_cloud_init_network() {
  if [[ -f "$CLOUD_INIT_NETWORK_CFG" ]]; then
    return
  fi
  log_info "Disabling cloud-init network management..."
  mkdir -p "$(dirname "$CLOUD_INIT_NETWORK_CFG")"
  echo "network: {config: disabled}" > "$CLOUD_INIT_NETWORK_CFG"
  log_success "Cloud-init network management disabled"
}

# Compute the MD5 hash of the inventory file contents (for Squid config header).
inventory_hash() {
  if [[ -f "$INVENTORY_FILE" ]]; then
    md5sum "$INVENTORY_FILE" | cut -d' ' -f1
  else
    echo "empty"
  fi
}

# =============================================================================
# Command: add
# =============================================================================

cmd_add() {
  local region=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --region)
        region="$2"
        shift 2
        ;;
      *)
        die "Unknown option for add: $1"
        ;;
    esac
  done

  if [[ -z "$region" ]]; then
    die "Usage: $0 add --region <region>"
  fi

  require_root
  require_doctl
  require_jq
  require_docker_compose
  ensure_inventory

  local droplet_id
  droplet_id=$(get_droplet_id)
  log_info "Droplet ID: $droplet_id"

  # Step 1: Create a Reserved IP in the given region.
  log_info "Creating Reserved IP in region '$region'..."
  local create_output
  create_output=$(doctl compute reserved-ip create --region "$region" --output json) \
    || die "Failed to create Reserved IP"

  local reserved_ip
  reserved_ip=$(echo "$create_output" | jq -r '.[0].ip')
  if [[ -z "$reserved_ip" || "$reserved_ip" == "null" ]]; then
    die "Failed to parse Reserved IP from doctl output"
  fi
  log_success "Reserved IP created: $reserved_ip"

  # Step 2: Assign the Reserved IP to this droplet.
  log_info "Assigning $reserved_ip to droplet $droplet_id..."
  doctl compute reserved-ip-action assign "$reserved_ip" "$droplet_id" --output json >/dev/null \
    || die "Failed to assign Reserved IP $reserved_ip to droplet $droplet_id"
  log_success "Reserved IP assigned to droplet"

  # Step 3: Retrieve the anchor IP from the DO metadata API.
  log_info "Retrieving anchor IP from metadata API..."
  local anchor_ip
  anchor_ip=$(retrieve_anchor_ip)
  log_success "Anchor IP: $anchor_ip"

  # Step 4: Configure the network interface.
  configure_interface "$anchor_ip"

  # Step 5: Persist via netplan.
  persist_netplan "$anchor_ip"

  # Step 6: Update inventory file.
  echo "${reserved_ip}:${anchor_ip}" >> "$INVENTORY_FILE"
  log_success "Added ${reserved_ip}:${anchor_ip} to inventory"

  # Step 7: Regenerate Squid config and reload.
  regenerate_and_reload_squid

  echo ""
  log_success "IP $reserved_ip added successfully"
  log_info "  Reserved IP: $reserved_ip"
  log_info "  Anchor IP:   $anchor_ip"
  log_info "  Interface:   $NETWORK_INTERFACE"
  log_info "  Total IPs:   $(count_inventory)"
}

# Retrieve the anchor IP from the DO metadata API.
# Retries until the metadata endpoint returns a valid anchor IP.
retrieve_anchor_ip() {
  local max_attempts=30
  local wait_seconds=2
  local attempt=1
  while [[ $attempt -le $max_attempts ]]; do
    local anchor
    anchor=$(curl -sf "${DO_METADATA_URL}/interfaces/public/0/anchor_ipv4/address" 2>/dev/null || true)
    if [[ -n "$anchor" && "$anchor" != "null" ]]; then
      echo "$anchor"
      return 0
    fi
    log_info "Waiting for anchor IP (attempt $attempt/$max_attempts)..."
    sleep "$wait_seconds"
    attempt=$((attempt + 1))
  done
  die "Timed out waiting for anchor IP from metadata API"
}

# Add the anchor IP to the network interface.
configure_interface() {
  local anchor_ip="$1"
  local cidr_suffix="/16"

  # Check if already configured.
  if ip addr show "$NETWORK_INTERFACE" | grep -q "$anchor_ip"; then
    log_info "Anchor IP $anchor_ip already configured on $NETWORK_INTERFACE"
    return 0
  fi

  log_info "Adding $anchor_ip$cidr_suffix to $NETWORK_INTERFACE..."
  ip addr add "${anchor_ip}${cidr_suffix}" dev "$NETWORK_INTERFACE" \
    || die "Failed to add $anchor_ip to $NETWORK_INTERFACE"
  log_success "Interface configured"
}

# Persist the anchor IP in netplan configuration.
persist_netplan() {
  local anchor_ip="$1"
  local netplan_file="${NETPLAN_DIR}/60-floating-ips.yaml"
  local cidr_suffix="/16"

  disable_cloud_init_network

  # Read existing addresses from the netplan file if it exists.
  local existing_addresses=""
  if [[ -f "$netplan_file" ]]; then
    existing_addresses=$(grep -oP '\d+\.\d+\.\d+\.\d+/\d+' "$netplan_file" 2>/dev/null || true)
  fi

  # Check if this anchor IP is already persisted.
  if echo "$existing_addresses" | grep -q "$anchor_ip"; then
    log_info "Anchor IP $anchor_ip already in netplan"
    return 0
  fi

  # Build the addresses list.
  local all_addresses=""
  if [[ -n "$existing_addresses" ]]; then
    all_addresses="$existing_addresses"$'\n'"${anchor_ip}${cidr_suffix}"
  else
    all_addresses="${anchor_ip}${cidr_suffix}"
  fi

  log_info "Writing netplan config: $netplan_file"

  # Write the netplan file atomically.
  local tmp_netplan
  tmp_netplan=$(mktemp "${netplan_file}.XXXXXX")

  {
    echo "# Managed by manage-ips.sh - DO NOT EDIT MANUALLY"
    echo "# Generated: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    echo "network:"
    echo "  version: 2"
    echo "  ethernets:"
    echo "    $NETWORK_INTERFACE:"
    echo "      addresses:"
    while IFS= read -r addr; do
      [[ -n "$addr" ]] && echo "        - $addr"
    done <<< "$all_addresses"
  } > "$tmp_netplan"

  chmod 600 "$tmp_netplan"
  mv "$tmp_netplan" "$netplan_file"
  netplan apply 2>/dev/null || log_warn "netplan apply returned non-zero (may be expected)"
  log_success "Netplan config persisted"
}

# =============================================================================
# Command: remove
# =============================================================================

cmd_remove() {
  local ip=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --ip)
        ip="$2"
        shift 2
        ;;
      *)
        die "Unknown option for remove: $1"
        ;;
    esac
  done

  if [[ -z "$ip" ]]; then
    die "Usage: $0 remove --ip <ip>"
  fi

  require_root
  require_doctl
  require_jq
  require_docker_compose
  ensure_inventory

  # Verify the IP is in our inventory.
  if ! grep -q "^${ip}:" "$INVENTORY_FILE" 2>/dev/null; then
    die "IP $ip not found in inventory ($INVENTORY_FILE)"
  fi

  local droplet_id
  droplet_id=$(get_droplet_id)

  # Step 1: Retrieve the anchor IP from inventory mapping.
  log_info "Looking up anchor IP for Reserved IP $ip..."
  local anchor_ip
  anchor_ip=$(get_anchor_for_reserved "$ip")

  # Step 2: Remove the anchor IP from the network interface.
  if [[ -n "$anchor_ip" ]]; then
    remove_interface "$anchor_ip"
    remove_netplan "$anchor_ip"
  else
    log_warn "Could not determine anchor IP; skipping interface cleanup"
  fi

  # Step 3: Unassign the Reserved IP from the droplet.
  log_info "Unassigning Reserved IP $ip from droplet $droplet_id..."
  doctl compute reserved-ip-action unassign "$ip" --output json >/dev/null 2>&1 \
    || log_warn "Unassign failed (may already be unassigned)"

  # Step 4: Release the Reserved IP.
  log_info "Releasing Reserved IP $ip..."
  doctl compute reserved-ip delete "$ip" --force \
    || log_warn "Failed to release Reserved IP $ip (may already be released)"
  log_success "Reserved IP released"

  # Step 5: Update inventory file.
  remove_from_inventory "$ip"

  # Step 6: Regenerate Squid config and reload.
  regenerate_and_reload_squid

  echo ""
  log_success "IP $ip removed successfully"
  log_info "  Total IPs remaining: $(count_inventory)"
}

# Remove the anchor IP from the network interface.
remove_interface() {
  local anchor_ip="$1"
  local cidr_suffix="/16"

  if ! ip addr show "$NETWORK_INTERFACE" | grep -q "$anchor_ip"; then
    log_info "Anchor IP $anchor_ip not found on $NETWORK_INTERFACE (already removed)"
    return 0
  fi

  log_info "Removing $anchor_ip$cidr_suffix from $NETWORK_INTERFACE..."
  ip addr del "${anchor_ip}${cidr_suffix}" dev "$NETWORK_INTERFACE" \
    || log_warn "Failed to remove $anchor_ip from $NETWORK_INTERFACE"
  log_success "Interface address removed"
}

# Remove the anchor IP from netplan configuration.
remove_netplan() {
  local anchor_ip="$1"
  local netplan_file="${NETPLAN_DIR}/60-floating-ips.yaml"

  if [[ ! -f "$netplan_file" ]]; then
    return 0
  fi

  # Read remaining addresses (excluding the one being removed).
  local remaining_addresses
  remaining_addresses=$(grep -oP '\d+\.\d+\.\d+\.\d+/\d+' "$netplan_file" 2>/dev/null \
    | grep -v "^${anchor_ip}/" || true)

  if [[ -z "$remaining_addresses" ]]; then
    # No addresses left; remove the netplan file.
    rm -f "$netplan_file"
    log_info "Removed netplan config (no addresses remaining)"
  else
    # Rewrite with remaining addresses.
    local tmp_netplan
    tmp_netplan=$(mktemp "${netplan_file}.XXXXXX")

    {
      echo "# Managed by manage-ips.sh - DO NOT EDIT MANUALLY"
      echo "# Generated: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
      echo "network:"
      echo "  version: 2"
      echo "  ethernets:"
      echo "    $NETWORK_INTERFACE:"
      echo "      addresses:"
      while IFS= read -r addr; do
        [[ -n "$addr" ]] && echo "        - $addr"
      done <<< "$remaining_addresses"
    } > "$tmp_netplan"

    chmod 600 "$tmp_netplan"
    mv "$tmp_netplan" "$netplan_file"
    log_info "Updated netplan config"
  fi

  netplan apply 2>/dev/null || log_warn "netplan apply returned non-zero"
}

# Remove an IP from the inventory file.
remove_from_inventory() {
  local ip="$1"
  local tmp_inventory
  tmp_inventory=$(mktemp "${INVENTORY_FILE}.XXXXXX")

  grep -v "^${ip}:" "$INVENTORY_FILE" > "$tmp_inventory" || true
  mv "$tmp_inventory" "$INVENTORY_FILE"
  log_success "Removed $ip from inventory"
}

# =============================================================================
# Command: list
# =============================================================================

cmd_list() {
  ensure_inventory

  local lines
  lines=$(read_inventory_raw)

  if [[ -z "$lines" ]]; then
    log_info "No IPs in inventory ($INVENTORY_FILE)"
    return 0
  fi

  local ip_count
  ip_count=$(echo "$lines" | wc -l | tr -d ' ')

  echo ""
  echo -e "${BLUE}=== Proxy IP Inventory ===${NC}"
  echo "File: $INVENTORY_FILE"
  echo "Total: $ip_count IP(s)"
  echo ""

  printf "%-5s  %-18s  %-18s  %-8s  %-8s\n" "PORT" "RESERVED IP" "ANCHOR IP" "IFACE" "SQUID"
  printf "%-5s  %-18s  %-18s  %-8s  %-8s\n" "-----" "------------------" "------------------" "--------" "--------"

  local port=$BASE_PORT
  while IFS=: read -r reserved_ip anchor_ip; do
    [[ -z "$reserved_ip" ]] && continue

    # Check if the anchor IP is bound to an interface.
    local iface_status="down"
    if [[ -n "$anchor_ip" ]] && ip addr show 2>/dev/null | grep -q "$anchor_ip"; then
      iface_status="up"
    fi

    # Check if Squid config references the anchor IP.
    local squid_status="absent"
    if [[ -f "$SQUID_CONF" ]] && [[ -n "$anchor_ip" ]] && grep -q "$anchor_ip" "$SQUID_CONF" 2>/dev/null; then
      squid_status="present"
    fi

    local iface_color="${RED}"
    [[ "$iface_status" == "up" ]] && iface_color="${GREEN}"

    local squid_color="${RED}"
    [[ "$squid_status" == "present" ]] && squid_color="${GREEN}"

    printf "%-5s  %-18s  %-18s  ${iface_color}%-8s${NC}  ${squid_color}%-8s${NC}\n" \
      "$port" "$reserved_ip" "${anchor_ip:---}" "$iface_status" "$squid_status"

    port=$((port + 1))
  done <<< "$lines"

  echo ""
}

# =============================================================================
# Command: validate
# =============================================================================

cmd_validate() {
  require_doctl
  require_jq
  ensure_inventory

  local droplet_id
  droplet_id=$(get_droplet_id)
  local drift_found=false

  echo ""
  echo -e "${BLUE}=== IP Validation Report ===${NC}"
  echo "Droplet ID: $droplet_id"
  echo "Inventory:  $INVENTORY_FILE"
  echo "Squid:      $SQUID_CONF"
  echo ""

  # Source 1: DigitalOcean API — Reserved IPs assigned to this droplet.
  log_info "Querying DigitalOcean API..."
  local do_ips_json
  do_ips_json=$(doctl compute reserved-ip list --output json 2>/dev/null) \
    || die "Failed to query DigitalOcean API"

  local do_ips
  do_ips=$(echo "$do_ips_json" \
    | jq -r ".[] | select(.droplet.id == ${droplet_id}) | .ip" 2>/dev/null \
    | sort || true)

  local do_ip_count=0
  if [[ -n "$do_ips" ]]; then
    do_ip_count=$(echo "$do_ips" | wc -l | tr -d ' ')
  fi
  echo "DO API:     $do_ip_count Reserved IP(s) assigned to this droplet"

  # Source 2: Inventory file (reserved IPs for DO API comparison).
  local inv_reserved_ips
  inv_reserved_ips=$(read_inventory | sort)

  # Inventory anchor IPs (for Squid and interface comparison).
  local inv_anchor_ips
  inv_anchor_ips=$(read_inventory_raw | cut -d: -f2 | sort)

  local inv_ip_count=0
  if [[ -n "$inv_reserved_ips" ]]; then
    inv_ip_count=$(echo "$inv_reserved_ips" | wc -l | tr -d ' ')
  fi
  echo "Inventory:  $inv_ip_count IP(s) in file"

  # Source 3: Network interfaces.
  local iface_ips
  iface_ips=$(ip addr show "$NETWORK_INTERFACE" 2>/dev/null \
    | grep -oP 'inet \K\d+\.\d+\.\d+\.\d+' \
    | sort || true)

  local iface_ip_count=0
  if [[ -n "$iface_ips" ]]; then
    iface_ip_count=$(echo "$iface_ips" | wc -l | tr -d ' ')
  fi
  echo "Interface:  $iface_ip_count IP(s) on $NETWORK_INTERFACE"

  # Source 4: Squid config — tcp_outgoing_address directives.
  local squid_ips=""
  if [[ -f "$SQUID_CONF" ]]; then
    squid_ips=$(grep -oP 'tcp_outgoing_address \K\d+\.\d+\.\d+\.\d+' "$SQUID_CONF" 2>/dev/null \
      | sort -u || true)
  fi

  local squid_ip_count=0
  if [[ -n "$squid_ips" ]]; then
    squid_ip_count=$(echo "$squid_ips" | wc -l | tr -d ' ')
  fi
  echo "Squid:      $squid_ip_count IP(s) in config"
  echo ""

  # Compare: DO API vs Inventory (both use reserved/public IPs).
  echo -e "${BLUE}--- DO API vs Inventory ---${NC}"
  compare_ip_lists "DO API" "$do_ips" "Inventory" "$inv_reserved_ips" || drift_found=true

  # Compare: Inventory anchor IPs vs Squid (both use anchor IPs).
  echo -e "${BLUE}--- Inventory (anchor) vs Squid ---${NC}"
  compare_ip_lists "Inventory" "$inv_anchor_ips" "Squid" "$squid_ips" || drift_found=true

  # Compare: Inventory anchor IPs vs Interfaces.
  echo -e "${BLUE}--- Inventory (anchor) vs Interface ---${NC}"
  validate_inventory_on_interfaces "$inv_anchor_ips" "$iface_ips" || drift_found=true

  echo ""
  if [[ "$drift_found" == "true" ]]; then
    log_warn "Drift detected. Review the report above."
    log_info "To fix Squid config: $0 regenerate-squid"
    return 1
  else
    log_success "No drift detected. All sources are consistent."
    return 0
  fi
}

# Compare two sorted IP lists and report differences.
compare_ip_lists() {
  local name_a="$1"
  local list_a="$2"
  local name_b="$3"
  local list_b="$4"
  local drift=false

  # IPs in A but not in B.
  local only_a
  only_a=$(comm -23 <(echo "$list_a") <(echo "$list_b") 2>/dev/null || true)
  if [[ -n "$only_a" ]]; then
    while IFS= read -r ip; do
      [[ -n "$ip" ]] && echo -e "  ${YELLOW}In $name_a but not $name_b: $ip${NC}"
    done <<< "$only_a"
    drift=true
  fi

  # IPs in B but not in A.
  local only_b
  only_b=$(comm -13 <(echo "$list_a") <(echo "$list_b") 2>/dev/null || true)
  if [[ -n "$only_b" ]]; then
    while IFS= read -r ip; do
      [[ -n "$ip" ]] && echo -e "  ${YELLOW}In $name_b but not $name_a: $ip${NC}"
    done <<< "$only_b"
    drift=true
  fi

  if [[ "$drift" == "false" ]]; then
    echo -e "  ${GREEN}Consistent${NC}"
  fi

  [[ "$drift" == "false" ]]
}

# Validate that inventory IPs appear in the interface IP list.
validate_inventory_on_interfaces() {
  local inv_ips="$1"
  local iface_ips="$2"
  local drift=false

  if [[ -z "$inv_ips" ]]; then
    echo -e "  ${GREEN}No inventory IPs to check${NC}"
    return 0
  fi

  while IFS= read -r ip; do
    [[ -z "$ip" ]] && continue
    if ! echo "$iface_ips" | grep -q "^${ip}$"; then
      echo -e "  ${YELLOW}Inventory IP $ip not found on interface $NETWORK_INTERFACE${NC}"
      drift=true
    fi
  done <<< "$inv_ips"

  if [[ "$drift" == "false" ]]; then
    echo -e "  ${GREEN}All inventory IPs found on interface${NC}"
  fi

  [[ "$drift" == "false" ]]
}

# =============================================================================
# Command: regenerate-squid
# =============================================================================

cmd_regenerate_squid() {
  require_root
  require_docker_compose
  ensure_inventory

  # Ensure config and log directories exist.
  mkdir -p "$(dirname "$SQUID_CONF")"
  mkdir -p "$SQUID_LOG_DIR"

  local ips
  ips=$(read_inventory_raw | cut -d: -f2)
  local ip_count
  ip_count=$(count_inventory)
  local hash
  hash=$(inventory_hash)

  log_info "Generating Squid config from $ip_count IP(s)..."

  local tmp_conf
  tmp_conf=$(mktemp "${SQUID_CONF}.XXXXXX")

  generate_squid_config "$ips" "$ip_count" "$hash" > "$tmp_conf"

  # Validate the generated config using the Squid container image.
  log_info "Validating Squid config..."
  if ! docker run --rm -v "${tmp_conf}:/etc/squid/squid.conf:ro" \
      ubuntu/squid:latest squid -k parse 2>/dev/null; then
    rm -f "$tmp_conf"
    die "Generated Squid config failed validation (squid -k parse)"
  fi
  log_success "Squid config validated"

  # Atomic write: move temp file into place.
  chmod 644 "$tmp_conf"
  mv "$tmp_conf" "$SQUID_CONF"
  log_success "Squid config written to $SQUID_CONF"

  log_info "IP count: $ip_count, Base port: $BASE_PORT"
}

# Generate the full Squid configuration.
generate_squid_config() {
  local ips="$1"
  local ip_count="$2"
  local hash="$3"

  cat <<HEADER
# =============================================================================
# Squid Forward Proxy Configuration
# Managed by manage-ips.sh - DO NOT EDIT MANUALLY
#
# Generated: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
# Inventory: $INVENTORY_FILE
# Inventory hash: $hash
# IP count: $ip_count
# =============================================================================

# -----------------------------------------------------------------------------
# Global Settings
# -----------------------------------------------------------------------------

# Disable caching (pure forward proxy)
cache deny all
cache_dir null /tmp
cache_mem 0 MB

# DNS
dns_v4_first on

# Shutdown timeout
shutdown_lifetime 5 seconds

# -----------------------------------------------------------------------------
# Access Control
# -----------------------------------------------------------------------------

# Allow connections from localhost and Docker networks only
acl localhost_acl src $LOCALHOST_NETWORK
acl docker_networks src $DOCKER_NETWORKS
acl SSL_ports port 443
acl Safe_ports port 80
acl Safe_ports port 443
acl Safe_ports port 1025-65535
acl CONNECT method CONNECT

# Deny requests to unsafe ports
http_access deny !Safe_ports
http_access deny CONNECT !SSL_ports

# Allow localhost and Docker networks
http_access allow localhost_acl
http_access allow docker_networks

# Deny everything else
http_access deny all

HEADER

  # Generate per-IP port bindings and outgoing address mappings.
  if [[ -z "$ips" ]]; then
    cat <<NO_IPS
# -----------------------------------------------------------------------------
# No IPs configured - single port fallback
# -----------------------------------------------------------------------------

http_port $BASE_PORT

NO_IPS
  else
    echo "# -----------------------------------------------------------------------------"
    echo "# Port Bindings (one port per IP)"
    echo "# -----------------------------------------------------------------------------"
    echo ""

    local port=$BASE_PORT
    while IFS= read -r ip; do
      [[ -z "$ip" ]] && continue
      echo "http_port ${port}"
      port=$((port + 1))
    done <<< "$ips"

    echo ""
    echo "# -----------------------------------------------------------------------------"
    echo "# Outgoing Address Mapping (localport -> tcp_outgoing_address)"
    echo "# -----------------------------------------------------------------------------"
    echo ""

    port=$BASE_PORT
    while IFS= read -r ip; do
      [[ -z "$ip" ]] && continue
      local acl_name="port_${port}"
      echo "acl ${acl_name} localport ${port}"
      echo "tcp_outgoing_address ${ip} ${acl_name}"
      echo "# Port ${port} -> ${ip}"
      echo ""
      port=$((port + 1))
    done <<< "$ips"

    # Fallback: first IP for any unmapped ports.
    local first_ip
    first_ip=$(echo "$ips" | head -1)
    echo "# Fallback for unmapped ports"
    echo "tcp_outgoing_address ${first_ip}"
    echo ""
  fi

  # Per-port access log tags.
  echo "# -----------------------------------------------------------------------------"
  echo "# Per-Port Access Log Tags"
  echo "# -----------------------------------------------------------------------------"
  echo ""

  if [[ -n "$ips" ]]; then
    local port=$BASE_PORT
    while IFS= read -r ip; do
      [[ -z "$ip" ]] && continue
      echo "access_log daemon:/var/log/squid/access-port-${port}.log squid port_${port}"
      port=$((port + 1))
    done <<< "$ips"
  fi

  echo ""
  echo "# vim: set ft=squid :"
}

# Regenerate Squid config and reload the Squid service.
regenerate_and_reload_squid() {
  cmd_regenerate_squid

  log_info "Reloading Squid container..."
  if docker exec "$SQUID_CONTAINER" squid -k reconfigure 2>/dev/null; then
    log_success "Squid reloaded"
  elif (cd "$COMPOSE_DIR" && docker compose -f docker-compose.base.yml -f docker-compose.prod.yml restart squid 2>/dev/null); then
    log_success "Squid restarted via docker compose"
  else
    log_warn "Squid container not running. Start it with: docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d squid"
  fi
}

# =============================================================================
# Main Dispatch
# =============================================================================

usage() {
  echo "Usage: $0 <command> [options]"
  echo ""
  echo "Commands:"
  echo "  add --region <region>    Create and assign a new Reserved IP"
  echo "  remove --ip <ip>         Remove and release a Reserved IP"
  echo "  list                     Show current IPs and their status"
  echo "  validate                 Check for drift between DO API, inventory, interfaces, and Squid"
  echo "  regenerate-squid         Regenerate Squid config from inventory"
  echo ""
  echo "Environment variables:"
  echo "  INVENTORY_FILE     Path to IP inventory (default: $INVENTORY_FILE)"
  echo "  SQUID_CONF         Path to Squid config (default: $SQUID_CONF)"
  echo "  SQUID_LOG_DIR      Path to Squid logs (default: $SQUID_LOG_DIR)"
  echo "  NETWORK_INTERFACE  Network interface (default: $NETWORK_INTERFACE)"
  echo ""
  echo "Examples:"
  echo "  $0 add --region nyc1"
  echo "  $0 remove --ip 203.0.113.50"
  echo "  $0 list"
  echo "  $0 validate"
  echo "  $0 regenerate-squid"
}

main() {
  if [[ $# -lt 1 ]]; then
    usage
    exit 1
  fi

  local command="$1"
  shift

  case "$command" in
    add)
      cmd_add "$@"
      ;;
    remove)
      cmd_remove "$@"
      ;;
    list)
      cmd_list "$@"
      ;;
    validate)
      cmd_validate "$@"
      ;;
    regenerate-squid)
      cmd_regenerate_squid "$@"
      ;;
    -h|--help|help)
      usage
      ;;
    *)
      die "Unknown command: $command (use --help for usage)"
      ;;
  esac
}

main "$@"
