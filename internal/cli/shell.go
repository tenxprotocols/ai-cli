package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tenxprotocols/ai-cli/internal/providers"
)

func newShellCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "shell [task description...]",
		Short: "Turn a natural-language description into a shell command",
		Long: `Turn a natural-language description into a shell command.

Prints the command to stdout and never executes it. Compose as you like:

  ai shell find files larger than 500MB modified in the last week
  ai shell show kubernetes contexts | pbcopy
  eval "$(ai shell count lines of Go code in this repo)"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			description := strings.Join(args, " ")
			if description == "" {
				return errors.New("describe the task, e.g.: ai shell list open ports")
			}

			resolved, err := resolveForCall(cmd.Name(), flags)
			if err != nil {
				return err
			}
			provider, err := buildProvider(cmd.Context(), resolved)
			if err != nil {
				return err
			}

			system := systemPrompt(flags)
			if system == "" {
				system = shellSystem()
			}
			response, err := provider.Complete(cmd.Context(), providers.Request{
				Model:     resolved.Model,
				System:    system,
				MaxTokens: resolved.MaxTokens,
				Messages:  userMessage(description),
			})
			if err != nil {
				return err
			}

			command := sanitizeCommand(responseText(response))
			if command == "" {
				return errors.New("model returned no command")
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), command)
			return err
		},
	}
}

func shellSystem() string {
	shell := filepath.Base(os.Getenv("SHELL"))
	if shell == "" || shell == "." {
		shell = "sh"
	}
	return fmt.Sprintf(`You translate task descriptions into shell commands for %s using %s.
Reply with only the command: no markdown, no explanation, no leading $.
Prefer one line. Chain with && only when a single command cannot do the job.
If the task is destructive, still produce the command; the user reviews before running.`,
		runtime.GOOS, shell)
}

// sanitizeCommand strips markdown fences and prompt markers models sometimes
// add despite instructions.
func sanitizeCommand(reply string) string {
	reply = strings.TrimSpace(reply)
	if strings.HasPrefix(reply, "```") {
		lines := strings.Split(reply, "\n")
		lines = lines[1:] // drop opening fence (with any language tag)
		if n := len(lines); n > 0 && strings.HasPrefix(strings.TrimSpace(lines[n-1]), "```") {
			lines = lines[:n-1]
		}
		reply = strings.TrimSpace(strings.Join(lines, "\n"))
	}
	return strings.TrimPrefix(reply, "$ ")
}

func responseText(response providers.Response) string {
	var text strings.Builder
	for _, message := range response.Messages {
		for _, part := range message.Content {
			if part.Type == providers.PartText {
				text.WriteString(part.Text)
			}
		}
	}
	return text.String()
}
