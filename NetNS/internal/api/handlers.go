package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenith/netns-mgr/internal/netns"
)

// === Namespace Handlers ===

type createNamespaceRequest struct {
	Name     string `json:"name" binding:"required"`
	Metadata string `json:"metadata"`
}

func (s *Server) createNamespace(c *gin.Context) {
	var request createNamespaceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create in system
	if err := s.namespaceManager.Create(request.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record in database
	ns, err := s.repository.CreateNamespace(request.Name, request.Metadata)
	if err != nil {
		s.namespaceManager.Delete(request.Name)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ns)
}

func (s *Server) listNamespaces(c *gin.Context) {
	namespaces, err := s.repository.ListNamespaces()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, namespaces)
}

func (s *Server) getNamespace(c *gin.Context) {
	name := c.Param("name")

	ns, err := s.repository.GetNamespaceByName(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if ns == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "namespace not found"})
		return
	}

	c.JSON(http.StatusOK, ns)
}

func (s *Server) deleteNamespace(c *gin.Context) {
	name := c.Param("name")

	// Delete from system
	if err := s.namespaceManager.Delete(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Remove from database
	s.repository.DeleteNamespace(name)

	c.JSON(http.StatusOK, gin.H{"message": "namespace deleted"})
}

// === Veth Handlers ===

type createVethRequest struct {
	Name      string `json:"name" binding:"required"`
	PeerName  string `json:"peer_name" binding:"required"`
	Namespace string `json:"namespace"`
	PeerNs    string `json:"peer_namespace"`
}

func (s *Server) createVeth(c *gin.Context) {
	var request createVethRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create in system
	if err := s.vethManager.Create(request.Name, request.PeerName, request.Namespace, request.PeerNs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get namespace IDs
	var nsID, peerNsID *int64
	if request.Namespace != "" {
		if ns, _ := s.repository.GetNamespaceByName(request.Namespace); ns != nil {
			nsID = &ns.ID
		}
	}
	if request.PeerNs != "" {
		if ns, _ := s.repository.GetNamespaceByName(request.PeerNs); ns != nil {
			peerNsID = &ns.ID
		}
	}

	// Record in database
	veth, err := s.repository.CreateVethPair(request.Name, request.PeerName, nsID, peerNsID)
	if err != nil {
		s.vethManager.Delete(request.Name)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, veth)
}

func (s *Server) listVeths(c *gin.Context) {
	veths, err := s.repository.ListVethPairs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, veths)
}

func (s *Server) deleteVeth(c *gin.Context) {
	name := c.Param("name")

	// Delete from system
	if err := s.vethManager.Delete(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Remove from database
	s.repository.DeleteVethPair(name)

	c.JSON(http.StatusOK, gin.H{"message": "veth pair deleted"})
}

// === Address Handlers ===

type addAddressRequest struct {
	Interface string `json:"interface" binding:"required"`
	Address   string `json:"address" binding:"required"`
	Namespace string `json:"namespace"`
}

func (s *Server) addAddress(c *gin.Context) {
	var request addAddressRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Add to system
	if err := s.addressManager.Add(request.Address, request.Interface, request.Namespace); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get namespace ID
	var nsID *int64
	if request.Namespace != "" {
		if ns, _ := s.repository.GetNamespaceByName(request.Namespace); ns != nil {
			nsID = &ns.ID
		}
	}

	// Record in database
	addr, err := s.repository.CreateIPAddress(request.Interface, nsID, request.Address)
	if err != nil {
		s.addressManager.Delete(request.Address, request.Interface, request.Namespace)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, addr)
}

func (s *Server) listAddresses(c *gin.Context) {
	nsName := c.Query("namespace")

	var nsID *int64
	if nsName != "" {
		if ns, _ := s.repository.GetNamespaceByName(nsName); ns != nil {
			nsID = &ns.ID
		}
	}

	addresses, err := s.repository.ListIPAddresses(nsID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, addresses)
}

func (s *Server) deleteAddress(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	// Get address info for system deletion
	addr, err := s.repository.GetIPAddress(id)
	if err != nil || addr == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "address not found"})
		return
	}

	// Get namespace name
	var nsName string
	if addr.NsID != nil {
		if ns, _ := s.repository.GetNamespace(*addr.NsID); ns != nil {
			nsName = ns.Name
		}
	}

	// Delete from system
	if err := s.addressManager.Delete(addr.Address, addr.InterfaceName, nsName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Remove from database
	s.repository.DeleteIPAddress(id)

	c.JSON(http.StatusOK, gin.H{"message": "address deleted"})
}

