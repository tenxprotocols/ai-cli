package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tenxprotocols/ai-cli/internal/output"
	"github.com/tenxprotocols/ai-cli/internal/providers"
)

func newAskCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "ask [prompt words...]",
		Short: "Ask the model a single question",
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := strings.Join(args, " ")
			if piped, ok := readStdinIfPiped(); ok {
				prompt = strings.TrimSpace(piped + "\n\n" + prompt)
			}
			if prompt == "" {
				return errors.New("empty prompt: pass words or pipe stdin")
			}
			return runPrompt(cmd, flags, prompt)
		},
	}
}

// runPrompt sends a single user prompt and renders the response in the
// configured format, streaming unless disabled.
func runPrompt(cmd *cobra.Command, flags *GlobalFlags, prompt string) error {
	format, err := output.ParseFormat(flags.Format)
	if err != nil {
		return err
	}
	resolved, err := resolveForCall(cmd.Name(), flags)
	if err != nil {
		return err
	}
	provider, err := buildProvider(cmd.Context(), resolved)
	if err != nil {
		return err
	}

	request := providers.Request{
		Model:       resolved.Model,
		System:      resolved.System,
		Temperature: resolved.Temperature,
		MaxTokens:   resolved.MaxTokens,
		Messages:    userMessage(prompt),
	}
	out := cmd.OutOrStdout()

	if flags.NoStream || format == output.FormatJSON {
		response, err := provider.Complete(cmd.Context(), request)
		if err != nil {
			return err
		}
		return output.Render(format, out, output.FromResponse(response))
	}
	request.Stream = true
	chunks, err := provider.Stream(cmd.Context(), request)
	if err != nil {
		return err
	}
	return output.Render(format, out, chunks)
}
