package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zenith/netns-mgr/internal/netns"
)

var bridgeNs string

var bridgeCmd = &cobra.Command{
	Use:   "bridge",
	Short: "Manage bridges",
}

var bridgeCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a bridge",
	Long: `Create a network bridge.

Examples:
  # Create bridge in host namespace
  netns-mgr bridge create br0

  # Create bridge in a namespace
  netns-mgr bridge create br0 --ns myns`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bridgeName := args[0]

		namespaceManager := netns.NewManager()
		bridgeManager := netns.NewBridgeManager(namespaceManager)

		// Create in system
		if err := bridgeManager.Create(bridgeName, bridgeNs); err != nil {
			return err
		}

		// Get namespace ID for DB
		var namespaceID *int64
		if bridgeNs != "" {
			namespaceRecord, err := Repo.GetNamespaceByName(bridgeNs)
			if err == nil && namespaceRecord != nil {
				namespaceID = &namespaceRecord.ID
			}
		}

		// Record in database
		_, err := Repo.CreateBridge(bridgeName, namespaceID)
		if err != nil {
			// Rollback system change
			bridgeManager.Delete(bridgeName, bridgeNs)
			return fmt.Errorf("failed to record bridge: %w", err)
		}

		fmt.Printf("Created bridge: %s\n", bridgeName)
		return nil
	},
}

var bridgeDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a bridge",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bridgeName := args[0]

		namespaceManager := netns.NewManager()
		bridgeManager := netns.NewBridgeManager(namespaceManager)

		// Delete from system
		if err := bridgeManager.Delete(bridgeName, bridgeNs); err != nil {
			return err
		}

		// Remove from database
		if err := Repo.DeleteBridge(bridgeName); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove from database: %v\n", err)
		}

		fmt.Printf("Deleted bridge: %s\n", bridgeName)
		return nil
	},
}

var bridgeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List bridges",
	RunE: func(cmd *cobra.Command, args []string) error {
		namespaceManager := netns.NewManager()
		bridgeManager := netns.NewBridgeManager(namespaceManager)

		bridgeInfos, err := bridgeManager.GetBridgeInfos(bridgeNs)
		if err != nil {
			return err
		}

		if len(bridgeInfos) == 0 {
			fmt.Println("No bridges found")
			return nil
		}

		tableWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tableWriter, "NAME\tSTATE\tPORTS")

		for _, bridgeInfo := range bridgeInfos {
			portsDisplay := "-"
			if len(bridgeInfo.Ports) > 0 {
				portsDisplay = strings.Join(bridgeInfo.Ports, ", ")
			}

			fmt.Fprintf(tableWriter, "%s\t%s\t%s\n",
				bridgeInfo.Name,
				bridgeInfo.State,
				portsDisplay,
			)
		}

		tableWriter.Flush()
		return nil
	},
}

var bridgeAddPortCmd = &cobra.Command{
	Use:   "add-port <bridge> <interface>",
	Short: "Add an interface to a bridge",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		bridgeName := args[0]
		interfaceName := args[1]

		namespaceManager := netns.NewManager()
		bridgeManager := netns.NewBridgeManager(namespaceManager)

		if err := bridgeManager.AddPort(bridgeName, interfaceName, bridgeNs); err != nil {
			return err
		}

		// Record in database
		bridgeRecord, err := Repo.GetBridgeByName(bridgeName)
		if err == nil && bridgeRecord != nil {
			Repo.AddBridgePort(bridgeRecord.ID, interfaceName)
		}

		fmt.Printf("Added %s to bridge %s\n", interfaceName, bridgeName)
		return nil
	},
}

var bridgeRemovePortCmd = &cobra.Command{
	Use:   "remove-port <bridge> <interface>",
	Short: "Remove an interface from a bridge",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		bridgeName := args[0]
		interfaceName := args[1]

		namespaceManager := netns.NewManager()
		bridgeManager := netns.NewBridgeManager(namespaceManager)

		if err := bridgeManager.RemovePort(interfaceName, bridgeNs); err != nil {
			return err
		}

		// Remove from database
		bridgeRecord, err := Repo.GetBridgeByName(bridgeName)
		if err == nil && bridgeRecord != nil {
			Repo.RemoveBridgePort(bridgeRecord.ID, interfaceName)
		}

		fmt.Printf("Removed %s from bridge %s\n", interfaceName, bridgeName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(bridgeCmd)

	bridgeCreateCmd.Flags().StringVar(&bridgeNs, "ns", "", "namespace")
	bridgeDeleteCmd.Flags().StringVar(&bridgeNs, "ns", "", "namespace")
	bridgeListCmd.Flags().StringVar(&bridgeNs, "ns", "", "namespace")
	bridgeAddPortCmd.Flags().StringVar(&bridgeNs, "ns", "", "namespace")
	bridgeRemovePortCmd.Flags().StringVar(&bridgeNs, "ns", "", "namespace")

	bridgeCmd.AddCommand(bridgeCreateCmd)
	bridgeCmd.AddCommand(bridgeDeleteCmd)
	bridgeCmd.AddCommand(bridgeListCmd)
	bridgeCmd.AddCommand(bridgeAddPortCmd)
	bridgeCmd.AddCommand(bridgeRemovePortCmd)
}
