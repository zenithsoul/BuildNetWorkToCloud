package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zenith/netns-mgr/internal/netns"
)

var (
	greNs       string
	greLocalIP  string
	greRemoteIP string
	greKey      uint32
	greTTL      uint8
)

var greCmd = &cobra.Command{
	Use:   "gre",
	Short: "Manage GRE tunnels",
	Long: `Manage GRE (Generic Routing Encapsulation) tunnels.

GRE tunnels allow point-to-point connections between network namespaces
or hosts, encapsulating packets inside GRE headers.`,
}

var greCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a GRE tunnel",
	Long: `Create a GRE tunnel interface.

Examples:
  # Create a basic GRE tunnel in host namespace
  netns-mgr gre create gre1 --local 10.0.0.1 --remote 10.0.0.2

  # Create a GRE tunnel in a namespace with a key
  netns-mgr gre create gre1 --local 10.0.0.1 --remote 10.0.0.2 --ns myns --key 100

  # Create a GRE tunnel with custom TTL
  netns-mgr gre create gre1 --local 10.0.0.1 --remote 10.0.0.2 --ttl 64`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tunnelName := args[0]

		if greLocalIP == "" || greRemoteIP == "" {
			return fmt.Errorf("--local and --remote flags are required")
		}

		namespaceManager := netns.NewManager()
		greManager := netns.NewGREManager(namespaceManager)

		// Create GRE tunnel with options
		tunnelConfig := netns.GRETunnel{
			Name:      tunnelName,
			LocalIP:   greLocalIP,
			RemoteIP:  greRemoteIP,
			Key:       greKey,
			TTL:       greTTL,
			Namespace: greNs,
		}

		if err := greManager.CreateWithOptions(tunnelConfig); err != nil {
			return err
		}

		// Get namespace ID for DB
		var namespaceID *int64
		if greNs != "" {
			namespaceRecord, err := Repo.GetNamespaceByName(greNs)
			if err == nil && namespaceRecord != nil {
				namespaceID = &namespaceRecord.ID
			}
		}

		// Record in database
		_, err := Repo.CreateGRETunnel(tunnelName, greLocalIP, greRemoteIP, greKey, greTTL, namespaceID)
		if err != nil {
			// Rollback system change
			greManager.Delete(tunnelName, greNs)
			return fmt.Errorf("failed to record GRE tunnel: %w", err)
		}

		fmt.Printf("Created GRE tunnel: %s (local=%s, remote=%s)\n", tunnelName, greLocalIP, greRemoteIP)
		return nil
	},
}

var greDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a GRE tunnel",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tunnelName := args[0]

		namespaceManager := netns.NewManager()
		greManager := netns.NewGREManager(namespaceManager)

		// Delete from system
		if err := greManager.Delete(tunnelName, greNs); err != nil {
			return err
		}

		// Remove from database
		if err := Repo.DeleteGRETunnel(tunnelName); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove from database: %v\n", err)
		}

		fmt.Printf("Deleted GRE tunnel: %s\n", tunnelName)
		return nil
	},
}

var greListCmd = &cobra.Command{
	Use:   "list",
	Short: "List GRE tunnels",
	RunE: func(cmd *cobra.Command, args []string) error {
		namespaceManager := netns.NewManager()
		greManager := netns.NewGREManager(namespaceManager)

		greTunnels, err := greManager.List(greNs)
		if err != nil {
			return err
		}

		if len(greTunnels) == 0 {
			fmt.Println("No GRE tunnels found")
			return nil
		}

		tableWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tableWriter, "NAME\tLOCAL\tREMOTE\tKEY\tTTL\tSTATE")

		for _, tunnelInfo := range greTunnels {
			keyDisplay := "-"
			if tunnelInfo.Key > 0 {
				keyDisplay = fmt.Sprintf("%d", tunnelInfo.Key)
			}

			ttlDisplay := "inherit"
			if tunnelInfo.TTL > 0 {
				ttlDisplay = fmt.Sprintf("%d", tunnelInfo.TTL)
			}

			fmt.Fprintf(tableWriter, "%s\t%s\t%s\t%s\t%s\t%s\n",
				tunnelInfo.Name,
				tunnelInfo.LocalIP,
				tunnelInfo.RemoteIP,
				keyDisplay,
				ttlDisplay,
				tunnelInfo.State,
			)
		}

		tableWriter.Flush()
		return nil
	},
}

var greUpCmd = &cobra.Command{
	Use:   "up <name>",
	Short: "Bring a GRE tunnel interface up",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tunnelName := args[0]

		namespaceManager := netns.NewManager()
		greManager := netns.NewGREManager(namespaceManager)

		if err := greManager.SetUp(tunnelName, greNs); err != nil {
			return err
		}

		fmt.Printf("GRE tunnel %s is now up\n", tunnelName)
		return nil
	},
}

var greDownCmd = &cobra.Command{
	Use:   "down <name>",
	Short: "Bring a GRE tunnel interface down",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tunnelName := args[0]

		namespaceManager := netns.NewManager()
		greManager := netns.NewGREManager(namespaceManager)

		if err := greManager.SetDown(tunnelName, greNs); err != nil {
			return err
		}

		fmt.Printf("GRE tunnel %s is now down\n", tunnelName)
		return nil
	},
}

