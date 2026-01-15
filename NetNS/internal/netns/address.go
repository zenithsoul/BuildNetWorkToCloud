package netns

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// Address family constants
const (
	familyAll = 0 // AF_UNSPEC - matches both IPv4 and IPv6
)

// AddressManager handles IP address operations
type AddressManager struct {
	namespaceManager *Manager
}

// NewAddressManager creates a new address manager
func NewAddressManager(namespaceManager *Manager) *AddressManager {
	return &AddressManager{namespaceManager: namespaceManager}
}

// Add adds an IP address to an interface
// Parameters:
//   - address: IP address in CIDR format (e.g., "10.0.0.1/24")
//   - interfaceName: name of the interface to add address to
//   - namespaceName: namespace where interface exists (empty = host)
func (addressManager *AddressManager) Add(address, interfaceName, namespaceName string) error {
	parsedAddress, err := netlink.ParseAddr(address)
	if err != nil {
		return fmt.Errorf("invalid address %q: %w", address, err)
	}

	if namespaceName == "" {
		networkLink, err := netlink.LinkByName(interfaceName)
		if err != nil {
			return fmt.Errorf("failed to find interface %q: %w", interfaceName, err)
		}
		return netlink.AddrAdd(networkLink, parsedAddress)
	}

	netlinkHandle, err := addressManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	networkLink, err := netlinkHandle.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to find interface %q in namespace %q: %w", interfaceName, namespaceName, err)
	}

	return netlinkHandle.AddrAdd(networkLink, parsedAddress)
}

// Delete removes an IP address from an interface
// Parameters:
//   - address: IP address in CIDR format to remove
//   - interfaceName: name of the interface to remove address from
//   - namespaceName: namespace where interface exists (empty = host)
func (addressManager *AddressManager) Delete(address, interfaceName, namespaceName string) error {
	parsedAddress, err := netlink.ParseAddr(address)
	if err != nil {
		return fmt.Errorf("invalid address %q: %w", address, err)
	}

	if namespaceName == "" {
		networkLink, err := netlink.LinkByName(interfaceName)
		if err != nil {
			return fmt.Errorf("failed to find interface %q: %w", interfaceName, err)
		}
		return netlink.AddrDel(networkLink, parsedAddress)
	}

	netlinkHandle, err := addressManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	networkLink, err := netlinkHandle.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to find interface %q in namespace %q: %w", interfaceName, namespaceName, err)
	}

	return netlinkHandle.AddrDel(networkLink, parsedAddress)
}

// List lists all addresses on an interface
// Parameters:
//   - interfaceName: name of the interface to list addresses for
//   - namespaceName: namespace where interface exists (empty = host)
func (addressManager *AddressManager) List(interfaceName, namespaceName string) ([]netlink.Addr, error) {
	if namespaceName == "" {
		networkLink, err := netlink.LinkByName(interfaceName)
		if err != nil {
			return nil, fmt.Errorf("failed to find interface %q: %w", interfaceName, err)
		}
		return netlink.AddrList(networkLink, familyAll)
	}

	netlinkHandle, err := addressManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return nil, err
	}
	defer netlinkHandle.Close()

	networkLink, err := netlinkHandle.LinkByName(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to find interface %q in namespace %q: %w", interfaceName, namespaceName, err)
	}

	return netlinkHandle.AddrList(networkLink, familyAll)
}

// ListAll lists all addresses in a namespace (or host if empty)
// Parameters:
//   - namespaceName: namespace to list addresses from (empty = host)
func (addressManager *AddressManager) ListAll(namespaceName string) (map[string][]netlink.Addr, error) {
	addressesByInterface := make(map[string][]netlink.Addr)

	if namespaceName == "" {
		networkLinks, err := netlink.LinkList()
		if err != nil {
			return nil, err
		}

		for _, networkLink := range networkLinks {
			addresses, err := netlink.AddrList(networkLink, familyAll)
			if err != nil {
				continue
			}
			if len(addresses) > 0 {
				addressesByInterface[networkLink.Attrs().Name] = addresses
			}
		}
		return addressesByInterface, nil
	}

	netlinkHandle, err := addressManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return nil, err
	}
	defer netlinkHandle.Close()

	networkLinks, err := netlinkHandle.LinkList()
	if err != nil {
		return nil, err
	}

	for _, networkLink := range networkLinks {
		addresses, err := netlinkHandle.AddrList(networkLink, familyAll)
		if err != nil {
			continue
		}
		if len(addresses) > 0 {
			addressesByInterface[networkLink.Attrs().Name] = addresses
		}
	}

	return addressesByInterface, nil
}

// AddressInfo contains formatted address information
type AddressInfo struct {
	Interface string `json:"interface"`
	Address   string `json:"address"`
	Family    string `json:"family"`
	Scope     string `json:"scope"`
}

// GetAddressInfos returns formatted address information
// Parameters:
//   - namespaceName: namespace to get address info from (empty = host)
func (addressManager *AddressManager) GetAddressInfos(namespaceName string) ([]AddressInfo, error) {
	addressesByInterface, err := addressManager.ListAll(namespaceName)
	if err != nil {
		return nil, err
	}

	var addressInfoList []AddressInfo
	for interfaceName, addresses := range addressesByInterface {
		for _, address := range addresses {
			addressFamily := "IPv4"
			if address.IP.To4() == nil {
				addressFamily = "IPv6"
			}

			addressScope := scopeToString(address.Scope)

			addressInfoList = append(addressInfoList, AddressInfo{
				Interface: interfaceName,
				Address:   address.IPNet.String(),
				Family:    addressFamily,
				Scope:     addressScope,
			})
		}
	}

	return addressInfoList, nil
}

func scopeToString(scopeValue int) string {
	switch scopeValue {
	case 0:
		return "global"
	case 200:
		return "site"
	case 253:
		return "link"
	case 254:
		return "host"
	case 255:
		return "nowhere"
	default:
		return fmt.Sprintf("%d", scopeValue)
	}
}

// ParseCIDR parses a CIDR string and returns IP and network
func ParseCIDR(cidr string) (net.IP, *net.IPNet, error) {
	return net.ParseCIDR(cidr)
}
