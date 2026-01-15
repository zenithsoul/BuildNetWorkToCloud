package netns

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

const netnsPath = "/var/run/netns"

// Manager handles network namespace operations
type Manager struct{}

// NewManager creates a new namespace manager
func NewManager() *Manager {
	return &Manager{}
}

// Create creates a new network namespace
// Parameters:
//   - namespaceName: name of the namespace to create
func (namespaceManager *Manager) Create(namespaceName string) error {
	// Lock OS thread for namespace operations
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save current namespace
	originalNamespace, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get current namespace: %w", err)
	}
	defer originalNamespace.Close()

	// Ensure netns directory exists
	if err := os.MkdirAll(netnsPath, 0755); err != nil {
		return fmt.Errorf("failed to create netns directory: %w", err)
	}

	// Create new namespace
	newNamespace, err := netns.New()
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	// Bind mount the namespace to persist it
	namespacePath := filepath.Join(netnsPath, namespaceName)
	namespaceFile, err := os.Create(namespacePath)
	if err != nil {
		newNamespace.Close()
		netns.Set(originalNamespace)
		return fmt.Errorf("failed to create namespace file: %w", err)
	}
	namespaceFile.Close()

	// Get the namespace fd path
	namespaceFdPath := fmt.Sprintf("/proc/self/fd/%d", int(newNamespace))

	// Bind mount
	if err := mountBind(namespaceFdPath, namespacePath); err != nil {
		os.Remove(namespacePath)
		newNamespace.Close()
		netns.Set(originalNamespace)
		return fmt.Errorf("failed to bind mount namespace: %w", err)
	}

	// Bring up loopback in new namespace
	loopbackInterface, err := netlink.LinkByName("lo")
	if err == nil {
		netlink.LinkSetUp(loopbackInterface)
	}

	// Return to original namespace
	newNamespace.Close()
	if err := netns.Set(originalNamespace); err != nil {
		return fmt.Errorf("failed to restore original namespace: %w", err)
	}

	return nil
}

// Delete removes a network namespace
// Parameters:
//   - namespaceName: name of the namespace to delete
func (namespaceManager *Manager) Delete(namespaceName string) error {
	namespacePath := filepath.Join(netnsPath, namespaceName)

	// Check if namespace exists
	if _, err := os.Stat(namespacePath); os.IsNotExist(err) {
		return fmt.Errorf("namespace %q does not exist", namespaceName)
	}

	// Unmount the namespace
	if err := unmount(namespacePath); err != nil {
		return fmt.Errorf("failed to unmount namespace: %w", err)
	}

	// Remove the file
	if err := os.Remove(namespacePath); err != nil {
		return fmt.Errorf("failed to remove namespace file: %w", err)
	}

	return nil
}

// List returns all network namespaces
func (namespaceManager *Manager) List() ([]string, error) {
	directoryEntries, err := os.ReadDir(netnsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read netns directory: %w", err)
	}

	var namespaceNames []string
	for _, directoryEntry := range directoryEntries {
		if !directoryEntry.IsDir() {
			namespaceNames = append(namespaceNames, directoryEntry.Name())
		}
	}
	return namespaceNames, nil
}

// Exists checks if a namespace exists
// Parameters:
//   - namespaceName: name of the namespace to check
func (namespaceManager *Manager) Exists(namespaceName string) bool {
	namespacePath := filepath.Join(netnsPath, namespaceName)
	_, err := os.Stat(namespacePath)
	return err == nil
}

// GetHandle returns a netns handle for the given namespace
// Parameters:
//   - namespaceName: name of the namespace
func (namespaceManager *Manager) GetHandle(namespaceName string) (netns.NsHandle, error) {
	namespacePath := filepath.Join(netnsPath, namespaceName)
	return netns.GetFromPath(namespacePath)
}

// RunInNamespace executes a function in the specified namespace
// Parameters:
//   - namespaceName: name of the namespace to execute in
//   - functionToExecute: function to execute in the namespace
func (namespaceManager *Manager) RunInNamespace(namespaceName string, functionToExecute func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save current namespace
	originalNamespace, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get current namespace: %w", err)
	}
	defer originalNamespace.Close()

	// Get target namespace
	targetNamespace, err := namespaceManager.GetHandle(namespaceName)
	if err != nil {
		return fmt.Errorf("failed to get namespace %q: %w", namespaceName, err)
	}
	defer targetNamespace.Close()

	// Switch to target namespace
	if err := netns.Set(targetNamespace); err != nil {
		return fmt.Errorf("failed to enter namespace: %w", err)
	}

	// Execute the function
	executionError := functionToExecute()

	// Return to original namespace
	if err := netns.Set(originalNamespace); err != nil {
		return fmt.Errorf("failed to restore namespace: %w", err)
	}

	return executionError
}

// GetNetlinkHandle returns a netlink handle for operations in the namespace
// Parameters:
//   - namespaceName: name of the namespace
func (namespaceManager *Manager) GetNetlinkHandle(namespaceName string) (*netlink.Handle, error) {
	namespaceHandle, err := namespaceManager.GetHandle(namespaceName)
	if err != nil {
		return nil, err
	}

	netlinkHandle, err := netlink.NewHandleAt(namespaceHandle)
	if err != nil {
		namespaceHandle.Close()
		return nil, fmt.Errorf("failed to create netlink handle: %w", err)
	}

	return netlinkHandle, nil
}
