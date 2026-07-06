package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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

Prints the command to stdout. In an interactive terminal you are then
offered: copy to clipboard (default), run it, or do nothing. Nothing runs
without that explicit choice. Piped or scripted use prints the command only:

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
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), command); err != nil {
				return err
			}
			if !stdioIsTTY() {
				return nil // piped or scripted: stdout carries the command, nothing else
			}
			return offerAction(cmd.InOrStdin(), cmd.ErrOrStderr(), command)
		},
	}
}

// offerAction lets an interactive user act on the generated command. The
// prompt lives on stderr so stdout stays pure even in odd redirections.
func offerAction(in io.Reader, prompt io.Writer, command string) error {
	fmt.Fprint(prompt, "copy, run, or nothing? [C/r/n] ")
	scanner := bufio.NewScanner(in)
	answer := ""
	if scanner.Scan() {
		answer = strings.ToLower(strings.TrimSpace(scanner.Text()))
	}
	switch answer {
	case "", "c", "copy":
		if err := copyToClipboard(command); err != nil {
			return err
		}
		fmt.Fprintln(prompt, "copied")
		return nil
	case "r", "run":
		return runCommand(command)
	default:
		return nil
	}
}

// stdioIsTTY reports whether both stdin and stdout are terminals.
// Package-level so tests can substitute it.
var stdioIsTTY = func() bool {
	for _, f := range []*os.File{os.Stdin, os.Stdout} {
		info, err := f.Stat()
		if err != nil || info.Mode()&os.ModeCharDevice == 0 {
			return false
		}
	}
	return true
}

// runCommand executes the command through the user's shell with stdio
// attached. Package-level so tests can substitute it.
var runCommand = func(command string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	run := exec.Command(shell, "-c", command)
	run.Stdin, run.Stdout, run.Stderr = os.Stdin, os.Stdout, os.Stderr
	return run.Run()
}

// copyToClipboard pipes text to the first clipboard tool found.
// Package-level so tests can substitute it.
var copyToClipboard = func(text string) error {
	tools := [][]string{
		{"pbcopy"},
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
		{"clip"},
	}
	for _, tool := range tools {
		path, err := exec.LookPath(tool[0])
		if err != nil {
			continue
		}
		pipe := exec.Command(path, tool[1:]...)
		pipe.Stdin = strings.NewReader(text)
		return pipe.Run()
	}
	return errors.New("no clipboard tool found (pbcopy, wl-copy, xclip, xsel, clip)")
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
