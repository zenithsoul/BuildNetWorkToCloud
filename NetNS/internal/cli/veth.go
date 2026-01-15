package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zenith/netns-mgr/internal/netns"
)

var (
	vethPeer   string
	vethNs     string
	vethPeerNs string
)

var vethCmd = &cobra.Command{
	Use:   "veth",
	Short: "Manage virtual ethernet pairs",
}

var vethCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a veth pair",
	Long: `Create a virtual ethernet pair.

Examples:
  # Create veth pair in host namespace
  netns-mgr veth create veth0 --peer veth1

  # Create veth pair with one end in a namespace
  netns-mgr veth create veth0 --peer veth1 --ns myns

  # Create veth pair connecting two namespaces
  netns-mgr veth create veth0 --peer veth1 --ns ns1 --peer-ns ns2`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		interfaceName := args[0]

		if vethPeer == "" {
			return fmt.Errorf("--peer is required")
		}

		namespaceManager := netns.NewManager()
		vethManager := netns.NewVethManager(namespaceManager)

		// Create veth pair
		if err := vethManager.Create(interfaceName, vethPeer, vethNs, vethPeerNs); err != nil {
			return err
		}

		// Get namespace IDs for DB
		var namespaceID, peerNamespaceID *int64

		if vethNs != "" {
			namespaceRecord, err := Repo.GetNamespaceByName(vethNs)
			if err == nil && namespaceRecord != nil {
				namespaceID = &namespaceRecord.ID
			}
		}

		if vethPeerNs != "" {
			namespaceRecord, err := Repo.GetNamespaceByName(vethPeerNs)
			if err == nil && namespaceRecord != nil {
				peerNamespaceID = &namespaceRecord.ID
			}
		}

		// Record in database
		_, err := Repo.CreateVethPair(interfaceName, vethPeer, namespaceID, peerNamespaceID)
		if err != nil {
			// Rollback system change
			vethManager.Delete(interfaceName)
			return fmt.Errorf("failed to record veth pair: %w", err)
		}

		fmt.Printf("Created veth pair: %s <-> %s\n", interfaceName, vethPeer)
		return nil
	},
}

var vethDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a veth pair",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		interfaceName := args[0]

		namespaceManager := netns.NewManager()
		vethManager := netns.NewVethManager(namespaceManager)

		// Delete from system
		if err := vethManager.Delete(interfaceName); err != nil {
			return err
		}

		// Remove from database
		if err := Repo.DeleteVethPair(interfaceName); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove from database: %v\n", err)
		}

		fmt.Printf("Deleted veth pair: %s\n", interfaceName)
		return nil
	},
}

var vethListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all veth pairs",
	RunE: func(cmd *cobra.Command, args []string) error {
		vethPairs, err := Repo.ListVethPairs()
		if err != nil {
			return err
		}

		if len(vethPairs) == 0 {
			fmt.Println("No veth pairs found")
			return nil
		}

		tableWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tableWriter, "NAME\tPEER\tNAMESPACE\tPEER NAMESPACE\tCREATED")

		for _, vethPair := range vethPairs {
			namespaceName := "-"
			peerNamespaceName := "-"

			if vethPair.NsID != nil {
				namespaceRecord, _ := Repo.GetNamespace(*vethPair.NsID)
				if namespaceRecord != nil {
					namespaceName = namespaceRecord.Name
				}
			}

			if vethPair.PeerNsID != nil {
				namespaceRecord, _ := Repo.GetNamespace(*vethPair.PeerNsID)
				if namespaceRecord != nil {
					peerNamespaceName = namespaceRecord.Name
				}
			}

			fmt.Fprintf(tableWriter, "%s\t%s\t%s\t%s\t%s\n",
				vethPair.Name,
				vethPair.PeerName,
				namespaceName,
				peerNamespaceName,
				vethPair.CreatedAt.Format("2006-01-02 15:04:05"),
			)
		}

		tableWriter.Flush()
		return nil
	},
}

var vethUpCmd = &cobra.Command{
	Use:   "up <name>",
	Short: "Bring a veth interface up",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		interfaceName := args[0]

		namespaceManager := netns.NewManager()
		vethManager := netns.NewVethManager(namespaceManager)

		if err := vethManager.SetUp(interfaceName, vethNs); err != nil {
			return err
		}

		fmt.Printf("Interface %s is now up\n", interfaceName)
		return nil
	},
}

var vethDownCmd = &cobra.Command{
	Use:   "down <name>",
	Short: "Bring a veth interface down",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		interfaceName := args[0]

		namespaceManager := netns.NewManager()
		vethManager := netns.NewVethManager(namespaceManager)

		if err := vethManager.SetDown(interfaceName, vethNs); err != nil {
			return err
		}

		fmt.Printf("Interface %s is now down\n", interfaceName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(vethCmd)

	vethCreateCmd.Flags().StringVar(&vethPeer, "peer", "", "peer interface name (required)")
	vethCreateCmd.Flags().StringVar(&vethNs, "ns", "", "namespace for the interface")
	vethCreateCmd.Flags().StringVar(&vethPeerNs, "peer-ns", "", "namespace for the peer interface")

	vethUpCmd.Flags().StringVar(&vethNs, "ns", "", "namespace of the interface")
	vethDownCmd.Flags().StringVar(&vethNs, "ns", "", "namespace of the interface")

	vethCmd.AddCommand(vethCreateCmd)
	vethCmd.AddCommand(vethDeleteCmd)
	vethCmd.AddCommand(vethListCmd)
	vethCmd.AddCommand(vethUpCmd)
	vethCmd.AddCommand(vethDownCmd)
}
