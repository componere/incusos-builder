package cli

import (
	"runtime/debug"
	"strings"
)

const (
	// incusOSModule is the module path whose version --version reports.
	incusOSModule = "github.com/lxc/incus-os/incus-osd"
	// unknownPin is rendered when the module is absent from build info.
	unknownPin = "unknown"
)

// IncusOSPin returns the linked github.com/lxc/incus-os/incus-osd module
// version from the running binary's build info.
//
// Missing build info, a missing module entry, or an empty recorded version
// render as "unknown".
func IncusOSPin() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return unknownPin
	}
	return IncusOSPinFrom(info)
}

// IncusOSPinFrom returns the incus-osd module version recorded in info.
//
// A nil info, a missing module entry, or an empty recorded version render
// as "unknown". Tests inject a fake [debug.BuildInfo]; production callers
// use [IncusOSPin], which reads the running binary.
func IncusOSPinFrom(info *debug.BuildInfo) string {
	if info == nil {
		return unknownPin
	}
	for _, dep := range info.Deps {
		if dep == nil || dep.Path != incusOSModule {
			continue
		}
		if version := strings.TrimSpace(dep.Version); version != "" {
			return version
		}
		return unknownPin
	}
	return unknownPin
}
