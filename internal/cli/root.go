package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tenxprotocols/ai-cli/internal/logging"
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
	LogLevel   string
	LogFormat  string
	LogFile    string
	LogSecrets bool
}

// NewRoot builds the top-level `ai` command.
func NewRoot() *cobra.Command {
	root, _ := newRootWithFlags()
	return root
}

// NewRootWithSink builds the root command around an existing log sink,
// adjusting it in place rather than opening a second one. main opens the sink
// before dispatch so that dispatch decisions are logged too.
func NewRootWithSink(sink *logging.Sink) *cobra.Command {
	root, _ := newRoot(sink)
	return root
}

// newRootWithFlags exposes the bound flags, for the pre-dispatch peek.
func newRootWithFlags() (*cobra.Command, *GlobalFlags) { return newRoot(nil) }

func newRoot(sink *logging.Sink) (*cobra.Command, *GlobalFlags) {
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
	persistent.StringVar(&flags.LogLevel, "log-level", envDefault("AI_CLI_LOG_LEVEL", ""),
		"log level: "+strings.Join(logging.LevelNames(), "|")+" (env: AI_CLI_LOG_LEVEL)")
	persistent.StringVar(&flags.LogFormat, "log-format", envDefault("AI_CLI_LOG_FORMAT", ""),
		"log format: "+strings.Join(logging.FormatNames(), "|")+" (env: AI_CLI_LOG_FORMAT)")
	persistent.StringVar(&flags.LogFile, "log-file", envDefault("AI_CLI_LOG_FILE", ""),
		"write logs to this file instead of stderr (env: AI_CLI_LOG_FILE)")
	persistent.BoolVar(&flags.LogSecrets, "log-secrets", false,
		"log API keys and Authorization headers unredacted")

	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		options, err := flags.logOptions()
		if err != nil {
			return err
		}
		active := sink
		switch {
		case active == nil:
			if active, err = logging.Open(options, cmd.ErrOrStderr()); err != nil {
				return err
			}
		case active.Options() != options:
			// The peek missed something — correct the sink in place so the
			// level changes without losing what is already written.
			if err := active.Reopen(options, cmd.ErrOrStderr()); err != nil {
				return err
			}
		}
		log := active.Logger()
		cmd.SetContext(logging.NewContext(cmd.Context(), log))
		log.Debug("cli: command", "name", cmd.Name(), "level", logging.LevelName(options.Level))
		return nil
	}

	root.AddCommand(newVersionCmd())
	root.AddCommand(newInitCmd(flags))
	root.AddCommand(newAskCmd(flags))
	root.AddCommand(newShellCmd(flags))
	root.AddCommand(newConfigCmd(flags))
	root.AddCommand(newModelsCmd(flags))
	root.AddCommand(newProfileCmd(flags))
	return root, flags
}

// envDefault returns the env var's value if set and non-empty, else fallback.
func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
