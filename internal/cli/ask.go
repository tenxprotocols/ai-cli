package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newAskCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ask [prompt words...]",
		Short: "Ask the model a single question",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Real implementation lands in Task 15. For now, echo so dispatch
			// tests and manual exercise work end-to-end.
			fmt.Fprintf(cmd.OutOrStdout(), "stub ask: %s\n", strings.Join(args, " "))
			return nil
		},
	}
}
