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

func newShellCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "shell [task description...]",
		Short: "Turn a natural-language description into a shell command",
		Long: `Turn a natural-language description into a shell command.

Prints the command to stdout and never executes it. Compose as you like:

  ai shell find files larger than 500MB modified in the last week
  ai shell show kubernetes contexts | pbcopy
  eval "$(ai shell count lines of Go code in this repo)"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			desc := strings.Join(args, " ")
			if desc == "" {
				return errors.New("describe the task, e.g.: ai shell list open ports")
			}

			r, err := resolveForCall(gf)
			if err != nil {
				return err
			}
			p, err := buildProvider(cmd.Context(), r)
			if err != nil {
				return err
			}

			system := systemPrompt(gf)
			if system == "" {
				system = shellSystem()
			}
			resp, err := p.Complete(cmd.Context(), providers.Request{
				Model:     r.Model,
				System:    system,
				MaxTokens: r.MaxTokens,
				Messages:  userMessage(desc),
			})
			if err != nil {
				return err
			}

			command := sanitizeCommand(responseText(resp))
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
func sanitizeCommand(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		lines = lines[1:] // drop opening fence (with any language tag)
		if n := len(lines); n > 0 && strings.HasPrefix(strings.TrimSpace(lines[n-1]), "```") {
			lines = lines[:n-1]
		}
		s = strings.TrimSpace(strings.Join(lines, "\n"))
	}
	return strings.TrimPrefix(s, "$ ")
}

func responseText(resp providers.Response) string {
	var s strings.Builder
	for _, m := range resp.Messages {
		for _, p := range m.Content {
			if p.Type == providers.PartText {
				s.WriteString(p.Text)
			}
		}
	}
	return s.String()
}
