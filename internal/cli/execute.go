package cli

import (
	"context"
	"fmt"
	"os"
)

// Execute constructs the root command from options, runs it to completion,
// and returns the process exit code from ARCHITECTURE §3.
func Execute(ctx context.Context, options Options) int {
	root := NewRootCommand(options)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return exitSuccess
	}
	dest := options.Err
	if dest == nil {
		dest = os.Stderr
	}
	if _, writeErr := fmt.Fprintln(dest, err); writeErr != nil {
		return exitInternal
	}
	return exitCode(err)
}
