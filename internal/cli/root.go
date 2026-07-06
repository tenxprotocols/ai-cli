package cli

import (
	"os"

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
	persistent.StringVar(&flags.Provider, "provider", "", "override provider for this call (env: AI_CLI_PROVIDER)")
	persistent.StringVar(&flags.Model, "model", "", "override model, passed verbatim to the provider (env: AI_CLI_MODEL)")
	persistent.StringVar(&flags.Format, "format", envDefault("AI_CLI_FORMAT", "text"), "output format: text|json|jsonl (env: AI_CLI_FORMAT)")
	persistent.BoolVar(&flags.NoStream, "no-stream", false, "disable streaming")
	persistent.StringVar(&flags.System, "system", "", "system prompt, inline (env: AI_CLI_SYSTEM)")
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

// envDefault returns the env var's value if set and non-empty, else fallback.
func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
