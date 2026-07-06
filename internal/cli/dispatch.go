package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

type ResolutionKind int

const (
	ResolveBuiltin ResolutionKind = iota
	ResolvePlugin
	ResolveAskFallback
)

type Resolution struct {
	Kind       ResolutionKind
	Args       []string // args to hand to cobra (for Builtin/AskFallback) or child (for Plugin)
	PluginPath string
}

// PluginLookup finds `ai-<name>` on $PATH. Returns (absPath, true) if found.
type PluginLookup func(name string) (string, bool)

// DefaultPluginLookup uses exec.LookPath.
func DefaultPluginLookup(name string) (string, bool) {
	p, err := exec.LookPath("ai-" + name)
	if err != nil {
		return "", false
	}
	return p, true
}

// ResolveArgs applies the dispatch rules described in the spec.
//
//	argv0    — os.Args[0]'s basename
//	argv     — full os.Args
//	known    — set of Cobra-registered subcommands
//	lookup   — plugin lookup (pass nil to skip plugin dispatch)
//
// Rules:
//  1. If argv0 matches "ai-<name>" and <name> is a known subcommand, rewrite
//     to {name, ...rest}.
//  2. Else split argv into (globalFlags, first-non-flag, rest). If the first
//     non-flag arg is a known subcommand, pass through. Otherwise, if a plugin
//     exists for it, exec that. Otherwise, treat everything after the global
//     flags as an `ask` invocation: {...globalFlags, "ask", ...rest}.
//  3. `ai` alone, or with only global flags/--help, pass through unchanged.
func ResolveArgs(argv0 string, argv []string, known map[string]bool, lookup PluginLookup) Resolution {
	base := filepath.Base(argv0)

	// Rule 1: argv0 is ai-<name>.
	if strings.HasPrefix(base, "ai-") && base != "ai" {
		name := strings.TrimPrefix(base, "ai-")
		if known[name] {
			return Resolution{Kind: ResolveBuiltin, Args: append([]string{name}, argv[1:]...)}
		}
		// Unknown ai-<name> binary invoked directly — let cobra show the error.
		return Resolution{Kind: ResolveBuiltin, Args: argv[1:]}
	}

	// Rule 2/3: argv[0] is "ai" (or equivalent). Partition rest into
	// (leadingFlags, positionals).
	rest := argv[1:]
	leading := []string{}
	i := 0
	for i < len(rest) {
		a := rest[i]
		if !strings.HasPrefix(a, "-") {
			break
		}
		// --help and -h short-circuit to cobra.
		if a == "--help" || a == "-h" || a == "--version" || a == "-v" {
			return Resolution{Kind: ResolveBuiltin, Args: rest}
		}
		leading = append(leading, a)
		// If flag takes a value and uses space form, consume the next arg.
		if !strings.Contains(a, "=") && flagTakesValue(a) && i+1 < len(rest) {
			i++
			leading = append(leading, rest[i])
		}
		i++
	}
	positionals := rest[i:]

	if len(positionals) == 0 {
		return Resolution{Kind: ResolveBuiltin, Args: rest}
	}
	first, tail := positionals[0], positionals[1:]

	if known[first] {
		return Resolution{Kind: ResolveBuiltin, Args: rest}
	}
	if lookup != nil {
		if path, ok := lookup(first); ok {
			childArgs := append([]string{"ai-" + first}, tail...)
			return Resolution{Kind: ResolvePlugin, Args: childArgs, PluginPath: path}
		}
	}
	// Fallback: everything becomes `ai ask ...`.
	out := make([]string, 0, len(leading)+1+len(positionals))
	out = append(out, leading...)
	out = append(out, "ask")
	out = append(out, positionals...)
	return Resolution{Kind: ResolveAskFallback, Args: out}
}

// flagTakesValue reports whether a global flag consumes the next argv entry.
// Boolean flags (--no-stream) do not. Any non-boolean global flag does.
func flagTakesValue(flag string) bool {
	name := strings.TrimLeft(flag, "-")
	if idx := strings.Index(name, "="); idx >= 0 {
		name = name[:idx]
	}
	switch name {
	case "no-stream":
		return false
	default:
		return true
	}
}

// Exec hands control to a plugin binary. On Unix this replaces the process;
// on Windows, it spawns and relays the exit code.
func Exec(path string, args []string) error {
	if runtime.GOOS == "windows" {
		cmd := exec.Command(path, args[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	return syscall.Exec(path, args, os.Environ())
}
