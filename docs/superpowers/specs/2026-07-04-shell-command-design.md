# `ai shell` — Natural Language to Shell Command

**Date:** 2026-07-04
**Status:** Implemented
**Scope:** A subcommand that turns a natural-language task description into a single shell command.

## Problem

Remembering `find`/`kubectl`/`awk` incantations is a tax. The CLI already talks to LLMs; a dedicated subcommand can translate "find all files larger than 500MB created in the last week" into the command, ready to paste or substitute.

## Design

```
ai shell find all files larger than 500MB created in the last week
# → find . -type f -size +500M -mtime -7
```

- Positional args join into the description, same as `ask`. `ai-shell` works via the existing dispatch.
- Prints **only the command** to stdout, newline-terminated. Composable: `$(ai shell ...)`, `| pbcopy`, `eval`.
- **Never executes anything** — consistent with the v1 design's no-tool-execution stance. Review before you run.
- Built-in system prompt carries the platform (`runtime.GOOS`) and the user's shell (`$SHELL`), and demands bare-command output. An explicit `--system` flag overrides it; profile system prompts are ignored so a chatty profile can't break command generation.
- Non-streaming request; the response is sanitized (markdown fences and `$ ` markers stripped) before printing.
- Provider/model/profile selection is identical to `ask`.

## Alternatives considered

- `ai cmd` — tighter, but `shell` reads more naturally and matches prior art (`ai-shell`).
- `ai sh` / `ai do` — imply execution, which this deliberately does not do.
- `--run` flag to execute — rejected: out of scope per the v1 design's non-goals.
