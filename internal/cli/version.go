package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/tenxprotocols/ai-cli/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "ai %s (%s/%s, %s)\n",
				version.Version, runtime.GOOS, runtime.GOARCH, runtime.Version())
			return nil
		},
	}
}
