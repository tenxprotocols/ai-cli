package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/tenxprotocols/ai-cli/internal/cli"
)

func main() {
	root := cli.NewRoot()

	known := map[string]bool{}
	for _, sub := range root.Commands() {
		known[sub.Name()] = true
	}

	resolution := cli.ResolveArgs(os.Args[0], os.Args, known, cli.DefaultPluginLookup)
	if resolution.Kind == cli.ResolvePlugin {
		if err := cli.Exec(resolution.PluginPath, resolution.Args); err != nil {
			fmt.Fprintf(os.Stderr, "ai: exec %s: %v\n", resolution.PluginPath, err)
			os.Exit(cli.ExitAPIError)
		}
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	root.SetArgs(resolution.Args)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ai: %v\n", err)
		os.Exit(cli.ExitCode(err))
	}
}
