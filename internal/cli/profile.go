package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/tenxprotocols/ai-cli/internal/config"
)

func newProfileCmd(gf *GlobalFlags) *cobra.Command {
	c := &cobra.Command{
		Use:   "profile",
		Short: "Manage profiles (provider + model + system prompt)",
	}
	c.AddCommand(
		newProfileListCmd(gf),
		newProfileShowCmd(gf),
		newProfileUseCmd(gf),
		newProfileCreateCmd(gf),
		newProfileRmCmd(gf),
	)
	return c
}

// loadConfig reads the config file for profile subcommands.
func loadConfig(gf *GlobalFlags) (string, config.File, error) {
	path, err := resolveConfigPath(gf.ConfigPath)
	if err != nil {
		return "", config.File{}, err
	}
	f, err := config.LoadFile(path)
	return path, f, err
}

func newProfileListCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, f, err := loadConfig(gf)
			if err != nil {
				return err
			}
			names := make([]string, 0, len(f.Profiles))
			for n := range f.Profiles {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				p := f.Profiles[n]
				marker := " "
				if n == f.DefaultProfile {
					marker = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\t%s/%s\n", marker, n, p.Provider, p.Model)
			}
			return nil
		},
	}
}

func newProfileShowCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show [name]",
		Short: "Show a profile (defaults to active)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, f, err := loadConfig(gf)
			if err != nil {
				return err
			}
			name := f.DefaultProfile
			if len(args) == 1 {
				name = args[0]
			}
			p, ok := f.Profiles[name]
			if !ok {
				return fmt.Errorf("%w: %s", config.ErrUnknownProfile, name)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "profile  %s\nprovider %s\nmodel    %s\n", name, p.Provider, p.Model)
			if p.System != "" {
				fmt.Fprintf(w, "system   %s\n", p.System)
			}
			if p.Temperature != nil {
				fmt.Fprintf(w, "temperature %v\n", *p.Temperature)
			}
			if p.MaxTokens != nil {
				fmt.Fprintf(w, "max_tokens  %d\n", *p.MaxTokens)
			}
			return nil
		},
	}
}

func newProfileUseCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the default profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, f, err := loadConfig(gf)
			if err != nil {
				return err
			}
			if _, ok := f.Profiles[args[0]]; !ok {
				return fmt.Errorf("%w: %s", config.ErrUnknownProfile, args[0])
			}
			f.DefaultProfile = args[0]
			return config.SaveFile(path, f)
		},
	}
}

func newProfileCreateCmd(gf *GlobalFlags) *cobra.Command {
	var from, provider, model, system string
	c := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, f, err := loadConfig(gf)
			if err != nil {
				return err
			}
			name := args[0]
			if _, ok := f.Profiles[name]; ok {
				return fmt.Errorf("profile %q already exists", name)
			}
			var p config.Profile
			if from != "" {
				base, ok := f.Profiles[from]
				if !ok {
					return fmt.Errorf("%w: %s", config.ErrUnknownProfile, from)
				}
				p = base
			}
			if provider != "" {
				p.Provider = provider
			}
			if model != "" {
				p.Model = model
			}
			if system != "" {
				p.System = system
			}
			if p.Provider == "" || p.Model == "" {
				return fmt.Errorf("profile needs --provider and --model (or --from)")
			}
			f.Profiles[name] = p
			return config.SaveFile(path, f)
		},
	}
	c.Flags().StringVar(&from, "from", "", "copy fields from an existing profile")
	c.Flags().StringVar(&provider, "provider", "", "provider name")
	c.Flags().StringVar(&model, "model", "", "model (verbatim)")
	c.Flags().StringVar(&system, "system", "", "system prompt")
	return c
}

func newProfileRmCmd(gf *GlobalFlags) *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, f, err := loadConfig(gf)
			if err != nil {
				return err
			}
			name := args[0]
			if _, ok := f.Profiles[name]; !ok {
				return fmt.Errorf("%w: %s", config.ErrUnknownProfile, name)
			}
			if name == f.DefaultProfile && !force {
				return fmt.Errorf("%q is the default profile; use --force", name)
			}
			delete(f.Profiles, name)
			return config.SaveFile(path, f)
		},
	}
	c.Flags().BoolVar(&force, "force", false, "remove even if it is the default profile")
	return c
}
