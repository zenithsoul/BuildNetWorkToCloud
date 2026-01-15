package netns

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

// BridgeManager handles bridge operations
type BridgeManager struct {
	namespaceManager *Manager
}

// NewBridgeManager creates a new bridge manager
func NewBridgeManager(namespaceManager *Manager) *BridgeManager {
	return &BridgeManager{namespaceManager: namespaceManager}
}

// Create creates a new bridge
// Parameters:
//   - bridgeName: name of the bridge to create
//   - namespaceName: namespace to create bridge in (empty = host)
func (bridgeManager *BridgeManager) Create(bridgeName, namespaceName string) error {
	bridgeLink := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: bridgeName,
		},
	}

	if namespaceName == "" {
		if err := netlink.LinkAdd(bridgeLink); err != nil {
			return fmt.Errorf("failed to create bridge: %w", err)
		}
		return netlink.LinkSetUp(bridgeLink)
	}

	netlinkHandle, err := bridgeManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	if err := netlinkHandle.LinkAdd(bridgeLink); err != nil {
		return fmt.Errorf("failed to create bridge: %w", err)
	}

	// Get the link again to set it up
	networkLink, err := netlinkHandle.LinkByName(bridgeName)
	if err != nil {
		return err
	}

	return netlinkHandle.LinkSetUp(networkLink)
}

// Delete removes a bridge
// Parameters:
//   - bridgeName: name of the bridge to delete
//   - namespaceName: namespace where bridge exists (empty = host)
func (bridgeManager *BridgeManager) Delete(bridgeName, namespaceName string) error {
	if namespaceName == "" {
		networkLink, err := netlink.LinkByName(bridgeName)
		if err != nil {
			return fmt.Errorf("bridge %q not found: %w", bridgeName, err)
		}
		return netlink.LinkDel(networkLink)
	}

	netlinkHandle, err := bridgeManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	networkLink, err := netlinkHandle.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("bridge %q not found in namespace %q: %w", bridgeName, namespaceName, err)
	}

	return netlinkHandle.LinkDel(networkLink)
}

// AddPort adds an interface to a bridge
// Parameters:
//   - bridgeName: name of the bridge
//   - interfaceName: name of the interface to add as port
//   - namespaceName: namespace where bridge and interface exist (empty = host)
func (bridgeManager *BridgeManager) AddPort(bridgeName, interfaceName, namespaceName string) error {
	if namespaceName == "" {
		bridgeLink, err := netlink.LinkByName(bridgeName)
		if err != nil {
			return fmt.Errorf("bridge %q not found: %w", bridgeName, err)
		}

		interfaceLink, err := netlink.LinkByName(interfaceName)
		if err != nil {
			return fmt.Errorf("interface %q not found: %w", interfaceName, err)
		}

		return netlink.LinkSetMaster(interfaceLink, bridgeLink)
	}

	netlinkHandle, err := bridgeManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	bridgeLink, err := netlinkHandle.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("bridge %q not found in namespace %q: %w", bridgeName, namespaceName, err)
	}

	interfaceLink, err := netlinkHandle.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("interface %q not found in namespace %q: %w", interfaceName, namespaceName, err)
	}

	return netlinkHandle.LinkSetMaster(interfaceLink, bridgeLink)
}

// RemovePort removes an interface from a bridge
// Parameters:
//   - interfaceName: name of the interface to remove from bridge
//   - namespaceName: namespace where interface exists (empty = host)
func (bridgeManager *BridgeManager) RemovePort(interfaceName, namespaceName string) error {
	if namespaceName == "" {
		interfaceLink, err := netlink.LinkByName(interfaceName)
		if err != nil {
			return fmt.Errorf("interface %q not found: %w", interfaceName, err)
		}
		return netlink.LinkSetNoMaster(interfaceLink)
	}

	netlinkHandle, err := bridgeManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	interfaceLink, err := netlinkHandle.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("interface %q not found in namespace %q: %w", interfaceName, namespaceName, err)
	}

	return netlinkHandle.LinkSetNoMaster(interfaceLink)
}

