#!/bin/bash
#
# netns-restore-simple.sh - Simple version, easy to understand
#

set -e

DB_PATH="${HOME}/.netns-mgr/netns.db"

# Check root
if [[ $EUID -ne 0 ]]; then
    echo "Please run as root"
    exit 1
fi

# Check database
if [[ ! -f "$DB_PATH" ]]; then
    echo "Database not found: $DB_PATH"
    exit 1
fi

echo "=== Restoring Network Namespaces ==="

#--------------------------------------------
# Step 1: Create namespaces
#--------------------------------------------
echo "Step 1: Creating namespaces..."

# Query database, get list of names
names=$(sqlite3 "$DB_PATH" "SELECT name FROM namespaces;")

# Loop through each name
for name in $names; do
    echo "  Creating namespace: $name"
    ip netns add "$name" 2>/dev/null || echo "    (already exists)"
    ip netns exec "$name" ip link set lo up
done

#--------------------------------------------
# Step 2: Create veth pairs
#--------------------------------------------
echo "Step 2: Creating veth pairs..."

# Query: get veth name and peer name
sqlite3 -separator ' ' "$DB_PATH" "SELECT name, peer_name FROM veth_pairs;" | while read name peer; do
    echo "  Creating veth: $name <-> $peer"
    ip link add "$name" type veth peer name "$peer" 2>/dev/null || echo "    (already exists)"
done

#--------------------------------------------
# Step 3: Add IP addresses
#--------------------------------------------
echo "Step 3: Adding IP addresses..."

# Query: get interface, address, and namespace name (using JOIN)
sqlite3 -separator ' ' "$DB_PATH" "
    SELECT ip.interface_name, ip.address, COALESCE(ns.name, '')
    FROM ip_addresses ip
    LEFT JOIN namespaces ns ON ip.ns_id = ns.id;
" | while read iface addr nsname; do

    if [[ -z "$nsname" ]]; then
        # No namespace = host
        echo "  Adding $addr to $iface (host)"
        ip addr add "$addr" dev "$iface" 2>/dev/null || true
        ip link set "$iface" up 2>/dev/null || true
    else
        # In namespace
        echo "  Adding $addr to $iface (in $nsname)"
        ip netns exec "$nsname" ip addr add "$addr" dev "$iface" 2>/dev/null || true
        ip netns exec "$nsname" ip link set "$iface" up 2>/dev/null || true
    fi
done

#--------------------------------------------
# Step 4: Add routes
#--------------------------------------------
echo "Step 4: Adding routes..."

sqlite3 -separator ' ' "$DB_PATH" "
    SELECT r.destination, COALESCE(r.gateway,''), COALESCE(r.interface_name,''), COALESCE(ns.name, '')
    FROM routes r
    LEFT JOIN namespaces ns ON r.ns_id = ns.id;
" | while read dest gw iface nsname; do

    # Build route command
    cmd="ip route add $dest"
    [[ -n "$gw" ]] && cmd="$cmd via $gw"
    [[ -n "$iface" ]] && cmd="$cmd dev $iface"

    if [[ -z "$nsname" ]]; then
        echo "  Adding route: $dest (host)"
        $cmd 2>/dev/null || true
    else
        echo "  Adding route: $dest (in $nsname)"
        ip netns exec "$nsname" $cmd 2>/dev/null || true
    fi
done

#--------------------------------------------
# Step 5: Create bridges
#--------------------------------------------
echo "Step 5: Creating bridges..."

sqlite3 "$DB_PATH" "SELECT name FROM bridges;" | while read name; do
    echo "  Creating bridge: $name"
    ip link add "$name" type bridge 2>/dev/null || echo "    (already exists)"
    ip link set "$name" up
done

#--------------------------------------------
# Step 6: Create GRE tunnels
#--------------------------------------------
echo "Step 6: Creating GRE tunnels..."

# Query: get tunnel name, local_ip, remote_ip, key, ttl, namespace name
sqlite3 -separator ' ' "$DB_PATH" "
    SELECT g.name, g.local_ip, g.remote_ip, g.gre_key, g.ttl, COALESCE(ns.name, '')
    FROM gre_tunnels g
    LEFT JOIN namespaces ns ON g.ns_id = ns.id;
" | while read tunnel_name local_ip remote_ip gre_key ttl nsname; do

    # Build GRE tunnel command
    cmd="ip link add $tunnel_name type gre local $local_ip remote $remote_ip"
    [[ -n "$gre_key" && "$gre_key" != "0" ]] && cmd="$cmd key $gre_key"
    [[ -n "$ttl" && "$ttl" != "0" ]] && cmd="$cmd ttl $ttl"

    if [[ -z "$nsname" ]]; then
        echo "  Creating GRE tunnel: $tunnel_name (host)"
        $cmd 2>/dev/null || echo "    (already exists)"
        ip link set "$tunnel_name" up 2>/dev/null || true
    else
        echo "  Creating GRE tunnel: $tunnel_name (in $nsname)"
        ip netns exec "$nsname" $cmd 2>/dev/null || echo "    (already exists)"
        ip netns exec "$nsname" ip link set "$tunnel_name" up 2>/dev/null || true
    fi
done

echo "=== Done! ==="
