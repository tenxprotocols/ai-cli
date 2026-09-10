package cli

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tenxprotocols/ai-cli/internal/logging"
	"github.com/tenxprotocols/ai-cli/internal/output"
	"github.com/tenxprotocols/ai-cli/internal/providers"
)

func newAskCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "ask [prompt words...]",
		Short: "Ask the model a single question",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logging.FromContext(cmd.Context())
			prompt := strings.Join(args, " ")
			piped := false
			if stdin, ok := readStdinIfPiped(); ok {
				prompt = strings.TrimSpace(stdin + "\n\n" + prompt)
				piped = true
			}
			if prompt == "" {
				return errors.New("empty prompt: pass words or pipe stdin")
			}
			log.Debug("ask: prompt", "words", len(args), "chars", len(prompt), "piped", piped)
			return runPrompt(cmd, flags, prompt)
		},
	}
}

// runPrompt sends a single user prompt and renders the response in the
// configured format, streaming unless disabled.
func runPrompt(cmd *cobra.Command, flags *GlobalFlags, prompt string) error {
	log := logging.FromContext(cmd.Context())
	format, err := output.ParseFormat(flags.Format)
	if err != nil {
		return err
	}
	resolved, err := resolveForCall(cmd.Context(), cmd.Name(), flags)
	if err != nil {
		return err
	}
	provider, err := buildProvider(cmd.Context(), resolved, flags)
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
	streaming := !flags.NoStream && format != output.FormatJSON
	log.Debug("request: prepared",
		"format", format, "stream", streaming, "messages", len(request.Messages))

	start := time.Now()
	if !streaming {
		response, err := provider.Complete(cmd.Context(), request)
		if err != nil {
			return err
		}
		logUsage(log, start, response.Usage, response.StopReason)
		return output.Render(format, out, output.FromResponse(response))
	}

	request.Stream = true
	chunks, err := provider.Stream(cmd.Context(), request)
	if err != nil {
		return err
	}
	tap := &usageTap{}
	if err := output.Render(format, out, tap.wrap(chunks)); err != nil {
		return err
	}
	logUsage(log, start, tap.usage, tap.stop)
	return nil
}

// usageTap forwards a chunk stream unchanged while remembering the usage and
// stop reason, so both can be reported once rendering is done.
type usageTap struct {
	usage providers.Usage
	stop  string
}

func (t *usageTap) wrap(in <-chan providers.Chunk) <-chan providers.Chunk {
	out := make(chan providers.Chunk, cap(in))
	go func() {
		defer close(out)
		for chunk := range in {
			switch {
			case chunk.Type == providers.ChunkUsage && chunk.Usage != nil:
				t.usage = *chunk.Usage
			case chunk.Type == providers.ChunkMessageStop:
				t.stop = chunk.StopReason
			}
			out <- chunk
		}
	}()
	return out
}

// logUsage reports what the call cost and how long it took. A truncated
// response is worth a warning at the default level.
func logUsage(log *slog.Logger, start time.Time, usage providers.Usage, stop string) {
	log.Info("usage", "in", usage.InputTokens, "out", usage.OutputTokens,
		"stop", stop, "dur", time.Since(start).Round(time.Millisecond))
	if stop == providers.StopMaxTokens {
		log.Warn("response truncated: hit max_tokens", "out", usage.OutputTokens)
	}
}
