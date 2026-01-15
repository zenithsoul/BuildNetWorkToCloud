#!/bin/bash
#
# netns-restore-yaml.sh - Restore network namespaces from YAML config
#
# Requires: yq (https://github.com/mikefarah/yq)
#
# Usage: netns-restore-yaml.sh [--config /path/to/config.yaml]
#

set -e

CONFIG_FILE="/etc/netns-mgr/config.yaml"
LOG_FILE="/var/log/netns-restore.log"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --config|-c)
            CONFIG_FILE="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [--config /path/to/config.yaml]"
            exit 0
            ;;
        *)
            shift
            ;;
    esac
done

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $@" | tee -a "$LOG_FILE"
}

# Check requirements
check_requirements() {
    if [[ $EUID -ne 0 ]]; then
        echo "Must run as root"
        exit 1
    fi

    if ! command -v yq &>/dev/null; then
        echo "yq is required. Install: https://github.com/mikefarah/yq"
        exit 1
    fi

    if [[ ! -f "$CONFIG_FILE" ]]; then
        echo "Config not found: $CONFIG_FILE"
        exit 1
    fi
}

# Create namespaces
create_namespaces() {
    log "Creating namespaces..."
    local count=$(yq '.namespaces | length' "$CONFIG_FILE")

    for ((i=0; i<count; i++)); do
        local name=$(yq ".namespaces[$i].name" "$CONFIG_FILE")
        [[ "$name" == "null" ]] && continue

        if ! ip netns list | grep -q "^${name}$"; then
            ip netns add "$name"
            ip netns exec "$name" ip link set lo up
            log "  Created: $name"
        else
            log "  Exists: $name"
        fi
    done
}

# Create bridges
create_bridges() {
    log "Creating bridges..."
    local count=$(yq '.bridges | length' "$CONFIG_FILE")

    for ((i=0; i<count; i++)); do
        local name=$(yq ".bridges[$i].name" "$CONFIG_FILE")
        local ns=$(yq ".bridges[$i].namespace" "$CONFIG_FILE")
        [[ "$name" == "null" ]] && continue

        if [[ "$ns" == "null" || "$ns" == "" ]]; then
            if ! ip link show "$name" &>/dev/null; then
                ip link add "$name" type bridge
                ip link set "$name" up
                log "  Created bridge: $name"
            fi
        else
            if ! ip netns exec "$ns" ip link show "$name" &>/dev/null; then
                ip netns exec "$ns" ip link add "$name" type bridge
                ip netns exec "$ns" ip link set "$name" up
                log "  Created bridge: $name in $ns"
            fi
        fi
    done
}

# Create veth pairs
create_veths() {
    log "Creating veth pairs..."
    local count=$(yq '.veths | length' "$CONFIG_FILE")

    for ((i=0; i<count; i++)); do
        local name=$(yq ".veths[$i].name" "$CONFIG_FILE")
        local peer=$(yq ".veths[$i].peer" "$CONFIG_FILE")
        local ns=$(yq ".veths[$i].namespace" "$CONFIG_FILE")
        local peer_ns=$(yq ".veths[$i].peer_namespace" "$CONFIG_FILE")

        [[ "$name" == "null" ]] && continue

        # Check if exists
        if ip link show "$name" &>/dev/null 2>&1; then
            log "  Exists: $name"
            continue
        fi

        # Create veth pair
        ip link add "$name" type veth peer name "$peer"
        log "  Created: $name <-> $peer"

        # Move to namespace
        if [[ "$ns" != "null" && "$ns" != "" ]]; then
            ip link set "$name" netns "$ns"
            log "    Moved $name to $ns"
        fi

        if [[ "$peer_ns" != "null" && "$peer_ns" != "" ]]; then
            ip link set "$peer" netns "$peer_ns"
            log "    Moved $peer to $peer_ns"
        fi
    done
}

# Add IP addresses
add_addresses() {
    log "Adding IP addresses..."
    local count=$(yq '.addresses | length' "$CONFIG_FILE")

    for ((i=0; i<count; i++)); do
        local iface=$(yq ".addresses[$i].interface" "$CONFIG_FILE")
        local addr=$(yq ".addresses[$i].address" "$CONFIG_FILE")
        local ns=$(yq ".addresses[$i].namespace" "$CONFIG_FILE")

        [[ "$iface" == "null" || "$addr" == "null" ]] && continue

        if [[ "$ns" == "null" || "$ns" == "" ]]; then
            ip addr add "$addr" dev "$iface" 2>/dev/null || true
            ip link set "$iface" up 2>/dev/null || true
        else
            ip netns exec "$ns" ip addr add "$addr" dev "$iface" 2>/dev/null || true
            ip netns exec "$ns" ip link set "$iface" up 2>/dev/null || true
        fi
        log "  Added: $addr on $iface"
    done
}

