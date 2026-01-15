package netns

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// GREManager handles GRE tunnel operations
type GREManager struct {
	namespaceManager *Manager
}

// NewGREManager creates a new GRE tunnel manager
func NewGREManager(namespaceManager *Manager) *GREManager {
	return &GREManager{namespaceManager: namespaceManager}
}

// GRETunnel represents a GRE tunnel configuration
type GRETunnel struct {
	Name      string // Tunnel interface name (e.g., gre1)
	LocalIP   string // Local endpoint IP address
	RemoteIP  string // Remote endpoint IP address
	Key       uint32 // Optional GRE key for multiplexing (0 = no key)
	TTL       uint8  // Time to live (0 = inherit from inner packet)
	Namespace string // Namespace where tunnel is created (empty = host)
}

// Create creates a GRE tunnel
// Parameters:
//   - tunnelName: tunnel interface name (e.g., "gre1")
//   - localIP: local endpoint IP address
//   - remoteIP: remote endpoint IP address
//   - namespaceName: namespace to create tunnel in (empty = host)
func (greManager *GREManager) Create(tunnelName, localIP, remoteIP, namespaceName string) error {
	return greManager.CreateWithOptions(GRETunnel{
		Name:      tunnelName,
		LocalIP:   localIP,
		RemoteIP:  remoteIP,
		Namespace: namespaceName,
	})
}

// CreateWithOptions creates a GRE tunnel with full options
func (greManager *GREManager) CreateWithOptions(tunnelConfig GRETunnel) error {
	// Parse IP addresses
	localIPAddress := net.ParseIP(tunnelConfig.LocalIP)
	if localIPAddress == nil {
		return fmt.Errorf("invalid local IP: %s", tunnelConfig.LocalIP)
	}

	remoteIPAddress := net.ParseIP(tunnelConfig.RemoteIP)
	if remoteIPAddress == nil {
		return fmt.Errorf("invalid remote IP: %s", tunnelConfig.RemoteIP)
	}

	// Create GRE tunnel link
	greTunnelLink := &netlink.Gretun{
		LinkAttrs: netlink.LinkAttrs{
			Name: tunnelConfig.Name,
		},
		Local:  localIPAddress,
		Remote: remoteIPAddress,
	}

	// Set optional parameters
	if tunnelConfig.Key > 0 {
		greTunnelLink.IKey = tunnelConfig.Key
		greTunnelLink.OKey = tunnelConfig.Key
	}

	if tunnelConfig.TTL > 0 {
		greTunnelLink.Ttl = tunnelConfig.TTL
	}

	// Create in host or namespace
	if tunnelConfig.Namespace == "" {
		if err := netlink.LinkAdd(greTunnelLink); err != nil {
			return fmt.Errorf("failed to create GRE tunnel: %w", err)
		}
		return netlink.LinkSetUp(greTunnelLink)
	}

	// Create in namespace
	netlinkHandle, err := greManager.namespaceManager.GetNetlinkHandle(tunnelConfig.Namespace)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	if err := netlinkHandle.LinkAdd(greTunnelLink); err != nil {
		return fmt.Errorf("failed to create GRE tunnel in namespace %s: %w", tunnelConfig.Namespace, err)
	}

	// Get the link again to set it up
	tunnelLink, err := netlinkHandle.LinkByName(tunnelConfig.Name)
	if err != nil {
		return err
	}

	return netlinkHandle.LinkSetUp(tunnelLink)
}

// Delete removes a GRE tunnel
// Parameters:
//   - tunnelName: name of the GRE tunnel interface to delete
//   - namespaceName: namespace where tunnel exists (empty = host)
func (greManager *GREManager) Delete(tunnelName, namespaceName string) error {
	if namespaceName == "" {
		tunnelLink, err := netlink.LinkByName(tunnelName)
		if err != nil {
			return fmt.Errorf("GRE tunnel %q not found: %w", tunnelName, err)
		}
		return netlink.LinkDel(tunnelLink)
	}

	netlinkHandle, err := greManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	tunnelLink, err := netlinkHandle.LinkByName(tunnelName)
	if err != nil {
		return fmt.Errorf("GRE tunnel %q not found in namespace %q: %w", tunnelName, namespaceName, err)
	}

	return netlinkHandle.LinkDel(tunnelLink)
}

// SetUp brings a GRE tunnel interface up
// Parameters:
//   - tunnelName: name of the GRE tunnel interface
//   - namespaceName: namespace where tunnel exists (empty = host)
func (greManager *GREManager) SetUp(tunnelName, namespaceName string) error {
	if namespaceName == "" {
		tunnelLink, err := netlink.LinkByName(tunnelName)
		if err != nil {
			return err
		}
		return netlink.LinkSetUp(tunnelLink)
	}

	netlinkHandle, err := greManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	tunnelLink, err := netlinkHandle.LinkByName(tunnelName)
	if err != nil {
		return err
	}

	return netlinkHandle.LinkSetUp(tunnelLink)
}

