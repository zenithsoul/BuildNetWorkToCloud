package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zenith/netns-mgr/internal/api"
)

var (
	serverPort int
	serverHost string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the REST API server",
	Long: `Start the REST API server for managing network namespaces.

Examples:
  # Start on default port 8080
  netns-mgr serve

  # Start on custom port
  netns-mgr serve --port 9000

  # Bind to specific interface
  netns-mgr serve --host 0.0.0.0 --port 8080`,
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := fmt.Sprintf("%s:%d", serverHost, serverPort)

		server := api.NewServer(Repo)
		fmt.Printf("Starting API server on %s\n", addr)
		return server.Run(addr)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().IntVar(&serverPort, "port", 8080, "port to listen on")
	serveCmd.Flags().StringVar(&serverHost, "host", "127.0.0.1", "host to bind to")
}
