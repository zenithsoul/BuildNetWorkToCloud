package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zenith/netns-mgr/internal/netns"
)

var (
	ipInterface string
	ipNs        string
)

var ipCmd = &cobra.Command{
	Use:   "ip",
	Short: "Manage IP addresses",
}

var ipAddCmd = &cobra.Command{
	Use:   "add <address>",
	Short: "Add an IP address to an interface",
	Long: `Add an IP address to an interface.

The address must be in CIDR notation (e.g., 10.0.0.1/24).

Examples:
  # Add IP to interface in host namespace
  netns-mgr ip add 10.0.0.1/24 --interface eth0

  # Add IP to interface in a namespace
  netns-mgr ip add 10.0.0.1/24 --interface veth0 --ns myns`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ipAddress := args[0]

		if ipInterface == "" {
			return fmt.Errorf("--interface is required")
		}

		namespaceManager := netns.NewManager()
		addressManager := netns.NewAddressManager(namespaceManager)

		// Add to system
		if err := addressManager.Add(ipAddress, ipInterface, ipNs); err != nil {
			return err
		}

		// Get namespace ID for DB
		var namespaceID *int64
		if ipNs != "" {
			namespaceRecord, err := Repo.GetNamespaceByName(ipNs)
			if err == nil && namespaceRecord != nil {
				namespaceID = &namespaceRecord.ID
			}
		}

		// Record in database
		_, err := Repo.CreateIPAddress(ipInterface, namespaceID, ipAddress)
		if err != nil {
			// Rollback system change
			addressManager.Delete(ipAddress, ipInterface, ipNs)
			return fmt.Errorf("failed to record IP address: %w", err)
		}

		fmt.Printf("Added %s to %s\n", ipAddress, ipInterface)
		return nil
	},
}

var ipDeleteCmd = &cobra.Command{
	Use:   "delete <address>",
	Short: "Remove an IP address from an interface",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ipAddress := args[0]

		if ipInterface == "" {
			return fmt.Errorf("--interface is required")
		}

		namespaceManager := netns.NewManager()
		addressManager := netns.NewAddressManager(namespaceManager)

		// Delete from system
		if err := addressManager.Delete(ipAddress, ipInterface, ipNs); err != nil {
			return err
		}

		fmt.Printf("Removed %s from %s\n", ipAddress, ipInterface)
		return nil
	},
}

var ipListCmd = &cobra.Command{
	Use:   "list",
	Short: "List IP addresses",
	RunE: func(cmd *cobra.Command, args []string) error {
		namespaceManager := netns.NewManager()
		addressManager := netns.NewAddressManager(namespaceManager)

		addressInfos, err := addressManager.GetAddressInfos(ipNs)
		if err != nil {
			return err
		}

		if len(addressInfos) == 0 {
			fmt.Println("No IP addresses found")
			return nil
		}

		tableWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tableWriter, "INTERFACE\tADDRESS\tFAMILY\tSCOPE")

		for _, addressInfo := range addressInfos {
			fmt.Fprintf(tableWriter, "%s\t%s\t%s\t%s\n",
				addressInfo.Interface,
				addressInfo.Address,
				addressInfo.Family,
				addressInfo.Scope,
			)
		}

		tableWriter.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(ipCmd)

	ipAddCmd.Flags().StringVar(&ipInterface, "interface", "", "interface name (required)")
	ipAddCmd.Flags().StringVar(&ipNs, "ns", "", "namespace")

	ipDeleteCmd.Flags().StringVar(&ipInterface, "interface", "", "interface name (required)")
	ipDeleteCmd.Flags().StringVar(&ipNs, "ns", "", "namespace")

	ipListCmd.Flags().StringVar(&ipNs, "ns", "", "namespace (list all if not specified)")

	ipCmd.AddCommand(ipAddCmd)
	ipCmd.AddCommand(ipDeleteCmd)
	ipCmd.AddCommand(ipListCmd)
}