// SetDown brings a GRE tunnel interface down
// Parameters:
//   - tunnelName: name of the GRE tunnel interface
//   - namespaceName: namespace where tunnel exists (empty = host)
func (greManager *GREManager) SetDown(tunnelName, namespaceName string) error {
	if namespaceName == "" {
		tunnelLink, err := netlink.LinkByName(tunnelName)
		if err != nil {
			return err
		}
		return netlink.LinkSetDown(tunnelLink)
	}

	netlinkHandle, err := greManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	tunnelLink, err := netlinkHandle.LinkByName(tunnelName)
	if err != nil {
		return err
	}

	return netlinkHandle.LinkSetDown(tunnelLink)
}

// List returns all GRE tunnels in a namespace (or host if empty)
// Parameters:
//   - namespaceName: namespace to list tunnels from (empty = host)
func (greManager *GREManager) List(namespaceName string) ([]GRETunnelInfo, error) {
	var networkLinks []netlink.Link
	var err error

	if namespaceName == "" {
		networkLinks, err = netlink.LinkList()
	} else {
		netlinkHandle, handleErr := greManager.namespaceManager.GetNetlinkHandle(namespaceName)
		if handleErr != nil {
			return nil, handleErr
		}
		defer netlinkHandle.Close()
		networkLinks, err = netlinkHandle.LinkList()
	}

	if err != nil {
		return nil, err
	}

	var greTunnels []GRETunnelInfo
	for _, networkLink := range networkLinks {
		if networkLink.Type() == "gre" || networkLink.Type() == "gretap" {
			tunnelInfo := GRETunnelInfo{
				Name:  networkLink.Attrs().Name,
				State: "down",
			}

			// Check if up
			if networkLink.Attrs().Flags&1 != 0 { // IFF_UP
				tunnelInfo.State = "up"
			}

			// Get GRE specific attributes
			if greTunnel, ok := networkLink.(*netlink.Gretun); ok {
				if greTunnel.Local != nil {
					tunnelInfo.LocalIP = greTunnel.Local.String()
				}
				if greTunnel.Remote != nil {
					tunnelInfo.RemoteIP = greTunnel.Remote.String()
				}
				tunnelInfo.Key = greTunnel.IKey
				tunnelInfo.TTL = greTunnel.Ttl
			}

			greTunnels = append(greTunnels, tunnelInfo)
		}
	}

	return greTunnels, nil
}

// GRETunnelInfo contains GRE tunnel information
type GRETunnelInfo struct {
	Name     string `json:"name"`
	LocalIP  string `json:"local_ip"`
	RemoteIP string `json:"remote_ip"`
	Key      uint32 `json:"key,omitempty"`
	TTL      uint8  `json:"ttl,omitempty"`
	State    string `json:"state"`
}

// CreatePeerTunnels creates GRE tunnels between two namespaces
// This sets up a point-to-point GRE connection between namespace1 and namespace2
// Parameters:
//   - namespace1Name: first namespace name
//   - namespace1IP: IP address in namespace1 for tunnel endpoint
//   - namespace1TunnelIP: IP address to assign to tunnel interface in namespace1
//   - namespace2Name: second namespace name
//   - namespace2IP: IP address in namespace2 for tunnel endpoint
//   - namespace2TunnelIP: IP address to assign to tunnel interface in namespace2
//   - baseTunnelName: base name for tunnel interfaces
func (greManager *GREManager) CreatePeerTunnels(
	namespace1Name, namespace1IP, namespace1TunnelIP string,
	namespace2Name, namespace2IP, namespace2TunnelIP string,
	baseTunnelName string,
) error {
	// Tunnel names
	tunnel1Name := baseTunnelName + "-1"
	tunnel2Name := baseTunnelName + "-2"

	// Create tunnel in namespace1 (local=namespace1IP, remote=namespace2IP)
	err := greManager.Create(tunnel1Name, namespace1IP, namespace2IP, namespace1Name)
	if err != nil {
		return fmt.Errorf("failed to create tunnel in %s: %w", namespace1Name, err)
	}

	// Create tunnel in namespace2 (local=namespace2IP, remote=namespace1IP)
	err = greManager.Create(tunnel2Name, namespace2IP, namespace1IP, namespace2Name)
	if err != nil {
		// Cleanup on failure
		greManager.Delete(tunnel1Name, namespace1Name)
		return fmt.Errorf("failed to create tunnel in %s: %w", namespace2Name, err)
	}

	// Assign IP addresses to tunnel interfaces
	addressManager := NewAddressManager(greManager.namespaceManager)

	err = addressManager.Add(namespace1TunnelIP, tunnel1Name, namespace1Name)
	if err != nil {
		greManager.Delete(tunnel1Name, namespace1Name)
		greManager.Delete(tunnel2Name, namespace2Name)
		return fmt.Errorf("failed to assign IP to tunnel in %s: %w", namespace1Name, err)
	}

	err = addressManager.Add(namespace2TunnelIP, tunnel2Name, namespace2Name)
	if err != nil {
		greManager.Delete(tunnel1Name, namespace1Name)
		greManager.Delete(tunnel2Name, namespace2Name)
		return fmt.Errorf("failed to assign IP to tunnel in %s: %w", namespace2Name, err)
	}

	return nil
}
