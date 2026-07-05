# ai

A small, fast CLI for talking to LLMs. One static binary, zero AI-framework dependencies. Supports Anthropic, OpenAI, Gemini, OpenRouter, and any OpenAI-compatible endpoint (Ollama, LM Studio, vLLM, Groq, ...).

## Install

```bash
go install github.com/tenxprotocols/ai-cli/cmd/ai@latest
```

Or grab a binary from [releases](https://github.com/tenxprotocols/ai-cli/releases).

## Quickstart

```bash
export ANTHROPIC_API_KEY=...
cat > ~/.config/ai-cli/config.toml <<'EOF'
default_profile = "default"

[providers.anthropic]
type = "anthropic"

[profiles.default]
provider = "anthropic"
model    = "claude-sonnet-4-6"
EOF

ai what is the phase of the moon        # bare words become `ai ask ...`
```

## Commands

### `ai ask` — one-shot questions

```bash
ai ask why is the sky blue
git diff | ai ask write a commit message for this
ai --format json --no-stream ask capital of france
```

Piped stdin is prepended to the prompt. `--format text|json|jsonl` picks the output shape; text streams by default.

### `ai shell` — natural language to shell command

```bash
ai shell find all files larger than 500MB created in the last week
# find . -type f -size +500M -mtime -7

ai shell show kubernetes contexts | pbcopy
eval "$(ai shell count lines of go code in this repo)"
```

Prints exactly one command to stdout and **never executes it** — review, then run. The prompt knows your OS and shell.

### `ai models` — list models

```bash
ai models                    # all configured providers
ai --provider openai models  # one provider
```

### `ai profile` — switch provider/model/system as a unit

```bash
ai profile list
ai profile create work --provider anthropic --model claude-opus-4-7
ai profile use work
```

### `ai config` — read/write the config file

```bash
ai config show               # merged view, secrets redacted
ai config set profiles.default.model claude-opus-4-7
ai config edit               # opens $EDITOR
```

## Configuration

`~/.config/ai-cli/config.toml` (override with `--config` or `$AI_CLI_CONFIG`):

```toml
default_profile = "default"

[providers.anthropic]
type = "anthropic"

[providers.ollama]
type = "openai-compat"
base_url = "http://localhost:11434/v1"

[profiles.default]
provider = "anthropic"
model    = "claude-sonnet-4-6"

[profiles.local]
provider = "ollama"
model    = "llama3.1:70b"

# Per-subcommand overrides: shell uses a fast model, everything else doesn't.
[commands.shell]
model = "claude-haiku-4-5"
```

API keys come from env: `AI_CLI_<NAME>_API_KEY` first (uppercased provider block name), then the public convention (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`, `OPENROUTER_API_KEY`).

Precedence everywhere: **flags > `AI_CLI_*` env > public env > `[commands.<name>]` > profile**.

## Unix-friendly

- `ai-shell`, `ai-ask`, etc. work as symlinks (git-style dispatch); unknown subcommands fall through to `ai-<name>` binaries on `$PATH`, so third parties can add subcommands.
- Exit codes: `0` ok, `1` API error, `2` usage, `3` auth, `4` input, `130` interrupted.
- `--format jsonl` emits one event per line for agents; the first line is a schema marker.

## Development

```bash
mise install   # go, golangci-lint, go-task
task test      # go test -race ./...
task build     # ./bin/ai
```
