package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/tenxprotocols/ai-cli/internal/config"
)

func newProfileCmd(flags *GlobalFlags) *cobra.Command {
	profile := &cobra.Command{
		Use:   "profile",
		Short: "Manage profiles (provider + model + system prompt)",
	}
	profile.AddCommand(
		newProfileListCmd(flags),
		newProfileShowCmd(flags),
		newProfileUseCmd(flags),
		newProfileCreateCmd(flags),
		newProfileRmCmd(flags),
	)
	return profile
}

// loadConfig reads the config file, returning its path and contents.
func loadConfig(flags *GlobalFlags) (string, config.File, error) {
	path, err := resolveConfigPath(flags.ConfigPath)
	if err != nil {
		return "", config.File{}, err
	}
	file, err := config.LoadFile(path)
	return path, file, err
}

func newProfileListCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, file, err := loadConfig(flags)
			if err != nil {
				return err
			}
			names := make([]string, 0, len(file.Profiles))
			for name := range file.Profiles {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				profile := file.Profiles[name]
				marker := " "
				if name == file.DefaultProfile {
					marker = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\t%s/%s\n", marker, name, profile.Provider, profile.Model)
			}
			return nil
		},
	}
}

func newProfileShowCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show [name]",
		Short: "Show a profile (defaults to active)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, file, err := loadConfig(flags)
			if err != nil {
				return err
			}
			name := file.DefaultProfile
			if len(args) == 1 {
				name = args[0]
			}
			profile, ok := file.Profiles[name]
			if !ok {
				return fmt.Errorf("%w: %s", config.ErrUnknownProfile, name)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "profile  %s\nprovider %s\nmodel    %s\n", name, profile.Provider, profile.Model)
			if profile.System != "" {
				fmt.Fprintf(out, "system   %s\n", profile.System)
			}
			if profile.Temperature != nil {
				fmt.Fprintf(out, "temperature %v\n", *profile.Temperature)
			}
			if profile.MaxTokens != nil {
				fmt.Fprintf(out, "max_tokens  %d\n", *profile.MaxTokens)
			}
			return nil
		},
	}
}

func newProfileUseCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the default profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			path, file, err := loadConfig(flags)
			if err != nil {
				return err
			}
			if _, ok := file.Profiles[args[0]]; !ok {
				return fmt.Errorf("%w: %s", config.ErrUnknownProfile, args[0])
			}
			file.DefaultProfile = args[0]
			return config.SaveFile(path, file)
		},
	}
}

func newProfileCreateCmd(flags *GlobalFlags) *cobra.Command {
	var from, provider, model, system string
	create := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			path, file, err := loadConfig(flags)
			if err != nil {
				return err
			}
			name := args[0]
			if _, ok := file.Profiles[name]; ok {
				return fmt.Errorf("profile %q already exists", name)
			}
			var profile config.Profile
			if from != "" {
				base, ok := file.Profiles[from]
				if !ok {
					return fmt.Errorf("%w: %s", config.ErrUnknownProfile, from)
				}
				profile = base
			}
			if provider != "" {
				profile.Provider = provider
			}
			if model != "" {
				profile.Model = model
			}
			if system != "" {
				profile.System = system
			}
			if profile.Provider == "" || profile.Model == "" {
				return fmt.Errorf("profile needs --provider and --model (or --from)")
			}
			file.Profiles[name] = profile
			return config.SaveFile(path, file)
		},
	}
	create.Flags().StringVar(&from, "from", "", "copy fields from an existing profile")
	create.Flags().StringVar(&provider, "provider", "", "provider name")
	create.Flags().StringVar(&model, "model", "", "model (verbatim)")
	create.Flags().StringVar(&system, "system", "", "system prompt")
	return create
}

func newProfileRmCmd(flags *GlobalFlags) *cobra.Command {
	var force bool
	remove := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			path, file, err := loadConfig(flags)
			if err != nil {
				return err
			}
			name := args[0]
			if _, ok := file.Profiles[name]; !ok {
				return fmt.Errorf("%w: %s", config.ErrUnknownProfile, name)
			}
			if name == file.DefaultProfile && !force {
				return fmt.Errorf("%q is the default profile; use --force", name)
			}
			delete(file.Profiles, name)
			return config.SaveFile(path, file)
		},
	}
	remove.Flags().BoolVar(&force, "force", false, "remove even if it is the default profile")
	return remove
}
