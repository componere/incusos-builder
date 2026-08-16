package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/componere/incusos-builder/internal/cli"
)

//nolint:gochecknoglobals // GoReleaser injects these values with ldflags during releases.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// main starts the incusos-builder process.
func main() {
	os.Exit(run())
}

// run constructs process Options and returns the command exit code.
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return cli.Execute(ctx, cli.Options{
		In: os.Stdin,
		Build: cli.BuildInfo{
			Version: version,
			Commit:  commit,
			Date:    date,
		},
		Out:        os.Stdout,
		Err:        os.Stderr,
		IncusOSPin: cli.IncusOSPin(),
	})
}
