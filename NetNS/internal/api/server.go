package api

import (
	"github.com/gin-gonic/gin"
	"github.com/zenith/netns-mgr/internal/db"
	"github.com/zenith/netns-mgr/internal/netns"
)

// Server represents the API server
type Server struct {
	router           *gin.Engine
	repository       *db.Repository
	namespaceManager *netns.Manager
	vethManager      *netns.VethManager
	addressManager   *netns.AddressManager
	routeManager     *netns.RouteManager
	bridgeManager    *netns.BridgeManager
	greManager       *netns.GREManager
}

// NewServer creates a new API server
func NewServer(repository *db.Repository) *Server {
	gin.SetMode(gin.ReleaseMode)
	ginRouter := gin.Default()

	namespaceManager := netns.NewManager()

	server := &Server{
		router:           ginRouter,
		repository:       repository,
		namespaceManager: namespaceManager,
		vethManager:      netns.NewVethManager(namespaceManager),
		addressManager:   netns.NewAddressManager(namespaceManager),
		routeManager:     netns.NewRouteManager(namespaceManager),
		bridgeManager:    netns.NewBridgeManager(namespaceManager),
		greManager:       netns.NewGREManager(namespaceManager),
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Middleware
	s.router.Use(gin.Recovery())
	s.router.Use(corsMiddleware())

	// Health check
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API v1
	v1 := s.router.Group("/api/v1")
	{
		// Namespaces
		ns := v1.Group("/namespaces")
		{
			ns.POST("", s.createNamespace)
			ns.GET("", s.listNamespaces)
			ns.GET("/:name", s.getNamespace)
			ns.DELETE("/:name", s.deleteNamespace)
		}

		// Veth pairs
		veths := v1.Group("/veths")
		{
			veths.POST("", s.createVeth)
			veths.GET("", s.listVeths)
			veths.DELETE("/:name", s.deleteVeth)
		}

		// IP addresses
		addrs := v1.Group("/addresses")
		{
			addrs.POST("", s.addAddress)
			addrs.GET("", s.listAddresses)
			addrs.DELETE("/:id", s.deleteAddress)
		}

		// Routes
		routes := v1.Group("/routes")
		{
			routes.POST("", s.addRoute)
			routes.GET("", s.listRoutes)
			routes.DELETE("/:id", s.deleteRoute)
		}

		// Bridges
		bridges := v1.Group("/bridges")
		{
			bridges.POST("", s.createBridge)
			bridges.GET("", s.listBridges)
			bridges.DELETE("/:name", s.deleteBridge)
			bridges.POST("/:name/ports", s.addBridgePort)
			bridges.DELETE("/:name/ports/:iface", s.removeBridgePort)
		}

		// GRE Tunnels
		gre := v1.Group("/gre")
		{
			gre.POST("", s.createGRETunnel)
			gre.GET("", s.listGRETunnels)
			gre.GET("/:name", s.getGRETunnel)
			gre.DELETE("/:name", s.deleteGRETunnel)
			gre.POST("/:name/up", s.greUp)
			gre.POST("/:name/down", s.greDown)
			gre.POST("/peer", s.createPeerTunnels)
		}
	}
}

// Run starts the server
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

// corsMiddleware adds CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
