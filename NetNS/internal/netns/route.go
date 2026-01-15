package netns

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// RouteManager handles routing operations
type RouteManager struct {
	namespaceManager *Manager
}

// NewRouteManager creates a new route manager
func NewRouteManager(namespaceManager *Manager) *RouteManager {
	return &RouteManager{namespaceManager: namespaceManager}
}

// Add adds a route
// Parameters:
//   - destination: destination network in CIDR format (or "default" for default route)
//   - gateway: gateway IP address
//   - interfaceName: output interface name
//   - namespaceName: namespace to add route in (empty = host)
func (routeManager *RouteManager) Add(destination, gateway, interfaceName, namespaceName string) error {
	networkRoute, err := routeManager.buildRoute(destination, gateway, interfaceName, namespaceName)
	if err != nil {
		return err
	}

	if namespaceName == "" {
		return netlink.RouteAdd(networkRoute)
	}

	netlinkHandle, err := routeManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	return netlinkHandle.RouteAdd(networkRoute)
}

// Delete removes a route
// Parameters:
//   - destination: destination network in CIDR format (or "default")
//   - namespaceName: namespace to delete route from (empty = host)
func (routeManager *RouteManager) Delete(destination, namespaceName string) error {
	var destinationNetwork *net.IPNet
	var err error

	if destination != "default" && destination != "" {
		_, destinationNetwork, err = net.ParseCIDR(destination)
		if err != nil {
			return fmt.Errorf("invalid destination %q: %w", destination, err)
		}
	}

	networkRoute := &netlink.Route{Dst: destinationNetwork}

	if namespaceName == "" {
		return netlink.RouteDel(networkRoute)
	}

	netlinkHandle, err := routeManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return err
	}
	defer netlinkHandle.Close()

	return netlinkHandle.RouteDel(networkRoute)
}

// List returns all routes in a namespace
// Parameters:
//   - namespaceName: namespace to list routes from (empty = host)
func (routeManager *RouteManager) List(namespaceName string) ([]netlink.Route, error) {
	if namespaceName == "" {
		return netlink.RouteList(nil, familyAll)
	}

	netlinkHandle, err := routeManager.namespaceManager.GetNetlinkHandle(namespaceName)
	if err != nil {
		return nil, err
	}
	defer netlinkHandle.Close()

	return netlinkHandle.RouteList(nil, familyAll)
}

// RouteInfo contains formatted route information
type RouteInfo struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway,omitempty"`
	Interface   string `json:"interface,omitempty"`
	Scope       string `json:"scope"`
	Protocol    string `json:"protocol"`
}

// GetRouteInfos returns formatted route information
// Parameters:
//   - namespaceName: namespace to get route info from (empty = host)
func (routeManager *RouteManager) GetRouteInfos(namespaceName string) ([]RouteInfo, error) {
	routes, err := routeManager.List(namespaceName)
	if err != nil {
		return nil, err
	}

	var routeInfoList []RouteInfo
	for _, routeEntry := range routes {
		destinationString := "default"
		if routeEntry.Dst != nil {
			destinationString = routeEntry.Dst.String()
		}

		gatewayString := ""
		if routeEntry.Gw != nil {
			gatewayString = routeEntry.Gw.String()
		}

		interfaceName := ""
		if routeEntry.LinkIndex > 0 {
			if namespaceName == "" {
				networkLink, err := netlink.LinkByIndex(routeEntry.LinkIndex)
				if err == nil {
					interfaceName = networkLink.Attrs().Name
				}
			} else {
				netlinkHandle, err := routeManager.namespaceManager.GetNetlinkHandle(namespaceName)
				if err == nil {
					networkLink, err := netlinkHandle.LinkByIndex(routeEntry.LinkIndex)
					if err == nil {
						interfaceName = networkLink.Attrs().Name
					}
					netlinkHandle.Close()
				}
			}
		}

		routeInfoList = append(routeInfoList, RouteInfo{
			Destination: destinationString,
			Gateway:     gatewayString,
			Interface:   interfaceName,
			Scope:       scopeToString(int(routeEntry.Scope)),
			Protocol:    protocolToString(int(routeEntry.Protocol)),
		})
	}

	return routeInfoList, nil
}

// buildRoute creates a netlink Route from parameters
// Parameters:
//   - destination: destination network in CIDR format
//   - gateway: gateway IP address
//   - interfaceName: output interface name
//   - namespaceName: namespace context for interface lookup
func (routeManager *RouteManager) buildRoute(destination, gateway, interfaceName, namespaceName string) (*netlink.Route, error) {
	networkRoute := &netlink.Route{}

	// Parse destination
	if destination != "default" && destination != "" {
		_, destinationNetwork, err := net.ParseCIDR(destination)
		if err != nil {
			return nil, fmt.Errorf("invalid destination %q: %w", destination, err)
		}
		networkRoute.Dst = destinationNetwork
	}

	// Parse gateway
	if gateway != "" {
		gatewayIP := net.ParseIP(gateway)
		if gatewayIP == nil {
			return nil, fmt.Errorf("invalid gateway %q", gateway)
		}
		networkRoute.Gw = gatewayIP
	}

	// Get interface index
	if interfaceName != "" {
		var networkLink netlink.Link
		var err error

		if namespaceName == "" {
			networkLink, err = netlink.LinkByName(interfaceName)
		} else {
			netlinkHandle, handleErr := routeManager.namespaceManager.GetNetlinkHandle(namespaceName)
			if handleErr != nil {
				return nil, handleErr
			}
			networkLink, err = netlinkHandle.LinkByName(interfaceName)
			netlinkHandle.Close()
		}

		if err != nil {
			return nil, fmt.Errorf("failed to find interface %q: %w", interfaceName, err)
		}
		networkRoute.LinkIndex = networkLink.Attrs().Index
	}

	return networkRoute, nil
}

// AddDefault adds a default route
// Parameters:
//   - gateway: gateway IP address
//   - interfaceName: output interface name
//   - namespaceName: namespace to add route in (empty = host)
func (routeManager *RouteManager) AddDefault(gateway, interfaceName, namespaceName string) error {
	return routeManager.Add("", gateway, interfaceName, namespaceName)
}

// DeleteDefault removes the default route
// Parameters:
//   - namespaceName: namespace to delete route from (empty = host)
func (routeManager *RouteManager) DeleteDefault(namespaceName string) error {
	return routeManager.Delete("default", namespaceName)
}

func protocolToString(protocolValue int) string {
	switch protocolValue {
	case 0:
		return "unspec"
	case 1:
		return "redirect"
	case 2:
		return "kernel"
	case 3:
		return "boot"
	case 4:
		return "static"
	case 8:
		return "gated"
	case 9:
		return "ra"
	case 10:
		return "mrt"
	case 11:
		return "zebra"
	case 12:
		return "bird"
	case 13:
		return "dnrouted"
	case 14:
		return "xorp"
	case 15:
		return "ntk"
	case 16:
		return "dhcp"
	case 17:
		return "mrouted"
	case 42:
		return "babel"
	case 186:
		return "bgp"
	case 187:
		return "isis"
	case 188:
		return "ospf"
	case 189:
		return "rip"
	case 192:
		return "eigrp"
	default:
		return fmt.Sprintf("%d", protocolValue)
	}
}
