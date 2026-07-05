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

func newModelsCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List models from configured providers",
		Long:  "List models from configured providers. Filter with --provider.",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := output.ParseFormat(flags.Format)
			if err != nil {
				return err
			}
			path, file, err := loadConfig(flags)
			if err != nil {
				return err
			}

			names := make([]string, 0, len(file.Providers))
			for name := range file.Providers {
				if flags.Provider == "" || flags.Provider == name {
					names = append(names, name)
				}
			}
			sort.Strings(names)
			if len(names) == 0 {
				return fmt.Errorf("no matching providers configured in %s", path)
			}

			var rows []modelRow
			for _, name := range names {
				providerCfg := file.Providers[name]
				provider, err := buildProvider(cmd.Context(), config.Resolved{
					ProviderName: name,
					ProviderType: providerCfg.Type,
					BaseURL:      providerCfg.BaseURL,
					APIKey:       config.ResolveAPIKeyForProbe(name, providerCfg.Type, providerCfg.APIKey, config.OSEnv),
				})
				if err == nil {
					var models []providers.ModelInfo
					models, err = provider.ListModels(cmd.Context())
					for _, model := range models {
						rows = append(rows, modelRow{Provider: name, ID: model.ID, DisplayName: model.DisplayName})
					}
				}
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", name, err)
				}
			}

			return renderModelRows(format, cmd, rows)
		},
	}
}

func renderModelRows(format output.Format, cmd *cobra.Command, rows []modelRow) error {
	out := cmd.OutOrStdout()
	switch format {
	case output.FormatText:
		for _, row := range rows {
			if row.DisplayName != "" {
				fmt.Fprintf(out, "%s/%s\t%s\n", row.Provider, row.ID, row.DisplayName)
				continue
			}
			fmt.Fprintf(out, "%s/%s\n", row.Provider, row.ID)
		}
		return nil
	case output.FormatJSON:
		return json.NewEncoder(out).Encode(rows)
	default: // jsonl
		encoder := json.NewEncoder(out)
		for _, row := range rows {
			if err := encoder.Encode(row); err != nil {
				return err
			}
		}
		return nil
	}
}
