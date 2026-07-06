package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tenxprotocols/ai-cli/internal/config"
	"github.com/tenxprotocols/ai-cli/internal/providers"
)

func newInitCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactive setup wizard",
		Long:  "Walks through provider, model, and profile setup and writes the config file. Existing config is merged, never overwritten.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, file, err := loadConfig(flags)
			if err != nil {
				return err
			}
			wizard := &wizard{
				in:  bufio.NewScanner(cmd.InOrStdin()),
				out: cmd.OutOrStdout(),
			}
			return runInit(cmd.Context(), wizard, path, file)
		},
	}
}

var builtinTypes = []string{"anthropic", "openai", "gemini", "openrouter"}

func runInit(ctx context.Context, w *wizard, path string, file config.File) error {
	status := "existing"
	if len(file.Providers) == 0 && len(file.Profiles) == 0 {
		status = "new"
	}
	fmt.Fprintf(w.out, "Config: %s (%s)\n\n", path, status)

	// Provider: built-ins annotated with detected keys, best detection preselected.
	options := make([]string, 0, len(builtinTypes)+1)
	preselect := 0
	for i, typ := range builtinTypes {
		note := ""
		if config.ResolveAPIKeyForProbe(typ, typ, "", config.OSEnv) != "" {
			note = "  (key detected)"
			if preselect == 0 {
				preselect = i + 1
			}
		}
		options = append(options, typ+note)
	}
	ollamaModel, ollamaUp := ollamaProbe()
	note := "  (not running)"
	if ollamaUp {
		note = "  (running)"
		if preselect == 0 {
			preselect = len(builtinTypes) + 1
		}
	}
	options = append(options, "ollama"+note, "custom OpenAI-compatible endpoint")
	if preselect == 0 {
		preselect = 1
	}
	choice := w.choose("Provider", options, preselect)

	name, typ, baseURL, defaultModel := "", "", "", ""
	switch {
	case choice <= len(builtinTypes):
		typ = builtinTypes[choice-1]
		name = typ
		defaultModel = config.DefaultModel(typ)
	case choice == len(builtinTypes)+1:
		name, typ, baseURL = "ollama", "openai-compat", "http://localhost:11434/v1"
		defaultModel = ollamaModel
	default:
		typ = "openai-compat"
		name = w.ask("Provider name (e.g. bifrost)", "")
		if name == "" {
			return errors.New("a custom provider needs a name")
		}
		baseURL = w.ask("Base URL (e.g. https://proxy.example.com/v1)", "")
		if baseURL == "" {
			return errors.New("a custom provider needs a base_url")
		}
	}

	// Model: offer the provider's live list when credentials allow.
	apiKey := config.ResolveAPIKeyForProbe(name, typ, file.Providers[name].APIKey, config.OSEnv)
	if models := listModelsQuietly(ctx, name, typ, apiKey, baseURL); len(models) > 0 {
		picked := w.choose("Model", modelOptions(models, defaultModel), modelPreselect(models, defaultModel))
		defaultModel = models[picked-1].ID
	} else {
		defaultModel = w.ask("Model (passed verbatim)", defaultModel)
		if defaultModel == "" {
			return errors.New("a profile needs a model")
		}
	}

	profileName := w.ask("Profile name", "default")

	// Merge and save. Existing entries are preserved.
	if file.Providers == nil {
		file.Providers = map[string]config.Provider{}
	}
	if file.Profiles == nil {
		file.Profiles = map[string]config.Profile{}
	}
	existing := file.Providers[name]
	file.Providers[name] = config.Provider{Type: typ, APIKey: existing.APIKey, BaseURL: baseURL}
	file.Profiles[profileName] = config.Profile{Provider: name, Model: defaultModel}
	if file.DefaultProfile == "" || w.ask(fmt.Sprintf("Make %q the default profile? [y/N]", profileName), "n") == "y" {
		file.DefaultProfile = profileName
	}
	if err := config.SaveFile(path, file); err != nil {
		return err
	}

	fmt.Fprintf(w.out, "\nWrote %s\n  profile %s = %s / %s\n", path, profileName, name, defaultModel)
	if apiKey == "" && typ != "openai-compat" {
		fmt.Fprintf(w.out, "\nNo API key found. Export one before use:\n  export AI_CLI_%s_API_KEY=...\n", strings.ToUpper(name))
	} else if apiKey == "" && baseURL != "" && !strings.Contains(baseURL, "localhost") {
		fmt.Fprintf(w.out, "\nIf the endpoint needs a key:\n  export AI_CLI_%s_API_KEY=...\n", strings.ToUpper(name))
	}
	fmt.Fprintf(w.out, "\nTry it:\n  ai what is the phase of the moon\n")
	return nil
}

// listModelsQuietly fetches the provider's models, returning nil on any
// failure — the wizard falls back to free-text entry. Package-level so tests
// can substitute it and stay off the network.
var listModelsQuietly = func(ctx context.Context, name, typ, apiKey, baseURL string) []providers.ModelInfo {
	if apiKey == "" && typ != "openai-compat" {
		return nil
	}
	registry := providers.NewRegistry()
	providers.RegisterBuiltins(registry)
	provider, err := registry.Get(typ, providers.Config{Name: name, APIKey: apiKey, BaseURL: baseURL})
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	models, err := provider.ListModels(ctx)
	if err != nil || len(models) == 0 {
		return nil
	}
	if len(models) > 10 {
		models = models[:10]
	}
	return models
}

func modelOptions(models []providers.ModelInfo, _ string) []string {
	options := make([]string, len(models))
	for i, m := range models {
		options[i] = m.ID
		if m.DisplayName != "" {
			options[i] += "  (" + m.DisplayName + ")"
		}
	}
	return options
}

func modelPreselect(models []providers.ModelInfo, preferred string) int {
	for i, m := range models {
		if m.ID == preferred {
			return i + 1
		}
	}
	return 1
}

// wizard reads answers line by line; empty input (or EOF) takes the default.
type wizard struct {
	in  *bufio.Scanner
	out io.Writer
}

func (w *wizard) ask(prompt, def string) string {
	if def != "" {
		fmt.Fprintf(w.out, "%s [%s]: ", prompt, def)
	} else {
		fmt.Fprintf(w.out, "%s: ", prompt)
	}
	if !w.in.Scan() {
		fmt.Fprintln(w.out)
		return def
	}
	answer := strings.TrimSpace(w.in.Text())
	if answer == "" {
		return def
	}
	return answer
}

func (w *wizard) choose(prompt string, options []string, def int) int {
	fmt.Fprintf(w.out, "%s:\n", prompt)
	for i, option := range options {
		fmt.Fprintf(w.out, "  %d) %s\n", i+1, option)
	}
	for {
		answer := w.ask("Choose", strconv.Itoa(def))
		n, err := strconv.Atoi(answer)
		if err == nil && n >= 1 && n <= len(options) {
			return n
		}
		fmt.Fprintf(w.out, "Enter a number between 1 and %d.\n", len(options))
	}
}
