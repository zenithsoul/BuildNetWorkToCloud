package db

import (
	"database/sql"
	"fmt"
)

// Repository handles database operations
type Repository struct {
	db *DB
}

// NewRepository creates a new repository
func NewRepository(db *DB) *Repository {
	return &Repository{db: db}
}

// === Namespace Operations ===

// CreateNamespace creates a new namespace record
func (r *Repository) CreateNamespace(name, metadata string) (*Namespace, error) {
	result, err := r.db.Exec(
		"INSERT INTO namespaces (name, metadata) VALUES (?, ?)",
		name, metadata,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create namespace: %w", err)
	}

	id, _ := result.LastInsertId()
	return r.GetNamespace(id)
}

// GetNamespace retrieves a namespace by ID
func (r *Repository) GetNamespace(id int64) (*Namespace, error) {
	ns := &Namespace{}
	err := r.db.QueryRow(
		"SELECT id, name, created_at, COALESCE(metadata, '') FROM namespaces WHERE id = ?",
		id,
	).Scan(&ns.ID, &ns.Name, &ns.CreatedAt, &ns.Metadata)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ns, nil
}

// GetNamespaceByName retrieves a namespace by name
func (r *Repository) GetNamespaceByName(name string) (*Namespace, error) {
	ns := &Namespace{}
	err := r.db.QueryRow(
		"SELECT id, name, created_at, COALESCE(metadata, '') FROM namespaces WHERE name = ?",
		name,
	).Scan(&ns.ID, &ns.Name, &ns.CreatedAt, &ns.Metadata)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ns, nil
}

// ListNamespaces returns all namespaces
func (r *Repository) ListNamespaces() ([]Namespace, error) {
	rows, err := r.db.Query("SELECT id, name, created_at, COALESCE(metadata, '') FROM namespaces ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var namespaces []Namespace
	for rows.Next() {
		var ns Namespace
		if err := rows.Scan(&ns.ID, &ns.Name, &ns.CreatedAt, &ns.Metadata); err != nil {
			return nil, err
		}
		namespaces = append(namespaces, ns)
	}
	return namespaces, rows.Err()
}

// DeleteNamespace deletes a namespace by name
func (r *Repository) DeleteNamespace(name string) error {
	result, err := r.db.Exec("DELETE FROM namespaces WHERE name = ?", name)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("namespace %q not found", name)
	}
	return nil
}

// === VethPair Operations ===

// CreateVethPair creates a new veth pair record
func (r *Repository) CreateVethPair(name, peerName string, nsID, peerNsID *int64) (*VethPair, error) {
	result, err := r.db.Exec(
		"INSERT INTO veth_pairs (name, peer_name, ns_id, peer_ns_id) VALUES (?, ?, ?, ?)",
		name, peerName, nsID, peerNsID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create veth pair: %w", err)
	}

	id, _ := result.LastInsertId()
	return r.GetVethPair(id)
}

