package main

import (
	"fmt"
	"reflect"
	"runtime/debug"

	apicustomizer "github.com/lxc/incus-os/incus-osd/api/customizer"
	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

func main() {
	fmt.Println("types:")
	for _, v := range []any{
		apicustomizer.ImagesPost{},
		apicustomizer.ImagesPostSeeds{},
		apiimages.Index{},
		apiimages.Update{},
		apiimages.UpdateFull{},
		apiimages.UpdateFile{},
		apiseed.Applications{},
		apiseed.Incus{},
		apiseed.Install{},
		apiseed.Kernel{},
		apiseed.MigrationManager{},
		apiseed.Network{},
		apiseed.OperationsCenter{},
		apiseed.Provider{},
		apiseed.Security{},
		apiseed.Services{},
		apiseed.Update{},
	} {
		fmt.Println(" ", reflect.TypeOf(v).String())
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("buildinfo: unavailable")
		return
	}
	fmt.Println("go:", info.GoVersion)
	fmt.Println("path:", info.Path)
	for _, m := range info.Deps {
		if m.Path == "github.com/lxc/incus-os/incus-osd" {
			fmt.Printf("incus-osd: version=%s sum=%s\n", m.Version, m.Sum)
		}
	}
}
