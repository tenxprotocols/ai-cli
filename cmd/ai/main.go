package main

import (
	"fmt"
	"os"

	"github.com/tenxprotocols/ai-cli/internal/cli"
)

func main() {
	root := cli.NewRoot()

	known := map[string]bool{}
	for _, c := range root.Commands() {
		known[c.Name()] = true
	}

	res := cli.ResolveArgs(os.Args[0], os.Args, known, cli.DefaultPluginLookup)
	switch res.Kind {
	case cli.ResolvePlugin:
		if err := cli.Exec(res.PluginPath, res.Args); err != nil {
			fmt.Fprintf(os.Stderr, "ai: exec %s: %v\n", res.PluginPath, err)
			os.Exit(1)
		}
		return
	default:
		root.SetArgs(res.Args)
		if err := root.Execute(); err != nil {
			fmt.Fprintf(os.Stderr, "ai: %v\n", err)
			os.Exit(1)
		}
	}
}
