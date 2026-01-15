package cli

import (
	"fmt"
	"os"
	"os/exec"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zenith/netns-mgr/internal/netns"
)

var nsCmd = &cobra.Command{
	Use:     "ns",
	Aliases: []string{"namespace"},
	Short:   "Manage network namespaces",
}

var nsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new network namespace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		namespaceName := args[0]
		namespaceManager := netns.NewManager()

		// Create in system
		if err := namespaceManager.Create(namespaceName); err != nil {
			return err
		}

		// Record in database
		_, err := Repo.CreateNamespace(namespaceName, "")
		if err != nil {
			// Rollback system change
			namespaceManager.Delete(namespaceName)
			return fmt.Errorf("failed to record namespace: %w", err)
		}

		fmt.Printf("Created namespace: %s\n", namespaceName)
		return nil
	},
}

var nsDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a network namespace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		namespaceName := args[0]
		namespaceManager := netns.NewManager()

		// Delete from system
		if err := namespaceManager.Delete(namespaceName); err != nil {
			return err
		}

		// Remove from database
		if err := Repo.DeleteNamespace(namespaceName); err != nil {
			// Log but don't fail - system change already made
			fmt.Fprintf(os.Stderr, "Warning: failed to remove from database: %v\n", err)
		}

		fmt.Printf("Deleted namespace: %s\n", namespaceName)
		return nil
	},
}

var nsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all network namespaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		namespaceManager := netns.NewManager()

		// Get from system
		systemNamespaces, err := namespaceManager.List()
		if err != nil {
			return err
		}

		// Get from database
		databaseNamespaces, err := Repo.ListNamespaces()
		if err != nil {
			return err
		}

		// Create a map for quick lookup
		databaseNamespaceMap := make(map[string]bool)
		for _, namespaceRecord := range databaseNamespaces {
			databaseNamespaceMap[namespaceRecord.Name] = true
		}

		tableWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tableWriter, "NAME\tSTATUS\tCREATED")

		for _, namespaceName := range systemNamespaces {
			status := "active"
			createdAt := "-"

			// Find in DB for creation time
			for _, namespaceRecord := range databaseNamespaces {
				if namespaceRecord.Name == namespaceName {
					createdAt = namespaceRecord.CreatedAt.Format("2006-01-02 15:04:05")
					break
				}
			}

			if !databaseNamespaceMap[namespaceName] {
				status = "untracked"
			}

			fmt.Fprintf(tableWriter, "%s\t%s\t%s\n", namespaceName, status, createdAt)
		}

		// Show DB entries that don't exist in system
		for _, namespaceRecord := range databaseNamespaces {
			foundInSystem := false
			for _, namespaceName := range systemNamespaces {
				if namespaceRecord.Name == namespaceName {
					foundInSystem = true
					break
				}
			}
			if !foundInSystem {
				fmt.Fprintf(tableWriter, "%s\t%s\t%s\n", namespaceRecord.Name, "orphaned", namespaceRecord.CreatedAt.Format("2006-01-02 15:04:05"))
			}
		}

		tableWriter.Flush()
		return nil
	},
}

var nsExecCmd = &cobra.Command{
	Use:   "exec <namespace> -- <command> [args...]",
	Short: "Execute a command in a namespace",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		namespaceName := args[0]

		// Find command start (after --)
		commandStartIndex := 1
		for argIndex, arg := range args {
			if arg == "--" {
				commandStartIndex = argIndex + 1
				break
			}
		}

		if commandStartIndex >= len(args) {
			return fmt.Errorf("no command specified")
		}

		commandArgs := args[commandStartIndex:]

		// Use ip netns exec for simplicity
		execArgs := append([]string{"netns", "exec", namespaceName}, commandArgs...)
		execCommand := exec.Command("ip", execArgs...)
		execCommand.Stdin = os.Stdin
		execCommand.Stdout = os.Stdout
		execCommand.Stderr = os.Stderr

		return execCommand.Run()
	},
}

func init() {
	rootCmd.AddCommand(nsCmd)
	nsCmd.AddCommand(nsCreateCmd)
	nsCmd.AddCommand(nsDeleteCmd)
	nsCmd.AddCommand(nsListCmd)
	nsCmd.AddCommand(nsExecCmd)
}