# Add routes
add_routes() {
    log "Adding routes..."
    local count=$(yq '.routes | length' "$CONFIG_FILE")

    for ((i=0; i<count; i++)); do
        local dest=$(yq ".routes[$i].destination" "$CONFIG_FILE")
        local gw=$(yq ".routes[$i].gateway" "$CONFIG_FILE")
        local iface=$(yq ".routes[$i].interface" "$CONFIG_FILE")
        local ns=$(yq ".routes[$i].namespace" "$CONFIG_FILE")

        [[ "$dest" == "null" ]] && continue

        local cmd="ip route add $dest"
        [[ "$gw" != "null" && "$gw" != "" ]] && cmd="$cmd via $gw"
        [[ "$iface" != "null" && "$iface" != "" ]] && cmd="$cmd dev $iface"

        if [[ "$ns" == "null" || "$ns" == "" ]]; then
            $cmd 2>/dev/null || true
        else
            ip netns exec "$ns" $cmd 2>/dev/null || true
        fi
        log "  Added route: $dest"
    done
}

# Add bridge ports
add_bridge_ports() {
    log "Adding bridge ports..."
    local bridge_count=$(yq '.bridges | length' "$CONFIG_FILE")

    for ((i=0; i<bridge_count; i++)); do
        local bridge=$(yq ".bridges[$i].name" "$CONFIG_FILE")
        local ns=$(yq ".bridges[$i].namespace" "$CONFIG_FILE")
        local port_count=$(yq ".bridges[$i].ports | length" "$CONFIG_FILE")

        for ((j=0; j<port_count; j++)); do
            local port=$(yq ".bridges[$i].ports[$j]" "$CONFIG_FILE")
            [[ "$port" == "null" ]] && continue

            if [[ "$ns" == "null" || "$ns" == "" ]]; then
                ip link set "$port" master "$bridge" 2>/dev/null || true
            else
                ip netns exec "$ns" ip link set "$port" master "$bridge" 2>/dev/null || true
            fi
            log "  Added port $port to $bridge"
        done
    done
}

# Create GRE tunnels
create_gre_tunnels() {
    log "Creating GRE tunnels..."
    local count=$(yq '.gre_tunnels | length' "$CONFIG_FILE")

    for ((i=0; i<count; i++)); do
        local tunnel_name=$(yq ".gre_tunnels[$i].name" "$CONFIG_FILE")
        local local_ip=$(yq ".gre_tunnels[$i].local_ip" "$CONFIG_FILE")
        local remote_ip=$(yq ".gre_tunnels[$i].remote_ip" "$CONFIG_FILE")
        local gre_key=$(yq ".gre_tunnels[$i].key" "$CONFIG_FILE")
        local ttl=$(yq ".gre_tunnels[$i].ttl" "$CONFIG_FILE")
        local ns=$(yq ".gre_tunnels[$i].namespace" "$CONFIG_FILE")

        [[ "$tunnel_name" == "null" ]] && continue
        [[ "$local_ip" == "null" || "$remote_ip" == "null" ]] && continue

        # Build GRE tunnel command
        local cmd="ip link add $tunnel_name type gre local $local_ip remote $remote_ip"
        [[ "$gre_key" != "null" && "$gre_key" != "" && "$gre_key" != "0" ]] && cmd="$cmd key $gre_key"
        [[ "$ttl" != "null" && "$ttl" != "" && "$ttl" != "0" ]] && cmd="$cmd ttl $ttl"

        if [[ "$ns" == "null" || "$ns" == "" ]]; then
            # Host namespace
            if ! ip link show "$tunnel_name" &>/dev/null; then
                $cmd
                ip link set "$tunnel_name" up
                log "  Created GRE tunnel: $tunnel_name (local=$local_ip, remote=$remote_ip)"
            else
                log "  Exists: $tunnel_name"
            fi
        else
            # In namespace
            if ! ip netns exec "$ns" ip link show "$tunnel_name" &>/dev/null; then
                ip netns exec "$ns" $cmd
                ip netns exec "$ns" ip link set "$tunnel_name" up
                log "  Created GRE tunnel: $tunnel_name in $ns (local=$local_ip, remote=$remote_ip)"
            else
                log "  Exists: $tunnel_name in $ns"
            fi
        fi
    done
}

# Main
main() {
    check_requirements

    log "=========================================="
    log "Restoring from: $CONFIG_FILE"
    log "=========================================="

    create_namespaces
    create_bridges
    create_veths
    add_addresses
    add_routes
    add_bridge_ports
    create_gre_tunnels

    log "=========================================="
    log "Restoration complete!"
    log "=========================================="
}

main "$@"
