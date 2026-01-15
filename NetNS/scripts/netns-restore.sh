#!/bin/bash
#
# netns-restore.sh - Restore network namespaces from SQLite database on boot
#
# Usage: netns-restore.sh [--db /path/to/netns.db]
#

set -e

# Default database path
DB_PATH="${HOME}/.netns-mgr/netns.db"
LOG_FILE="/var/log/netns-restore.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --db)
            DB_PATH="$2"
            shift 2
            ;;
        --log)
            LOG_FILE="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [--db /path/to/netns.db] [--log /path/to/log]"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Logging function
log() {
    local level=$1
    shift
    local msg="$@"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "${timestamp} [${level}] ${msg}" | tee -a "$LOG_FILE"
}

log_info() { log "INFO" "$@"; }
log_warn() { log "WARN" "${YELLOW}$@${NC}"; }
log_error() { log "ERROR" "${RED}$@${NC}"; }
log_success() { log "OK" "${GREEN}$@${NC}"; }

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root"
        exit 1
    fi
}

# Check if database exists
check_db() {
    if [[ ! -f "$DB_PATH" ]]; then
        log_error "Database not found: $DB_PATH"
        exit 1
    fi
    log_info "Using database: $DB_PATH"
}

# Query SQLite database
query_db() {
    sqlite3 -separator '|' "$DB_PATH" "$1"
}

# Create network namespace
# Parameters:
#   $1 = namespace_name : Name of the namespace to create
create_namespace() {
    local namespace_name=$1

    # Check if namespace already exists
    if ip netns list | grep -q "^${namespace_name}$"; then
        log_warn "Namespace '$namespace_name' already exists, skipping"
        return 0
    fi

    # Create the namespace
    ip netns add "$namespace_name"

    # Bring up loopback interface inside the namespace
    ip netns exec "$namespace_name" ip link set lo up

    log_success "Created namespace: $namespace_name"
}

# Create veth pair (virtual ethernet pair)
# Parameters:
#   $1 = veth_name           : Name of the first veth interface
#   $2 = peer_name           : Name of the peer veth interface
#   $3 = namespace_name      : Namespace to move veth_name into (optional)
#   $4 = peer_namespace_name : Namespace to move peer_name into (optional)
create_veth() {
    local veth_name=$1
    local peer_name=$2
    local namespace_name=$3
    local peer_namespace_name=$4

    # Check if veth already exists
    if ip link show "$veth_name" &>/dev/null; then
        log_warn "Veth '$veth_name' already exists, skipping"
        return 0
    fi

    # Create veth pair in host namespace
    ip link add "$veth_name" type veth peer name "$peer_name"

    # Move first interface to namespace if specified
    if [[ -n "$namespace_name" && "$namespace_name" != "NULL" ]]; then
        ip link set "$veth_name" netns "$namespace_name"
    fi

    # Move peer interface to namespace if specified
    if [[ -n "$peer_namespace_name" && "$peer_namespace_name" != "NULL" ]]; then
        ip link set "$peer_name" netns "$peer_namespace_name"
    fi

    log_success "Created veth pair: $veth_name <-> $peer_name"
}

# Add IP address to an interface
# Parameters:
#   $1 = interface_name : Name of the network interface
#   $2 = ip_address     : IP address in CIDR format (e.g., 10.0.0.1/24)
#   $3 = namespace_name : Namespace where interface exists (optional, empty = host)
add_ip_address() {
    local interface_name=$1
    local ip_address=$2
    local namespace_name=$3

    # Build the command
    local cmd="ip addr add $ip_address dev $interface_name"

    if [[ -n "$namespace_name" && "$namespace_name" != "NULL" ]]; then
        # Interface is in a namespace
        # Check if address already exists
        if ip netns exec "$namespace_name" ip addr show dev "$interface_name" 2>/dev/null | grep -q "$ip_address"; then
            log_warn "Address '$ip_address' already exists on $interface_name in $namespace_name, skipping"
            return 0
        fi
        # Add address and bring interface up
        ip netns exec "$namespace_name" $cmd 2>/dev/null || true
        ip netns exec "$namespace_name" ip link set "$interface_name" up 2>/dev/null || true
    else
        # Interface is in host namespace
        if ip addr show dev "$interface_name" 2>/dev/null | grep -q "$ip_address"; then
            log_warn "Address '$ip_address' already exists on $interface_name, skipping"
            return 0
        fi
        $cmd 2>/dev/null || true
        ip link set "$interface_name" up 2>/dev/null || true
    fi

    log_success "Added IP: $ip_address to $interface_name"
}

