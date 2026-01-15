package netns

import (
	"fmt"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// VethManager handles veth pair operations
type VethManager struct {
	namespaceManager *Manager
}

// NewVethManager creates a new veth manager
func NewVethManager(namespaceManager *Manager) *VethManager {
	return &VethManager{namespaceManager: namespaceManager}
}

// Create creates a veth pair and optionally moves ends to namespaces
// Parameters:
//   - interfaceName: name of the first veth interface
//   - peerInterfaceName: name of the peer veth interface
//   - namespaceName: namespace to move first interface into (empty = host)
//   - peerNamespaceName: namespace to move peer interface into (empty = host)
func (vethManager *VethManager) Create(interfaceName, peerInterfaceName string, namespaceName, peerNamespaceName string) error {
	// Create the veth pair in the host namespace
	vethPair := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: interfaceName,
		},
		PeerName: peerInterfaceName,
	}

	if err := netlink.LinkAdd(vethPair); err != nil {
		return fmt.Errorf("failed to create veth pair: %w", err)
	}

	// Move first end to namespace if specified
	if namespaceName != "" {
		if err := vethManager.moveToNamespace(interfaceName, namespaceName); err != nil {
			// Cleanup on failure
			netlink.LinkDel(vethPair)
			return err
		}
	}

	// Move peer end to namespace if specified
	if peerNamespaceName != "" {
		if err := vethManager.moveToNamespace(peerInterfaceName, peerNamespaceName); err != nil {
			// Cleanup on failure
			netlink.LinkDel(vethPair)
			return err
		}
	}

	return nil
}

// moveToNamespace moves an interface to a namespace
// Parameters:
//   - interfaceName: name of the interface to move
//   - namespaceName: name of the target namespace
func (vethManager *VethManager) moveToNamespace(interfaceName, namespaceName string) error {
	networkLink, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to find interface %q: %w", interfaceName, err)
	}

	namespaceHandle, err := vethManager.namespaceManager.GetHandle(namespaceName)
	if err != nil {
		return fmt.Errorf("failed to get namespace %q: %w", namespaceName, err)
	}
	defer namespaceHandle.Close()

	if err := netlink.LinkSetNsFd(networkLink, int(namespaceHandle)); err != nil {
		return fmt.Errorf("failed to move interface to namespace: %w", err)
	}

	return nil
}

// Delete removes a veth pair (deleting one end removes both)
// Parameters:
//   - interfaceName: name of the veth interface to delete
func (vethManager *VethManager) Delete(interfaceName string) error {
	// Try to find in host namespace first
	networkLink, err := netlink.LinkByName(interfaceName)
	if err == nil {
		return netlink.LinkDel(networkLink)
	}

	// Interface might be in a namespace, search all namespaces
	namespaceList, _ := vethManager.namespaceManager.List()
	for _, namespaceName := range namespaceList {
		netlinkHandle, err := vethManager.namespaceManager.GetNetlinkHandle(namespaceName)
		if err != nil {
			continue
		}

		networkLink, err := netlinkHandle.LinkByName(interfaceName)
		if err == nil {
			err = netlinkHandle.LinkDel(networkLink)
			netlinkHandle.Close()
			return err
		}
		netlinkHandle.Close()
	}

	return fmt.Errorf("veth %q not found", interfaceName)
}

// SetUp brings an interface up
// Parameters:
//   - interfaceName: name of the interface to bring up
//   - namespaceName: namespace where interface exists (empty = host)
func (vethManager *VethManager) SetUp(interfaceName, namespaceName string) error {
	if namespaceName == "" {
		networkLink, err := netlink.LinkByName(interfaceName)
		if err != nil {
			return fmt.Errorf("failed to find interface %q: %w", interfaceName, err)
		}
		return netlink.LinkSetUp(networkLink)
	}

	netlinkHandle, err := vethManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	networkLink, err := netlinkHandle.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to find interface %q in namespace %q: %w", interfaceName, namespaceName, err)
	}

	return netlinkHandle.LinkSetUp(networkLink)
}

// SetDown brings an interface down
// Parameters:
//   - interfaceName: name of the interface to bring down
//   - namespaceName: namespace where interface exists (empty = host)
func (vethManager *VethManager) SetDown(interfaceName, namespaceName string) error {
	if namespaceName == "" {
		networkLink, err := netlink.LinkByName(interfaceName)
		if err != nil {
			return fmt.Errorf("failed to find interface %q: %w", interfaceName, err)
		}
		return netlink.LinkSetDown(networkLink)
	}

	netlinkHandle, err := vethManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	networkLink, err := netlinkHandle.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to find interface %q in namespace %q: %w", interfaceName, namespaceName, err)
	}

	return netlinkHandle.LinkSetDown(networkLink)
}

// ListInterfaces lists all interfaces in a namespace (or host if empty)
// Parameters:
//   - namespaceName: namespace to list interfaces from (empty = host)
func (vethManager *VethManager) ListInterfaces(namespaceName string) ([]netlink.Link, error) {
	if namespaceName == "" {
		return netlink.LinkList()
	}

	netlinkHandle, err := vethManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return nil, err
	}
	defer netlinkHandle.Close()

	return netlinkHandle.LinkList()
}

// GetInterface returns interface info
// Parameters:
//   - interfaceName: name of the interface to get
//   - namespaceName: namespace where interface exists (empty = host)
func (vethManager *VethManager) GetInterface(interfaceName, namespaceName string) (netlink.Link, error) {
	if namespaceName == "" {
		return netlink.LinkByName(interfaceName)
	}

	netlinkHandle, err := vethManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return nil, err
	}
	defer netlinkHandle.Close()

	return netlinkHandle.LinkByName(interfaceName)
}

// MoveInterface moves an interface between namespaces
// Parameters:
//   - interfaceName: name of the interface to move
//   - fromNamespace: source namespace (empty = host)
//   - toNamespace: destination namespace (empty = host)
func (vethManager *VethManager) MoveInterface(interfaceName, fromNamespace, toNamespace string) error {
	var networkLink netlink.Link
	var err error

	// Get the interface from source
	if fromNamespace == "" {
		networkLink, err = netlink.LinkByName(interfaceName)
	} else {
		netlinkHandle, handleErr := vethManager.namespaceManager.GetNetlinkHandle(fromNamespace)
		if handleErr != nil {
			return handleErr
		}
		networkLink, err = netlinkHandle.LinkByName(interfaceName)
		netlinkHandle.Close()
	}
	if err != nil {
		return fmt.Errorf("failed to find interface %q: %w", interfaceName, err)
	}

	// Move to destination
	if toNamespace == "" {
		// Move to host namespace (pid 1's namespace)
		hostNamespace, err := netns.GetFromPid(1)
		if err != nil {
			return fmt.Errorf("failed to get host namespace: %w", err)
		}
		defer hostNamespace.Close()
		return netlink.LinkSetNsFd(networkLink, int(hostNamespace))
	}

	return vethManager.moveToNamespace(interfaceName, toNamespace)
}
