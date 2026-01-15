//go:build linux

package netns

import (
	"syscall"
)

// mountBind creates a bind mount from source to target
func mountBind(source, target string) error {
	return syscall.Mount(source, target, "none", syscall.MS_BIND, "")
}

// unmount removes a mount point
func unmount(target string) error {
	return syscall.Unmount(target, syscall.MNT_DETACH)
}