# Add route to routing table
# Parameters:
#   $1 = destination     : Destination network (CIDR) or "default"
#   $2 = gateway         : Gateway IP address (optional)
#   $3 = interface_name  : Output interface name (optional)
#   $4 = namespace_name  : Namespace to add route in (optional, empty = host)
add_route() {
    local destination=$1
    local gateway=$2
    local interface_name=$3
    local namespace_name=$4

    # Build the route command
    local cmd="ip route add"

    # Add destination (default or specific network)
    if [[ "$destination" == "default" || -z "$destination" ]]; then
        cmd="$cmd default"
    else
        cmd="$cmd $destination"
    fi

    # Add gateway if specified
    if [[ -n "$gateway" && "$gateway" != "NULL" && "$gateway" != "" ]]; then
        cmd="$cmd via $gateway"
    fi

    # Add output interface if specified
    if [[ -n "$interface_name" && "$interface_name" != "NULL" && "$interface_name" != "" ]]; then
        cmd="$cmd dev $interface_name"
    fi

    # Execute in namespace or host
    if [[ -n "$namespace_name" && "$namespace_name" != "NULL" ]]; then
        ip netns exec "$namespace_name" $cmd 2>/dev/null || log_warn "Route may already exist: $destination"
    else
        $cmd 2>/dev/null || log_warn "Route may already exist: $destination"
    fi

    log_success "Added route: $destination"
}

# Create network bridge
# Parameters:
#   $1 = bridge_name    : Name of the bridge to create
#   $2 = namespace_name : Namespace to create bridge in (optional, empty = host)
create_bridge() {
    local bridge_name=$1
    local namespace_name=$2

    if [[ -n "$namespace_name" && "$namespace_name" != "NULL" ]]; then
        # Create bridge in namespace
        if ip netns exec "$namespace_name" ip link show "$bridge_name" &>/dev/null; then
            log_warn "Bridge '$bridge_name' already exists in $namespace_name, skipping"
            return 0
        fi
        ip netns exec "$namespace_name" ip link add "$bridge_name" type bridge
        ip netns exec "$namespace_name" ip link set "$bridge_name" up
    else
        # Create bridge in host namespace
        if ip link show "$bridge_name" &>/dev/null; then
            log_warn "Bridge '$bridge_name' already exists, skipping"
            return 0
        fi
        ip link add "$bridge_name" type bridge
        ip link set "$bridge_name" up
    fi

    log_success "Created bridge: $bridge_name"
}

# Add interface to bridge as a port
# Parameters:
#   $1 = bridge_name    : Name of the bridge
#   $2 = interface_name : Name of the interface to add as port
#   $3 = namespace_name : Namespace where bridge exists (optional, empty = host)
add_bridge_port() {
    local bridge_name=$1
    local interface_name=$2
    local namespace_name=$3

    if [[ -n "$namespace_name" && "$namespace_name" != "NULL" ]]; then
        # Add port in namespace
        ip netns exec "$namespace_name" ip link set "$interface_name" master "$bridge_name" 2>/dev/null || true
    else
        # Add port in host namespace
        ip link set "$interface_name" master "$bridge_name" 2>/dev/null || true
    fi

    log_success "Added port $interface_name to bridge $bridge_name"
}

