package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/tenxprotocols/ai-cli/internal/cli"
	"github.com/tenxprotocols/ai-cli/internal/logging"
)

func main() {
	// Read the log flags before anything else, so dispatch and plugin hand-off
	// are logged at the level the command line asked for. Nothing is consumed:
	// the root command and any plugin still receive the original arguments.
	options, optionsErr := cli.PeekLogOptions(os.Args[1:])
	sink, err := logging.Open(options, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ai: %v\n", err)
		os.Exit(cli.ExitUsageError)
	}
	defer sink.Close()
	log := sink.Logger()
	if optionsErr != nil {
		// Cobra reports this authoritatively; note it in case it never gets there.
		log.Debug("cli: log flags fell back to defaults", "err", optionsErr)
	}

	root := cli.NewRootWithSink(sink)

	known := map[string]bool{}
	for _, sub := range root.Commands() {
		known[sub.Name()] = true
	}

	resolution := cli.ResolveArgs(os.Args[0], os.Args, known, cli.DefaultPluginLookup)
	log.Debug("cli: dispatch",
		"kind", resolution.Kind.String(), "plugin", resolution.PluginPath, "args", resolution.Args)

	if resolution.Kind == cli.ResolvePlugin {
		log.Info("plugin: exec", "path", resolution.PluginPath)
		if err := cli.Exec(resolution.PluginPath, resolution.Args, logging.PluginEnv(options)); err != nil {
			log.Error("plugin: exec failed", "path", resolution.PluginPath, "err", err)
			fmt.Fprintf(os.Stderr, "ai: exec %s: %v\n", resolution.PluginPath, err)
			os.Exit(cli.ExitAPIError)
		}
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	root.SetArgs(resolution.Args)
	if err := root.ExecuteContext(ctx); err != nil {
		code := cli.ExitCode(err)
		log.Error("run failed", "err", err, "exit", code)
		fmt.Fprintf(os.Stderr, "ai: %v\n", err)
		os.Exit(code)
	}
}
