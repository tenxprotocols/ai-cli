package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tenxprotocols/ai-cli/internal/output"
	"github.com/tenxprotocols/ai-cli/internal/providers"
)

func newAskCmd(gf *GlobalFlags) *cobra.Command {
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
			return runPrompt(cmd, gf, prompt)
		},
	}
}

// runPrompt sends a single user prompt and renders the response in the
// configured format, streaming unless disabled.
func runPrompt(cmd *cobra.Command, gf *GlobalFlags, prompt string) error {
	f, err := output.ParseFormat(gf.Format)
	if err != nil {
		return err
	}
	r, err := resolveForCall(gf)
	if err != nil {
		return err
	}
	p, err := buildProvider(cmd.Context(), r)
	if err != nil {
		return err
	}

	req := providers.Request{
		Model:       r.Model,
		System:      r.System,
		Temperature: r.Temperature,
		MaxTokens:   r.MaxTokens,
		Messages:    userMessage(prompt),
	}
	w := cmd.OutOrStdout()

	if gf.NoStream || f == output.FormatJSON {
		resp, err := p.Complete(cmd.Context(), req)
		if err != nil {
			return err
		}
		return output.Render(f, w, output.FromResponse(resp))
	}
	req.Stream = true
	ch, err := p.Stream(cmd.Context(), req)
	if err != nil {
		return err
	}
	return output.Render(f, w, ch)
}