# Create GRE tunnel
# Parameters:
#   $1 = tunnel_name    : Name of the GRE tunnel interface (e.g., gre1)
#   $2 = local_ip       : Local endpoint IP address
#   $3 = remote_ip      : Remote endpoint IP address
#   $4 = gre_key        : GRE key for multiplexing (0 = no key)
#   $5 = ttl            : Time to live (0 = inherit)
#   $6 = namespace_name : Namespace to create tunnel in (optional, empty = host)
create_gre_tunnel() {
    local tunnel_name=$1
    local local_ip=$2
    local remote_ip=$3
    local gre_key=$4
    local ttl=$5
    local namespace_name=$6

    # Build the command
    local cmd="ip link add $tunnel_name type gre local $local_ip remote $remote_ip"

    # Add key if specified and non-zero
    if [[ -n "$gre_key" && "$gre_key" != "0" && "$gre_key" != "NULL" ]]; then
        cmd="$cmd key $gre_key"
    fi

    # Add TTL if specified and non-zero
    if [[ -n "$ttl" && "$ttl" != "0" && "$ttl" != "NULL" ]]; then
        cmd="$cmd ttl $ttl"
    fi

    if [[ -n "$namespace_name" && "$namespace_name" != "NULL" ]]; then
        # Check if tunnel already exists in namespace
        if ip netns exec "$namespace_name" ip link show "$tunnel_name" &>/dev/null; then
            log_warn "GRE tunnel '$tunnel_name' already exists in $namespace_name, skipping"
            return 0
        fi
        # Create tunnel in namespace
        ip netns exec "$namespace_name" $cmd 2>/dev/null || true
        ip netns exec "$namespace_name" ip link set "$tunnel_name" up 2>/dev/null || true
    else
        # Check if tunnel already exists in host
        if ip link show "$tunnel_name" &>/dev/null; then
            log_warn "GRE tunnel '$tunnel_name' already exists, skipping"
            return 0
        fi
        # Create tunnel in host namespace
        $cmd 2>/dev/null || true
        ip link set "$tunnel_name" up 2>/dev/null || true
    fi

    log_success "Created GRE tunnel: $tunnel_name (local=$local_ip, remote=$remote_ip)"
}

# Main restore function
restore_all() {
    log_info "=========================================="
    log_info "Starting network namespace restoration"
    log_info "=========================================="

    # 1. Restore namespaces
    log_info "Restoring namespaces..."

    # Get all namespace names from database
    namespaces=$(query_db "SELECT name FROM namespaces ORDER BY id;")

    # Loop through each namespace
    for name in $namespaces; do
        if [[ -n "$name" ]]; then
            create_namespace "$name"
        fi
    done

    # 2. Restore bridges (before veths, as veths might connect to bridges)
    log_info "Restoring bridges..."

    # Query: SELECT id, name, ns_id FROM bridges
    # Example output: "1|br0|2" means id=1, bridge_name=br0, namespace_id=2
    while IFS='|' read -r bridge_id bridge_name namespace_id; do
        # Skip if bridge_name is empty
        [[ -z "$bridge_name" ]] && continue

        # Convert namespace_id to namespace_name
        namespace_name=""
        if [[ -n "$namespace_id" && "$namespace_id" != "NULL" ]]; then
            namespace_name=$(query_db "SELECT name FROM namespaces WHERE id=$namespace_id;")
        fi

        create_bridge "$bridge_name" "$namespace_name"
    done < <(query_db "SELECT id, name, ns_id FROM bridges ORDER BY id;")

    # 3. Restore veth pairs
    log_info "Restoring veth pairs..."

    # Query output: "1|veth0|veth1|2|3"
    # means: id=1, veth_name=veth0, peer_name=veth1, namespace_id=2, peer_namespace_id=3
    while IFS='|' read -r veth_id veth_name peer_name namespace_id peer_namespace_id; do
        [[ -z "$veth_name" ]] && continue

        # Convert IDs to names
        namespace_name=""
        peer_namespace_name=""

        if [[ -n "$namespace_id" && "$namespace_id" != "NULL" ]]; then
            namespace_name=$(query_db "SELECT name FROM namespaces WHERE id=$namespace_id;")
        fi

        if [[ -n "$peer_namespace_id" && "$peer_namespace_id" != "NULL" ]]; then
            peer_namespace_name=$(query_db "SELECT name FROM namespaces WHERE id=$peer_namespace_id;")
        fi

        create_veth "$veth_name" "$peer_name" "$namespace_name" "$peer_namespace_name"
    done < <(query_db "SELECT id, name, peer_name, ns_id, peer_ns_id FROM veth_pairs ORDER BY id;")

    # 4. Restore IP addresses
    log_info "Restoring IP addresses..."

    # Query output: "1|eth0|2|10.0.0.1/24"
    # means: id=1, interface=eth0, namespace_id=2, ip_address=10.0.0.1/24
    while IFS='|' read -r addr_id interface_name namespace_id ip_address; do
        [[ -z "$ip_address" ]] && continue

        namespace_name=""
        if [[ -n "$namespace_id" && "$namespace_id" != "NULL" ]]; then
            namespace_name=$(query_db "SELECT name FROM namespaces WHERE id=$namespace_id;")
        fi

        add_ip_address "$interface_name" "$ip_address" "$namespace_name"
    done < <(query_db "SELECT id, interface_name, ns_id, address FROM ip_addresses ORDER BY id;")

    # 5. Restore routes
    log_info "Restoring routes..."

    # Query output: "1|2|10.0.0.0/24|10.0.0.1|eth0"
    # means: id=1, namespace_id=2, destination=10.0.0.0/24, gateway=10.0.0.1, interface=eth0
    while IFS='|' read -r route_id namespace_id destination gateway interface_name; do
        [[ -z "$destination" ]] && continue

        namespace_name=""
        if [[ -n "$namespace_id" && "$namespace_id" != "NULL" ]]; then
            namespace_name=$(query_db "SELECT name FROM namespaces WHERE id=$namespace_id;")
        fi

        add_route "$destination" "$gateway" "$interface_name" "$namespace_name"
    done < <(query_db "SELECT id, ns_id, destination, gateway, interface_name FROM routes ORDER BY id;")

    # 6. Restore bridge ports
    log_info "Restoring bridge ports..."

    # Query output: "1|2|eth0"
    # means: id=1, bridge_id=2, interface=eth0
    while IFS='|' read -r port_id bridge_id interface_name; do
        [[ -z "$interface_name" ]] && continue

        # Get bridge info
        bridge_name=$(query_db "SELECT name FROM bridges WHERE id=$bridge_id;")
        namespace_id=$(query_db "SELECT ns_id FROM bridges WHERE id=$bridge_id;")

        namespace_name=""
        if [[ -n "$namespace_id" && "$namespace_id" != "NULL" ]]; then
            namespace_name=$(query_db "SELECT name FROM namespaces WHERE id=$namespace_id;")
        fi

        add_bridge_port "$bridge_name" "$interface_name" "$namespace_name"
    done < <(query_db "SELECT id, bridge_id, interface_name FROM bridge_ports ORDER BY id;")

    # 7. Restore GRE tunnels
    log_info "Restoring GRE tunnels..."

    # Query output: "1|gre1|10.0.0.1|10.0.0.2|100|64|2"
    # means: id=1, name=gre1, local_ip=10.0.0.1, remote_ip=10.0.0.2, key=100, ttl=64, ns_id=2
    while IFS='|' read -r tunnel_id tunnel_name local_ip remote_ip gre_key ttl namespace_id; do
        [[ -z "$tunnel_name" ]] && continue

        namespace_name=""
        if [[ -n "$namespace_id" && "$namespace_id" != "NULL" ]]; then
            namespace_name=$(query_db "SELECT name FROM namespaces WHERE id=$namespace_id;")
        fi

        create_gre_tunnel "$tunnel_name" "$local_ip" "$remote_ip" "$gre_key" "$ttl" "$namespace_name"
    done < <(query_db "SELECT id, name, local_ip, remote_ip, gre_key, ttl, ns_id FROM gre_tunnels ORDER BY id;")

    log_info "=========================================="
    log_info "Restoration complete!"
    log_info "=========================================="
}

