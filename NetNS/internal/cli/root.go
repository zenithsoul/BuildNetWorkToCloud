package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zenith/netns-mgr/internal/db"
)

var (
	dbPath string
	DB     *db.DB
	Repo   *db.Repository
)

var rootCmd = &cobra.Command{
	Use:   "netns-mgr",
	Short: "Network namespace manager",
	Long: `A CLI tool for managing Linux network namespaces with persistence.

Supports creating and managing:
  - Network namespaces
  - Virtual ethernet (veth) pairs
  - IP addresses
  - Routes
  - Bridges
  - GRE tunnels (for peering namespaces)

All operations are persisted to a SQLite database.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip DB initialization for help commands
		if cmd.Name() == "help" || cmd.Name() == "version" {
			return nil
		}

		var err error
		DB, err = db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		Repo = db.NewRepository(DB)
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if DB != nil {
			DB.Close()
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "database path (default: ~/.netns-mgr/netns.db)")
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
