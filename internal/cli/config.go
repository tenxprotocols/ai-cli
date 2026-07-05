package cli

import (
	"fmt"
	"os"
	"os/exec"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/tenxprotocols/ai-cli/internal/config"
)

func newConfigCmd(gf *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage config file",
	}
	cmd.AddCommand(
		newConfigShowCmd(gf),
		newConfigPathCmd(gf),
		newConfigGetCmd(gf),
		newConfigSetCmd(gf),
		newConfigEditCmd(gf),
	)
	return cmd
}

func newConfigShowCmd(gf *GlobalFlags) *cobra.Command {
	var showSecrets bool
	c := &cobra.Command{
		Use:   "show",
		Short: "Print the resolved config (secrets redacted by default)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := resolveConfigPath(gf.ConfigPath)
			if err != nil {
				return err
			}
			f, err := config.LoadFile(p)
			if err != nil {
				return err
			}
			if !showSecrets {
				for k, v := range f.Providers {
					v.APIKey = redact(v.APIKey)
					f.Providers[k] = v
				}
			}
			b, err := toml.Marshal(f)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
	c.Flags().BoolVar(&showSecrets, "show-secrets", false, "show API keys in plaintext")
	return c
}

func newConfigPathCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := resolveConfigPath(gf.ConfigPath)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), p)
			return nil
		},
	}
}

func newConfigGetCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <dotted.key>",
		Short: "Print a single config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := resolveConfigPath(gf.ConfigPath)
			if err != nil {
				return err
			}
			f, err := config.LoadFile(p)
			if err != nil {
				return err
			}
			v, err := config.GetKey(f, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), v)
			return nil
		},
	}
}

func newConfigSetCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "set <dotted.key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			p, err := resolveConfigPath(gf.ConfigPath)
			if err != nil {
				return err
			}
			f, err := config.LoadFile(p)
			if err != nil {
				return err
			}
			if err := config.SetKey(&f, args[0], args[1]); err != nil {
				return err
			}
			return config.SaveFile(p, f)
		},
	}
}

func newConfigEditCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open config in $EDITOR",
		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := resolveConfigPath(gf.ConfigPath)
			if err != nil {
				return err
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			ec := exec.Command(editor, p)
			ec.Stdin = os.Stdin
			ec.Stdout = os.Stdout
			ec.Stderr = os.Stderr
			return ec.Run()
		},
	}
}

func resolveConfigPath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if v, ok := os.LookupEnv("AI_CLI_CONFIG"); ok && v != "" {
		return v, nil
	}
	return config.DefaultPath()
}

func redact(s string) string {
	if len(s) <= 4 {
		return "••••"
	}
	return "••••" + s[len(s)-4:]
}
