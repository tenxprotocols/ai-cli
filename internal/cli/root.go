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
	gf := &GlobalFlags{}
	root := &cobra.Command{
		Use:           "ai",
		Short:         "Talk to LLMs from the command line",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	pf := root.PersistentFlags()
	pf.StringVar(&gf.Profile, "profile", "", "profile name (env: AI_CLI_PROFILE)")
	pf.StringVar(&gf.Provider, "provider", "", "override provider for this call")
	pf.StringVar(&gf.Model, "model", "", "override model (verbatim to provider)")
	pf.StringVar(&gf.Format, "format", "text", "output format: text|json|jsonl")
	pf.BoolVar(&gf.NoStream, "no-stream", false, "disable streaming")
	pf.StringVar(&gf.System, "system", "", "system prompt (inline)")
	pf.StringVar(&gf.SystemFile, "system-file", "", "system prompt from file")
	pf.StringVar(&gf.ConfigPath, "config", "", "config file path (env: AI_CLI_CONFIG)")

	root.AddCommand(newVersionCmd())
	// Further subcommands registered in later tasks.
	return root
}
