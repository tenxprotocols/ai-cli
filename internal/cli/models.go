package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/tenxprotocols/ai-cli/internal/config"
	"github.com/tenxprotocols/ai-cli/internal/output"
	"github.com/tenxprotocols/ai-cli/internal/providers"
)

type modelRow struct {
	Provider    string `json:"provider"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name,omitempty"`
}

func newModelsCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List models from configured providers",
		Long:  "List models from configured providers. Filter with --provider.",
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := output.ParseFormat(gf.Format)
			if err != nil {
				return err
			}
			path, err := resolveConfigPath(gf.ConfigPath)
			if err != nil {
				return err
			}
			cfg, err := config.LoadFile(path)
			if err != nil {
				return err
			}

			names := make([]string, 0, len(cfg.Providers))
			for name := range cfg.Providers {
				if gf.Provider == "" || gf.Provider == name {
					names = append(names, name)
				}
			}
			sort.Strings(names)
			if len(names) == 0 {
				return fmt.Errorf("no matching providers configured in %s", path)
			}

			var rows []modelRow
			for _, name := range names {
				pc := cfg.Providers[name]
				p, err := buildProvider(cmd.Context(), config.Resolved{
					ProviderName: name,
					ProviderType: pc.Type,
					BaseURL:      pc.BaseURL,
					APIKey:       config.ResolveAPIKeyForProbe(name, pc.Type, pc.APIKey, config.OSEnv),
				})
				if err == nil {
					var list []providers.ModelInfo
					list, err = p.ListModels(cmd.Context())
					for _, m := range list {
						rows = append(rows, modelRow{Provider: name, ID: m.ID, DisplayName: m.DisplayName})
					}
				}
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", name, err)
				}
			}

			w := cmd.OutOrStdout()
			if f == output.FormatText {
				for _, r := range rows {
					if r.DisplayName != "" {
						fmt.Fprintf(w, "%s/%s\t%s\n", r.Provider, r.ID, r.DisplayName)
						continue
					}
					fmt.Fprintf(w, "%s/%s\n", r.Provider, r.ID)
				}
				return nil
			}
			enc := json.NewEncoder(w)
			if f == output.FormatJSON {
				return enc.Encode(rows)
			}
			for _, r := range rows {
				if err := enc.Encode(r); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