// === Route Handlers ===

type addRouteRequest struct {
	Destination string `json:"destination" binding:"required"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Namespace   string `json:"namespace"`
}

func (s *Server) addRoute(c *gin.Context) {
	var request addRouteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.Gateway == "" && request.Interface == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "either gateway or interface is required"})
		return
	}

	// Add to system
	if err := s.routeManager.Add(request.Destination, request.Gateway, request.Interface, request.Namespace); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get namespace ID
	var nsID *int64
	if request.Namespace != "" {
		if ns, _ := s.repository.GetNamespaceByName(request.Namespace); ns != nil {
			nsID = &ns.ID
		}
	}

	// Record in database
	route, err := s.repository.CreateRoute(nsID, request.Destination, request.Gateway, request.Interface)
	if err != nil {
		s.routeManager.Delete(request.Destination, request.Namespace)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, route)
}

func (s *Server) listRoutes(c *gin.Context) {
	nsName := c.Query("namespace")

	var nsID *int64
	if nsName != "" {
		if ns, _ := s.repository.GetNamespaceByName(nsName); ns != nil {
			nsID = &ns.ID
		}
	}

	routes, err := s.repository.ListRoutes(nsID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, routes)
}

func (s *Server) deleteRoute(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	// Get route info for system deletion
	route, err := s.repository.GetRoute(id)
	if err != nil || route == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
		return
	}

	// Get namespace name
	var nsName string
	if route.NsID != nil {
		if ns, _ := s.repository.GetNamespace(*route.NsID); ns != nil {
			nsName = ns.Name
		}
	}

	// Delete from system
	if err := s.routeManager.Delete(route.Destination, nsName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Remove from database
	s.repository.DeleteRoute(id)

	c.JSON(http.StatusOK, gin.H{"message": "route deleted"})
}

// === Bridge Handlers ===

type createBridgeRequest struct {
	Name      string `json:"name" binding:"required"`
	Namespace string `json:"namespace"`
}

func (s *Server) createBridge(c *gin.Context) {
	var request createBridgeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create in system
	if err := s.bridgeManager.Create(request.Name, request.Namespace); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get namespace ID
	var nsID *int64
	if request.Namespace != "" {
		if ns, _ := s.repository.GetNamespaceByName(request.Namespace); ns != nil {
			nsID = &ns.ID
		}
	}

	// Record in database
	bridge, err := s.repository.CreateBridge(request.Name, nsID)
	if err != nil {
		s.bridgeManager.Delete(request.Name, request.Namespace)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, bridge)
}

func (s *Server) listBridges(c *gin.Context) {
	bridges, err := s.repository.ListBridges()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, bridges)
}

func (s *Server) deleteBridge(c *gin.Context) {
	name := c.Param("name")
	nsName := c.Query("namespace")

	// Delete from system
	if err := s.bridgeManager.Delete(name, nsName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Remove from database
	s.repository.DeleteBridge(name)

	c.JSON(http.StatusOK, gin.H{"message": "bridge deleted"})
}

type addPortRequest struct {
	Interface string `json:"interface" binding:"required"`
}

func (s *Server) addBridgePort(c *gin.Context) {
	bridgeName := c.Param("name")
	nsName := c.Query("namespace")

	var request addPortRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Add to system
	if err := s.bridgeManager.AddPort(bridgeName, request.Interface, nsName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record in database
	if bridge, _ := s.repository.GetBridgeByName(bridgeName); bridge != nil {
		s.repository.AddBridgePort(bridge.ID, request.Interface)
	}

	c.JSON(http.StatusOK, gin.H{"message": "port added"})
}

func (s *Server) removeBridgePort(c *gin.Context) {
	bridgeName := c.Param("name")
	ifaceName := c.Param("iface")
	nsName := c.Query("namespace")

	// Remove from system
	if err := s.bridgeManager.RemovePort(ifaceName, nsName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Remove from database
	if bridge, _ := s.repository.GetBridgeByName(bridgeName); bridge != nil {
		s.repository.RemoveBridgePort(bridge.ID, ifaceName)
	}

	c.JSON(http.StatusOK, gin.H{"message": "port removed"})
}

// === GRE Tunnel Handlers ===

type createGRETunnelRequest struct {
	Name      string `json:"name" binding:"required"`
	LocalIP   string `json:"local_ip" binding:"required"`
	RemoteIP  string `json:"remote_ip" binding:"required"`
	Key       uint32 `json:"key"`
	TTL       uint8  `json:"ttl"`
	Namespace string `json:"namespace"`
}

func (s *Server) createGRETunnel(c *gin.Context) {
	var request createGRETunnelRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create in system
	tunnel := netns.GRETunnel{
		Name:      request.Name,
		LocalIP:   request.LocalIP,
		RemoteIP:  request.RemoteIP,
		Key:       request.Key,
		TTL:       request.TTL,
		Namespace: request.Namespace,
	}
	if err := s.greManager.CreateWithOptions(tunnel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get namespace ID
	var nsID *int64
	if request.Namespace != "" {
		if ns, _ := s.repository.GetNamespaceByName(request.Namespace); ns != nil {
			nsID = &ns.ID
		}
	}

	// Record in database
	greTunnel, err := s.repository.CreateGRETunnel(request.Name, request.LocalIP, request.RemoteIP, request.Key, request.TTL, nsID)
	if err != nil {
		s.greManager.Delete(request.Name, request.Namespace)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, greTunnel)
}

func (s *Server) listGRETunnels(c *gin.Context) {
	nsName := c.Query("namespace")

	var nsID *int64
	if nsName != "" {
		if ns, _ := s.repository.GetNamespaceByName(nsName); ns != nil {
			nsID = &ns.ID
		}
	}

	tunnels, err := s.repository.ListGRETunnels(nsID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tunnels)
}

func (s *Server) getGRETunnel(c *gin.Context) {
	name := c.Param("name")

	tunnel, err := s.repository.GetGRETunnelByName(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if tunnel == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "GRE tunnel not found"})
		return
	}

	c.JSON(http.StatusOK, tunnel)
}

func (s *Server) deleteGRETunnel(c *gin.Context) {
	name := c.Param("name")
	nsName := c.Query("namespace")

	// Delete from system
	if err := s.greManager.Delete(name, nsName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Remove from database
	s.repository.DeleteGRETunnel(name)

	c.JSON(http.StatusOK, gin.H{"message": "GRE tunnel deleted"})
}

func (s *Server) greUp(c *gin.Context) {
	name := c.Param("name")
	nsName := c.Query("namespace")

	if err := s.greManager.SetUp(name, nsName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "GRE tunnel is up"})
}

func (s *Server) greDown(c *gin.Context) {
	name := c.Param("name")
	nsName := c.Query("namespace")

	if err := s.greManager.SetDown(name, nsName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "GRE tunnel is down"})
}

type createPeerTunnelsRequest struct {
	TunnelName  string `json:"tunnel_name" binding:"required"`
	Ns1         string `json:"ns1" binding:"required"`
	Ns1IP       string `json:"ns1_ip" binding:"required"`
	Ns1TunnelIP string `json:"ns1_tunnel_ip" binding:"required"`
	Ns2         string `json:"ns2" binding:"required"`
	Ns2IP       string `json:"ns2_ip" binding:"required"`
	Ns2TunnelIP string `json:"ns2_tunnel_ip" binding:"required"`
}

func (s *Server) createPeerTunnels(c *gin.Context) {
	var request createPeerTunnelsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create peer tunnels in system
	err := s.greManager.CreatePeerTunnels(
		request.Ns1, request.Ns1IP, request.Ns1TunnelIP,
		request.Ns2, request.Ns2IP, request.Ns2TunnelIP,
		request.TunnelName,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record in database
	tunnel1Name := request.TunnelName + "-1"
	tunnel2Name := request.TunnelName + "-2"

	var ns1ID, ns2ID *int64
	if ns1, _ := s.repository.GetNamespaceByName(request.Ns1); ns1 != nil {
		ns1ID = &ns1.ID
	}
	if ns2, _ := s.repository.GetNamespaceByName(request.Ns2); ns2 != nil {
		ns2ID = &ns2.ID
	}

	s.repository.CreateGRETunnel(tunnel1Name, request.Ns1IP, request.Ns2IP, 0, 0, ns1ID)
	s.repository.CreateGRETunnel(tunnel2Name, request.Ns2IP, request.Ns1IP, 0, 0, ns2ID)

	c.JSON(http.StatusCreated, gin.H{
		"message": "peer tunnels created",
		"tunnels": []string{tunnel1Name, tunnel2Name},
	})
}
