package cli

import (
	"github.com/spf13/cobra"
)

// GlobalFlags holds flags that apply to every subcommand. Bound on the root
// command so they're inherited; individual subcommands read via Context.
type GlobalFlags struct {
	Profile    string
	Provider   string
	Model      string
	Format     string
	NoStream   bool
	System     string
	SystemFile string
	ConfigPath string
}

// NewRoot builds the top-level `ai` command.
func NewRoot() *cobra.Command {
	flags := &GlobalFlags{}
	root := &cobra.Command{
		Use:           "ai",
		Short:         "Talk to LLMs from the command line",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	persistent := root.PersistentFlags()
	persistent.StringVar(&flags.Profile, "profile", "", "profile name (env: AI_CLI_PROFILE)")
	persistent.StringVar(&flags.Provider, "provider", "", "override provider for this call")
	persistent.StringVar(&flags.Model, "model", "", "override model (verbatim to provider)")
	persistent.StringVar(&flags.Format, "format", "text", "output format: text|json|jsonl")
	persistent.BoolVar(&flags.NoStream, "no-stream", false, "disable streaming")
	persistent.StringVar(&flags.System, "system", "", "system prompt (inline)")
	persistent.StringVar(&flags.SystemFile, "system-file", "", "system prompt from file")
	persistent.StringVar(&flags.ConfigPath, "config", "", "config file path (env: AI_CLI_CONFIG)")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newAskCmd(flags))
	root.AddCommand(newShellCmd(flags))
	root.AddCommand(newConfigCmd(flags))
	root.AddCommand(newModelsCmd(flags))
	root.AddCommand(newProfileCmd(flags))
	return root
}