var grePeerNs1    string
var grePeerNs1IP  string
var grePeerNs1TIP string
var grePeerNs2    string
var grePeerNs2IP  string
var grePeerNs2TIP string

var grePeerCmd = &cobra.Command{
	Use:   "peer <tunnel-name>",
	Short: "Create bidirectional GRE tunnels between two namespaces",
	Long: `Create a GRE tunnel pair between two namespaces.

This creates GRE tunnels in both namespaces, allowing them to communicate
through the tunnel interfaces.

Examples:
  # Peer ns1 and ns2 with GRE tunnels
  netns-mgr gre peer mytunnel \
    --ns1 ns1 --ns1-ip 10.0.0.1 --ns1-tunnel-ip 192.168.1.1/30 \
    --ns2 ns2 --ns2-ip 10.0.0.2 --ns2-tunnel-ip 192.168.1.2/30`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tunnelName := args[0]

		// Validate required flags
		if grePeerNs1 == "" || grePeerNs2 == "" {
			return fmt.Errorf("--ns1 and --ns2 flags are required")
		}
		if grePeerNs1IP == "" || grePeerNs2IP == "" {
			return fmt.Errorf("--ns1-ip and --ns2-ip flags are required")
		}
		if grePeerNs1TIP == "" || grePeerNs2TIP == "" {
			return fmt.Errorf("--ns1-tunnel-ip and --ns2-tunnel-ip flags are required")
		}

		namespaceManager := netns.NewManager()
		greManager := netns.NewGREManager(namespaceManager)

		// Create peer tunnels
		err := greManager.CreatePeerTunnels(
			grePeerNs1, grePeerNs1IP, grePeerNs1TIP,
			grePeerNs2, grePeerNs2IP, grePeerNs2TIP,
			tunnelName,
		)
		if err != nil {
			return err
		}

		// Record in database
		tunnel1Name := tunnelName + "-1"
		tunnel2Name := tunnelName + "-2"

		// Get namespace IDs
		namespace1Record, _ := Repo.GetNamespaceByName(grePeerNs1)
		namespace2Record, _ := Repo.GetNamespaceByName(grePeerNs2)

		var namespace1ID, namespace2ID *int64
		if namespace1Record != nil {
			namespace1ID = &namespace1Record.ID
		}
		if namespace2Record != nil {
			namespace2ID = &namespace2Record.ID
		}

		// Record tunnels
		Repo.CreateGRETunnel(tunnel1Name, grePeerNs1IP, grePeerNs2IP, 0, 0, namespace1ID)
		Repo.CreateGRETunnel(tunnel2Name, grePeerNs2IP, grePeerNs1IP, 0, 0, namespace2ID)

		fmt.Printf("Created GRE tunnel pair:\n")
		fmt.Printf("  %s in %s (local=%s, remote=%s, tunnel IP=%s)\n", tunnel1Name, grePeerNs1, grePeerNs1IP, grePeerNs2IP, grePeerNs1TIP)
		fmt.Printf("  %s in %s (local=%s, remote=%s, tunnel IP=%s)\n", tunnel2Name, grePeerNs2, grePeerNs2IP, grePeerNs1IP, grePeerNs2TIP)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(greCmd)

	// Create command flags
	greCreateCmd.Flags().StringVar(&greNs, "ns", "", "namespace to create tunnel in")
	greCreateCmd.Flags().StringVar(&greLocalIP, "local", "", "local endpoint IP address (required)")
	greCreateCmd.Flags().StringVar(&greRemoteIP, "remote", "", "remote endpoint IP address (required)")
	greCreateCmd.Flags().Uint32Var(&greKey, "key", 0, "GRE key for multiplexing (0 = no key)")
	greCreateCmd.Flags().Uint8Var(&greTTL, "ttl", 0, "time to live (0 = inherit)")

	// Delete command flags
	greDeleteCmd.Flags().StringVar(&greNs, "ns", "", "namespace")

	// List command flags
	greListCmd.Flags().StringVar(&greNs, "ns", "", "namespace")

	// Up/down command flags
	greUpCmd.Flags().StringVar(&greNs, "ns", "", "namespace")
	greDownCmd.Flags().StringVar(&greNs, "ns", "", "namespace")

	// Peer command flags
	grePeerCmd.Flags().StringVar(&grePeerNs1, "ns1", "", "first namespace name (required)")
	grePeerCmd.Flags().StringVar(&grePeerNs1IP, "ns1-ip", "", "IP address in ns1 for tunnel endpoint (required)")
	grePeerCmd.Flags().StringVar(&grePeerNs1TIP, "ns1-tunnel-ip", "", "IP address to assign to tunnel interface in ns1 (required)")
	grePeerCmd.Flags().StringVar(&grePeerNs2, "ns2", "", "second namespace name (required)")
	grePeerCmd.Flags().StringVar(&grePeerNs2IP, "ns2-ip", "", "IP address in ns2 for tunnel endpoint (required)")
	grePeerCmd.Flags().StringVar(&grePeerNs2TIP, "ns2-tunnel-ip", "", "IP address to assign to tunnel interface in ns2 (required)")

	// Add subcommands
	greCmd.AddCommand(greCreateCmd)
	greCmd.AddCommand(greDeleteCmd)
	greCmd.AddCommand(greListCmd)
	greCmd.AddCommand(greUpCmd)
	greCmd.AddCommand(greDownCmd)
	greCmd.AddCommand(grePeerCmd)
}
