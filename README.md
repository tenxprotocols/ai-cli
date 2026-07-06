# ai

A small, fast CLI for talking to LLMs. One static binary, zero AI-framework dependencies.

Works with **Anthropic**, **OpenAI**, **Gemini**, **OpenRouter**, and any **OpenAI-compatible** endpoint (Ollama, LM Studio, vLLM, Groq, Bifrost, ...).

## Install

```bash
brew install tenxprotocols/tap/ai            # macOS
go install github.com/tenxprotocols/ai-cli/cmd/ai@latest
# or download a tarball: https://github.com/tenxprotocols/ai-cli/releases
```

## 60-second start

```bash
export ANTHROPIC_API_KEY=sk-ant-...

mkdir -p ~/.config/ai-cli && cat > ~/.config/ai-cli/config.toml <<'EOF'
default_profile = "default"

[providers.anthropic]
type = "anthropic"

[profiles.default]
provider = "anthropic"
model    = "claude-sonnet-4-6"
EOF

ai what is the phase of the moon
```

## What it does

| Command | Purpose |
|---|---|
| `ai <words>` / `ai ask` | One-shot prompt; streams; reads piped stdin |
| `ai shell <description>` | Natural language → one shell command on stdout (never executed) |
| `ai models` | List models across configured providers |
| `ai profile` | Switch provider + model + system prompt as a unit |
| `ai config` | Read/write the config file |

```bash
git diff | ai ask write a commit message for this
ai shell find all files larger than 500MB created in the last week
ai --format json --no-stream ask capital of france     # for scripts and agents
```

Unix-first: pipes in, plain text out, `--format json|jsonl` for machines, meaningful exit codes, and git-style dispatch (`ai-shell` symlinks; unknown subcommands run `ai-<name>` from `$PATH`).

## Docs

- **[Commands](docs/commands.md)** — every command, flag, output format, and exit code
- **[Configuration](docs/configuration.md)** — every config value and its env equivalent, from a five-line TLDR to the full reference

## Development

```bash
mise install    # go, golangci-lint, go-task
task test       # go test -race ./...
task build      # ./bin/ai
```

Releases: push a `v*.*.*` tag; GoReleaser does the rest. MIT licensed.
