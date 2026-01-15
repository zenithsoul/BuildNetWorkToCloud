package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zenith/netns-mgr/internal/netns"
)

var (
	routeGateway   string
	routeInterface string
	routeNs        string
)

var routeCmd = &cobra.Command{
	Use:   "route",
	Short: "Manage routes",
}

var routeAddCmd = &cobra.Command{
	Use:   "add <destination>",
	Short: "Add a route",
	Long: `Add a route to the routing table.

The destination must be in CIDR notation (e.g., 10.0.0.0/8) or "default".

Examples:
  # Add default route
  netns-mgr route add default --gateway 10.0.0.1

  # Add route to specific network
  netns-mgr route add 192.168.0.0/24 --gateway 10.0.0.1

  # Add route via interface
  netns-mgr route add 192.168.0.0/24 --interface eth0

  # Add route in namespace
  netns-mgr route add default --gateway 10.0.0.1 --ns myns`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		destinationNetwork := args[0]

		if routeGateway == "" && routeInterface == "" {
			return fmt.Errorf("either --gateway or --interface is required")
		}

		namespaceManager := netns.NewManager()
		routeManager := netns.NewRouteManager(namespaceManager)

		// Add to system
		if err := routeManager.Add(destinationNetwork, routeGateway, routeInterface, routeNs); err != nil {
			return err
		}

		// Get namespace ID for DB
		var namespaceID *int64
		if routeNs != "" {
			namespaceRecord, err := Repo.GetNamespaceByName(routeNs)
			if err == nil && namespaceRecord != nil {
				namespaceID = &namespaceRecord.ID
			}
		}

		// Record in database
		_, err := Repo.CreateRoute(namespaceID, destinationNetwork, routeGateway, routeInterface)
		if err != nil {
			// Rollback system change
			routeManager.Delete(destinationNetwork, routeNs)
			return fmt.Errorf("failed to record route: %w", err)
		}

		fmt.Printf("Added route: %s\n", destinationNetwork)
		return nil
	},
}

var routeDeleteCmd = &cobra.Command{
	Use:   "delete <destination>",
	Short: "Delete a route",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		destinationNetwork := args[0]

		namespaceManager := netns.NewManager()
		routeManager := netns.NewRouteManager(namespaceManager)

		// Delete from system
		if err := routeManager.Delete(destinationNetwork, routeNs); err != nil {
			return err
		}

		fmt.Printf("Deleted route: %s\n", destinationNetwork)
		return nil
	},
}

var routeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List routes",
	RunE: func(cmd *cobra.Command, args []string) error {
		namespaceManager := netns.NewManager()
		routeManager := netns.NewRouteManager(namespaceManager)

		routeInfos, err := routeManager.GetRouteInfos(routeNs)
		if err != nil {
			return err
		}

		if len(routeInfos) == 0 {
			fmt.Println("No routes found")
			return nil
		}

		tableWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tableWriter, "DESTINATION\tGATEWAY\tINTERFACE\tSCOPE\tPROTOCOL")

		for _, routeInfo := range routeInfos {
			gatewayDisplay := routeInfo.Gateway
			if gatewayDisplay == "" {
				gatewayDisplay = "-"
			}
			interfaceDisplay := routeInfo.Interface
			if interfaceDisplay == "" {
				interfaceDisplay = "-"
			}

			fmt.Fprintf(tableWriter, "%s\t%s\t%s\t%s\t%s\n",
				routeInfo.Destination,
				gatewayDisplay,
				interfaceDisplay,
				routeInfo.Scope,
				routeInfo.Protocol,
			)
		}

		tableWriter.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(routeCmd)

	routeAddCmd.Flags().StringVar(&routeGateway, "gateway", "", "gateway address")
	routeAddCmd.Flags().StringVar(&routeInterface, "interface", "", "interface name")
	routeAddCmd.Flags().StringVar(&routeNs, "ns", "", "namespace")

	routeDeleteCmd.Flags().StringVar(&routeNs, "ns", "", "namespace")

	routeListCmd.Flags().StringVar(&routeNs, "ns", "", "namespace")

	routeCmd.AddCommand(routeAddCmd)
	routeCmd.AddCommand(routeDeleteCmd)
	routeCmd.AddCommand(routeListCmd)
}