// GetVethPair retrieves a veth pair by ID
func (r *Repository) GetVethPair(id int64) (*VethPair, error) {
	veth := &VethPair{}
	err := r.db.QueryRow(
		"SELECT id, name, peer_name, ns_id, peer_ns_id, created_at FROM veth_pairs WHERE id = ?",
		id,
	).Scan(&veth.ID, &veth.Name, &veth.PeerName, &veth.NsID, &veth.PeerNsID, &veth.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return veth, nil
}

// GetVethPairByName retrieves a veth pair by name
func (r *Repository) GetVethPairByName(name string) (*VethPair, error) {
	veth := &VethPair{}
	err := r.db.QueryRow(
		"SELECT id, name, peer_name, ns_id, peer_ns_id, created_at FROM veth_pairs WHERE name = ?",
		name,
	).Scan(&veth.ID, &veth.Name, &veth.PeerName, &veth.NsID, &veth.PeerNsID, &veth.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return veth, nil
}

// ListVethPairs returns all veth pairs
func (r *Repository) ListVethPairs() ([]VethPair, error) {
	rows, err := r.db.Query("SELECT id, name, peer_name, ns_id, peer_ns_id, created_at FROM veth_pairs ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []VethPair
	for rows.Next() {
		var v VethPair
		if err := rows.Scan(&v.ID, &v.Name, &v.PeerName, &v.NsID, &v.PeerNsID, &v.CreatedAt); err != nil {
			return nil, err
		}
		pairs = append(pairs, v)
	}
	return pairs, rows.Err()
}

// DeleteVethPair deletes a veth pair by name
func (r *Repository) DeleteVethPair(name string) error {
	result, err := r.db.Exec("DELETE FROM veth_pairs WHERE name = ?", name)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("veth pair %q not found", name)
	}
	return nil
}

// === IPAddress Operations ===

// CreateIPAddress creates a new IP address record
func (r *Repository) CreateIPAddress(interfaceName string, nsID *int64, address string) (*IPAddress, error) {
	result, err := r.db.Exec(
		"INSERT INTO ip_addresses (interface_name, ns_id, address) VALUES (?, ?, ?)",
		interfaceName, nsID, address,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create IP address: %w", err)
	}

	id, _ := result.LastInsertId()
	return r.GetIPAddress(id)
}

// GetIPAddress retrieves an IP address by ID
func (r *Repository) GetIPAddress(id int64) (*IPAddress, error) {
	ip := &IPAddress{}
	err := r.db.QueryRow(
		"SELECT id, interface_name, ns_id, address, created_at FROM ip_addresses WHERE id = ?",
		id,
	).Scan(&ip.ID, &ip.InterfaceName, &ip.NsID, &ip.Address, &ip.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ip, nil
}

// ListIPAddresses returns all IP addresses, optionally filtered by namespace
func (r *Repository) ListIPAddresses(nsID *int64) ([]IPAddress, error) {
	var rows *sql.Rows
	var err error

	if nsID != nil {
		rows, err = r.db.Query(
			"SELECT id, interface_name, ns_id, address, created_at FROM ip_addresses WHERE ns_id = ? ORDER BY interface_name",
			*nsID,
		)
	} else {
		rows, err = r.db.Query("SELECT id, interface_name, ns_id, address, created_at FROM ip_addresses ORDER BY interface_name")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addresses []IPAddress
	for rows.Next() {
		var ip IPAddress
		if err := rows.Scan(&ip.ID, &ip.InterfaceName, &ip.NsID, &ip.Address, &ip.CreatedAt); err != nil {
			return nil, err
		}
		addresses = append(addresses, ip)
	}
	return addresses, rows.Err()
}

// DeleteIPAddress deletes an IP address by ID
func (r *Repository) DeleteIPAddress(id int64) error {
	result, err := r.db.Exec("DELETE FROM ip_addresses WHERE id = ?", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("IP address with ID %d not found", id)
	}
	return nil
}

// === Route Operations ===

// CreateRoute creates a new route record
func (r *Repository) CreateRoute(nsID *int64, destination, gateway, interfaceName string) (*Route, error) {
	result, err := r.db.Exec(
		"INSERT INTO routes (ns_id, destination, gateway, interface_name) VALUES (?, ?, ?, ?)",
		nsID, destination, gateway, interfaceName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create route: %w", err)
	}

	id, _ := result.LastInsertId()
	return r.GetRoute(id)
}

// GetRoute retrieves a route by ID
func (r *Repository) GetRoute(id int64) (*Route, error) {
	route := &Route{}
	err := r.db.QueryRow(
		"SELECT id, ns_id, destination, COALESCE(gateway, ''), COALESCE(interface_name, ''), created_at FROM routes WHERE id = ?",
		id,
	).Scan(&route.ID, &route.NsID, &route.Destination, &route.Gateway, &route.InterfaceName, &route.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return route, nil
}

// ListRoutes returns all routes, optionally filtered by namespace
func (r *Repository) ListRoutes(nsID *int64) ([]Route, error) {
	var rows *sql.Rows
	var err error

	if nsID != nil {
		rows, err = r.db.Query(
			"SELECT id, ns_id, destination, COALESCE(gateway, ''), COALESCE(interface_name, ''), created_at FROM routes WHERE ns_id = ? ORDER BY destination",
			*nsID,
		)
	} else {
		rows, err = r.db.Query("SELECT id, ns_id, destination, COALESCE(gateway, ''), COALESCE(interface_name, ''), created_at FROM routes ORDER BY destination")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []Route
	for rows.Next() {
		var rt Route
		if err := rows.Scan(&rt.ID, &rt.NsID, &rt.Destination, &rt.Gateway, &rt.InterfaceName, &rt.CreatedAt); err != nil {
			return nil, err
		}
		routes = append(routes, rt)
	}
	return routes, rows.Err()
}

// DeleteRoute deletes a route by ID
func (r *Repository) DeleteRoute(id int64) error {
	result, err := r.db.Exec("DELETE FROM routes WHERE id = ?", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("route with ID %d not found", id)
	}
	return nil
}

// === Bridge Operations ===

// CreateBridge creates a new bridge record
func (r *Repository) CreateBridge(name string, nsID *int64) (*Bridge, error) {
	result, err := r.db.Exec(
		"INSERT INTO bridges (name, ns_id) VALUES (?, ?)",
		name, nsID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create bridge: %w", err)
	}

	id, _ := result.LastInsertId()
	return r.GetBridge(id)
}

// GetBridge retrieves a bridge by ID
func (r *Repository) GetBridge(id int64) (*Bridge, error) {
	br := &Bridge{}
	err := r.db.QueryRow(
		"SELECT id, name, ns_id, created_at FROM bridges WHERE id = ?",
		id,
	).Scan(&br.ID, &br.Name, &br.NsID, &br.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return br, nil
}

// GetBridgeByName retrieves a bridge by name
func (r *Repository) GetBridgeByName(name string) (*Bridge, error) {
	br := &Bridge{}
	err := r.db.QueryRow(
		"SELECT id, name, ns_id, created_at FROM bridges WHERE name = ?",
		name,
	).Scan(&br.ID, &br.Name, &br.NsID, &br.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return br, nil
}

// ListBridges returns all bridges
func (r *Repository) ListBridges() ([]Bridge, error) {
	rows, err := r.db.Query("SELECT id, name, ns_id, created_at FROM bridges ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bridges []Bridge
	for rows.Next() {
		var br Bridge
		if err := rows.Scan(&br.ID, &br.Name, &br.NsID, &br.CreatedAt); err != nil {
			return nil, err
		}
		bridges = append(bridges, br)
	}
	return bridges, rows.Err()
}

// DeleteBridge deletes a bridge by name
func (r *Repository) DeleteBridge(name string) error {
	result, err := r.db.Exec("DELETE FROM bridges WHERE name = ?", name)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("bridge %q not found", name)
	}
	return nil
}

// === Bridge Port Operations ===

// AddBridgePort adds an interface to a bridge
func (r *Repository) AddBridgePort(bridgeID int64, interfaceName string) (*BridgePort, error) {
	result, err := r.db.Exec(
		"INSERT INTO bridge_ports (bridge_id, interface_name) VALUES (?, ?)",
		bridgeID, interfaceName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add bridge port: %w", err)
	}

	id, _ := result.LastInsertId()
	port := &BridgePort{}
	err = r.db.QueryRow(
		"SELECT id, bridge_id, interface_name, created_at FROM bridge_ports WHERE id = ?",
		id,
	).Scan(&port.ID, &port.BridgeID, &port.InterfaceName, &port.CreatedAt)
	if err != nil {
		return nil, err
	}
	return port, nil
}

// ListBridgePorts returns all ports for a bridge
func (r *Repository) ListBridgePorts(bridgeID int64) ([]BridgePort, error) {
	rows, err := r.db.Query(
		"SELECT id, bridge_id, interface_name, created_at FROM bridge_ports WHERE bridge_id = ? ORDER BY interface_name",
		bridgeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ports []BridgePort
	for rows.Next() {
		var p BridgePort
		if err := rows.Scan(&p.ID, &p.BridgeID, &p.InterfaceName, &p.CreatedAt); err != nil {
			return nil, err
		}
		ports = append(ports, p)
	}
	return ports, rows.Err()
}

// RemoveBridgePort removes an interface from a bridge
func (r *Repository) RemoveBridgePort(bridgeID int64, interfaceName string) error {
	result, err := r.db.Exec(
		"DELETE FROM bridge_ports WHERE bridge_id = ? AND interface_name = ?",
		bridgeID, interfaceName,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("port %q not found on bridge", interfaceName)
	}
	return nil
}

// === GRE Tunnel Operations ===

// CreateGRETunnel creates a new GRE tunnel record
// Parameters:
//   - name: tunnel interface name (e.g., "gre1")
//   - localIP: local endpoint IP address
//   - remoteIP: remote endpoint IP address
//   - key: GRE key for multiplexing (0 = no key)
//   - ttl: time to live (0 = inherit from inner packet)
//   - nsID: namespace ID where tunnel is created (nil = host)
func (r *Repository) CreateGRETunnel(name, localIP, remoteIP string, key uint32, ttl uint8, nsID *int64) (*GRETunnel, error) {
	result, err := r.db.Exec(
		"INSERT INTO gre_tunnels (name, local_ip, remote_ip, gre_key, ttl, ns_id) VALUES (?, ?, ?, ?, ?, ?)",
		name, localIP, remoteIP, key, ttl, nsID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create GRE tunnel: %w", err)
	}

	id, _ := result.LastInsertId()
	return r.GetGRETunnel(id)
}

// GetGRETunnel retrieves a GRE tunnel by ID
func (r *Repository) GetGRETunnel(id int64) (*GRETunnel, error) {
	tunnel := &GRETunnel{}
	err := r.db.QueryRow(
		"SELECT id, name, local_ip, remote_ip, gre_key, ttl, ns_id, created_at FROM gre_tunnels WHERE id = ?",
		id,
	).Scan(&tunnel.ID, &tunnel.Name, &tunnel.LocalIP, &tunnel.RemoteIP, &tunnel.Key, &tunnel.TTL, &tunnel.NsID, &tunnel.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return tunnel, nil
}

// GetGRETunnelByName retrieves a GRE tunnel by name
func (r *Repository) GetGRETunnelByName(name string) (*GRETunnel, error) {
	tunnel := &GRETunnel{}
	err := r.db.QueryRow(
		"SELECT id, name, local_ip, remote_ip, gre_key, ttl, ns_id, created_at FROM gre_tunnels WHERE name = ?",
		name,
	).Scan(&tunnel.ID, &tunnel.Name, &tunnel.LocalIP, &tunnel.RemoteIP, &tunnel.Key, &tunnel.TTL, &tunnel.NsID, &tunnel.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return tunnel, nil
}

// ListGRETunnels returns all GRE tunnels, optionally filtered by namespace
func (r *Repository) ListGRETunnels(nsID *int64) ([]GRETunnel, error) {
	var rows *sql.Rows
	var err error

	if nsID != nil {
		rows, err = r.db.Query(
			"SELECT id, name, local_ip, remote_ip, gre_key, ttl, ns_id, created_at FROM gre_tunnels WHERE ns_id = ? ORDER BY name",
			*nsID,
		)
	} else {
		rows, err = r.db.Query("SELECT id, name, local_ip, remote_ip, gre_key, ttl, ns_id, created_at FROM gre_tunnels ORDER BY name")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tunnels []GRETunnel
	for rows.Next() {
		var t GRETunnel
		if err := rows.Scan(&t.ID, &t.Name, &t.LocalIP, &t.RemoteIP, &t.Key, &t.TTL, &t.NsID, &t.CreatedAt); err != nil {
			return nil, err
		}
		tunnels = append(tunnels, t)
	}
	return tunnels, rows.Err()
}

// DeleteGRETunnel deletes a GRE tunnel by name
func (r *Repository) DeleteGRETunnel(name string) error {
	result, err := r.db.Exec("DELETE FROM gre_tunnels WHERE name = ?", name)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("GRE tunnel %q not found", name)
	}
	return nil
}
