//go:build !linux

package netns

import "errors"

var errNotLinux = errors.New("network namespaces are only supported on Linux")

// mountBind is not supported on non-Linux platforms
func mountBind(source, target string) error {
	return errNotLinux
}

// unmount is not supported on non-Linux platforms
func unmount(target string) error {
	return errNotLinux
}