// ListPorts returns all interfaces attached to a bridge
// Parameters:
//   - bridgeName: name of the bridge
//   - namespaceName: namespace where bridge exists (empty = host)
func (bridgeManager *BridgeManager) ListPorts(bridgeName, namespaceName string) ([]string, error) {
	var networkLinks []netlink.Link
	var bridgeLink netlink.Link
	var err error

	if namespaceName == "" {
		bridgeLink, err = netlink.LinkByName(bridgeName)
		if err != nil {
			return nil, fmt.Errorf("bridge %q not found: %w", bridgeName, err)
		}
		networkLinks, err = netlink.LinkList()
	} else {
		netlinkHandle, handleErr := bridgeManager.namespaceManager.GetNetlinkHandle(namespaceName)
		if handleErr != nil {
			return nil, handleErr
		}
		defer netlinkHandle.Close()

		bridgeLink, err = netlinkHandle.LinkByName(bridgeName)
		if err != nil {
			return nil, fmt.Errorf("bridge %q not found in namespace %q: %w", bridgeName, namespaceName, err)
		}
		networkLinks, err = netlinkHandle.LinkList()
	}

	if err != nil {
		return nil, err
	}

	bridgeIndex := bridgeLink.Attrs().Index
	var portNames []string

	for _, networkLink := range networkLinks {
		if networkLink.Attrs().MasterIndex == bridgeIndex {
			portNames = append(portNames, networkLink.Attrs().Name)
		}
	}

	return portNames, nil
}

// List returns all bridges in a namespace
// Parameters:
//   - namespaceName: namespace to list bridges from (empty = host)
func (bridgeManager *BridgeManager) List(namespaceName string) ([]string, error) {
	var networkLinks []netlink.Link
	var err error

	if namespaceName == "" {
		networkLinks, err = netlink.LinkList()
	} else {
		netlinkHandle, handleErr := bridgeManager.namespaceManager.GetNetlinkHandle(namespaceName)
		if handleErr != nil {
			return nil, handleErr
		}
		defer netlinkHandle.Close()
		networkLinks, err = netlinkHandle.LinkList()
	}

	if err != nil {
		return nil, err
	}

	var bridgeNames []string
	for _, networkLink := range networkLinks {
		if networkLink.Type() == "bridge" {
			bridgeNames = append(bridgeNames, networkLink.Attrs().Name)
		}
	}

	return bridgeNames, nil
}

// BridgeInfo contains bridge information with ports
type BridgeInfo struct {
	Name  string   `json:"name"`
	Ports []string `json:"ports"`
	State string   `json:"state"`
}

// GetBridgeInfos returns detailed bridge information
// Parameters:
//   - namespaceName: namespace to get bridge info from (empty = host)
func (bridgeManager *BridgeManager) GetBridgeInfos(namespaceName string) ([]BridgeInfo, error) {
	bridgeNames, err := bridgeManager.List(namespaceName)
	if err != nil {
		return nil, err
	}

	var bridgeInfoList []BridgeInfo
	for _, bridgeName := range bridgeNames {
		portNames, _ := bridgeManager.ListPorts(bridgeName, namespaceName)

		bridgeState := "down"
		var networkLink netlink.Link
		if namespaceName == "" {
			networkLink, _ = netlink.LinkByName(bridgeName)
		} else {
			netlinkHandle, _ := bridgeManager.namespaceManager.GetNetlinkHandle(namespaceName)
			if netlinkHandle != nil {
				networkLink, _ = netlinkHandle.LinkByName(bridgeName)
				netlinkHandle.Close()
			}
		}
		if networkLink != nil && networkLink.Attrs().Flags&1 != 0 { // IFF_UP
			bridgeState = "up"
		}

		bridgeInfoList = append(bridgeInfoList, BridgeInfo{
			Name:  bridgeName,
			Ports: portNames,
			State: bridgeState,
		})
	}

	return bridgeInfoList, nil
}

// SetUp brings a bridge up
// Parameters:
//   - bridgeName: name of the bridge to bring up
//   - namespaceName: namespace where bridge exists (empty = host)
func (bridgeManager *BridgeManager) SetUp(bridgeName, namespaceName string) error {
	if namespaceName == "" {
		networkLink, err := netlink.LinkByName(bridgeName)
		if err != nil {
			return err
		}
		return netlink.LinkSetUp(networkLink)
	}

	netlinkHandle, err := bridgeManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	networkLink, err := netlinkHandle.LinkByName(bridgeName)
	if err != nil {
		return err
	}

	return netlinkHandle.LinkSetUp(networkLink)
}

// SetDown brings a bridge down
// Parameters:
//   - bridgeName: name of the bridge to bring down
//   - namespaceName: namespace where bridge exists (empty = host)
func (bridgeManager *BridgeManager) SetDown(bridgeName, namespaceName string) error {
	if namespaceName == "" {
		networkLink, err := netlink.LinkByName(bridgeName)
		if err != nil {
			return err
		}
		return netlink.LinkSetDown(networkLink)
	}

	netlinkHandle, err := bridgeManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	networkLink, err := netlinkHandle.LinkByName(bridgeName)
	if err != nil {
		return err
	}

	return netlinkHandle.LinkSetDown(networkLink)
}
