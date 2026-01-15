package db

import "time"

// Namespace represents a network namespace
type Namespace struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Metadata  string    `json:"metadata,omitempty"`
}

// VethPair represents a virtual ethernet pair
type VethPair struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	PeerName  string    `json:"peer_name"`
	NsID      *int64    `json:"ns_id,omitempty"`
	PeerNsID  *int64    `json:"peer_ns_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// IPAddress represents an IP address assigned to an interface
type IPAddress struct {
	ID            int64     `json:"id"`
	InterfaceName string    `json:"interface_name"`
	NsID          *int64    `json:"ns_id,omitempty"`
	Address       string    `json:"address"` // CIDR format
	CreatedAt     time.Time `json:"created_at"`
}

// Route represents a network route
type Route struct {
	ID            int64     `json:"id"`
	NsID          *int64    `json:"ns_id,omitempty"`
	Destination   string    `json:"destination"` // CIDR or "default"
	Gateway       string    `json:"gateway,omitempty"`
	InterfaceName string    `json:"interface_name,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Bridge represents a network bridge
type Bridge struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	NsID      *int64    `json:"ns_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// BridgePort represents a port attached to a bridge
type BridgePort struct {
	ID            int64     `json:"id"`
	BridgeID      int64     `json:"bridge_id"`
	InterfaceName string    `json:"interface_name"`
	CreatedAt     time.Time `json:"created_at"`
}

// GRETunnel represents a GRE tunnel configuration
type GRETunnel struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`       // Tunnel interface name (e.g., gre1)
	LocalIP   string    `json:"local_ip"`   // Local endpoint IP address
	RemoteIP  string    `json:"remote_ip"`  // Remote endpoint IP address
	Key       uint32    `json:"key"`        // GRE key for multiplexing (0 = no key)
	TTL       uint8     `json:"ttl"`        // Time to live (0 = inherit)
	NsID      *int64    `json:"ns_id"`      // Namespace where tunnel is created
	CreatedAt time.Time `json:"created_at"`
}

// NamespaceWithDetails includes related resources
type NamespaceWithDetails struct {
	Namespace
	VethPairs   []VethPair  `json:"veth_pairs,omitempty"`
	IPAddresses []IPAddress `json:"ip_addresses,omitempty"`
	Routes      []Route     `json:"routes,omitempty"`
	Bridges     []Bridge    `json:"bridges,omitempty"`
	GRETunnels  []GRETunnel `json:"gre_tunnels,omitempty"`
}