# Cleanup function - Remove all managed namespaces
# This deletes all namespaces that are recorded in the database
cleanup_all() {
    log_info "Cleaning up all managed namespaces..."

    # Query: SELECT id, name FROM namespaces
    # Output: "1|ns1" means namespace_id=1, namespace_name=ns1
    while IFS='|' read -r namespace_id namespace_name; do
        # Skip empty names
        [[ -z "$namespace_name" ]] && continue

        # Delete the namespace
        ip netns del "$namespace_name" 2>/dev/null && log_info "Deleted namespace: $namespace_name" || true
    done < <(query_db "SELECT id, name FROM namespaces;")
}

# Main entry point
# Usage:
#   ./netns-restore.sh restore  - Restore all namespaces from database
#   ./netns-restore.sh cleanup  - Delete all managed namespaces
main() {
    # Step 1: Check if running as root
    check_root

    # Step 2: Check if database exists
    check_db

    # Step 3: Run the requested action
    # ${1:-restore} means: use $1 if set, otherwise default to "restore"
    case "${1:-restore}" in
        restore)
            restore_all
            ;;
        cleanup)
            cleanup_all
            ;;
        *)
            echo "Usage: $0 {restore|cleanup}"
            exit 1
            ;;
    esac
}

# Call main function with all script arguments
# "$@" passes all arguments to main()
main "$@"
