# AI CLI Design

**Date:** 2026-04-23
**Status:** Draft
**Scope:** A static-binary Go CLI (`ai`) for interacting with multiple LLM providers, composable in Unix pipelines and usable by AI agents.

## Problem

There is no single CLI that lets a developer (or an agent) talk to Anthropic, OpenAI, Gemini, OpenRouter, and arbitrary OpenAI-compatible endpoints through one consistent interface, compose with Unix pipes, and be extended by third parties — while remaining trivial to install as a single static binary.

## Goals

- One Go binary, cross-platform, installable via download or `go install`.
- Git-style dispatch: both `ai chat` and `ai-chat` work; unknown subcommands fall through to `ai-<name>` on `$PATH`.
- Unix-first: text in, text out, pipes, stdin, useful exit codes. Opt-in JSON/JSONL for machines.
- Multiple providers behind a thin, uniform interface. Adding an OpenAI-compatible endpoint is zero Go code.
- Profiles so users can swap provider + model + system prompt as one unit.
- Tool-use pass-through: the CLI surfaces tool-call intents to the caller but never executes them.
- Persistent, resumable chat sessions stored as append-only JSONL.

## Non-Goals

- The CLI is not an agent framework; it does not execute tools, spawn subprocesses, or orchestrate multi-step workflows on the model's behalf.
- No telemetry, no analytics, no crash reporting.
- No OS keychain integration in v1 (plain-file secrets with env overrides).
- No TUI chat interface beyond a single input textarea; response rendering remains plain stdout.

## Design Principles

- **Thin wrapper, not a framework.** Adapters translate types and stream events; retries, SSE parsing, and model-specific features are the SDK's job.
- **Verbatim data.** `--model` strings are passed through to the provider untouched. No magic parsing.
- **Explicit over magic.** Output format is chosen with `--format`, never auto-switched on TTY detection.
- **Composable.** Everything works under pipes: `cat | ai ask | jq`.
- **Extensible at the edges.** New subcommands can be shipped as separate binaries discovered on `$PATH`; new OpenAI-compatible providers are added by TOML block.
- **YAGNI.** Features not on this list are out of scope for v1.

## 1. Architecture & Project Layout

### 1.1 Binary dispatch

A single Go binary, `ai`. At install time, symlinks `ai-chat`, `ai-ask`, `ai-config`, `ai-models` are created pointing at it. The entry point inspects `argv[0]`:

- If `argv[0]` matches `ai-<name>`, args are rewritten to `[ai, <name>, ...]` and handed to Cobra.
- If invoked as plain `ai <name> ...` and `<name>` is not a Cobra-registered subcommand, the dispatcher checks `$PATH` for `ai-<name>` and, if found, hands over to it. On Unix this uses `syscall.Exec` to fully replace the process. On Windows, `exec.Command` is used with stdin/stdout/stderr wired through and the child's exit code propagated.
- If no Cobra subcommand matches and no `ai-<name>` exists on `$PATH`, args are treated as a prompt and forwarded to the `ask` subcommand. So `ai what is the phase of the moon` is shorthand for `ai ask what is the phase of the moon`.

Resolution order for `ai <first-word> [rest...]`:

1. Exact Cobra subcommand match (`chat`, `ask`, `config`, `models`, `profile`, `version`)
2. `ai-<first-word>` executable on `$PATH`
3. Fallback: `ai ask <first-word> <rest...>`

Global flags parse first regardless of dispatch path, so `ai --model foo what is X` works.

### 1.2 Project layout

```
ai-cli/
├── cmd/ai/
│   └── main.go               # argv[0] + PATH dispatch, cobra root
├── internal/
│   ├── cli/                  # one file per subcommand
│   │   ├── ask.go
│   │   ├── chat.go
│   │   ├── config.go
│   │   ├── models.go
│   │   └── profile.go
│   ├── providers/
│   │   ├── provider.go       # Provider interface + shared types
│   │   ├── registry.go       # name -> constructor
│   │   ├── anthropic.go
│   │   ├── openai.go         # backs openai, openrouter, openai-compat
│   │   ├── gemini.go
│   │   └── testdata/         # recorded fixtures
│   ├── config/
│   │   ├── load.go           # TOML + env merge
│   │   ├── profile.go
│   │   └── schema.go
│   ├── chat/
│   │   ├── repl.go           # bubbletea textarea + slash completion
│   │   └── store.go          # JSONL persistence
│   ├── output/
│   │   ├── text.go
│   │   ├── json.go
│   │   └── jsonl.go
│   └── version/
│       └── version.go        # set via ldflags; release-please source
├── test/
│   ├── cli/                  # golden tests for the built binary
│   └── fake-openai/          # in-process OpenAI-compatible test server
├── docs/
├── .github/workflows/
│   ├── ci.yml
│   ├── release-please.yml
│   └── release-build.yml
├── Taskfile.yml
├── release-please-config.json
├── .release-please-manifest.json
├── go.mod
└── README.md
```

### 1.3 Dependency boundaries

- `cmd/ai` imports only `internal/cli`.
- `internal/cli` imports `providers`, `config`, `chat`, `output`, `version`.
- `internal/providers` imports no other internal package; adapters receive constructor configs.
- `internal/config` knows provider names as strings, not provider types.
- `internal/chat` and `internal/output` are pure: no network, no provider knowledge beyond the `Chunk`/`Message` types defined in `providers`.

Binary size target: ~25MB static, stripped. Built with `CGO_ENABLED=0` for the static variant.

## 2. Command Surface

### 2.1 Global flags

All flags apply to every subcommand unless noted.

| Flag | Meaning |
|---|---|
| `--profile <name>` | Select profile; default is `$AI_CLI_PROFILE` or the config's `default_profile` |
| `--provider <name>` | Override the profile's provider for this invocation |
| `--model <name>` | Override the profile's model; passed **verbatim** to the current provider |
| `--format text\|json\|jsonl` | Output format; default `text` |
| `--no-stream` | Disable streaming even when format allows it |
| `--system <text>` | Override profile's system prompt with inline text |
| `--system-file <path>` | Override profile's system prompt with file contents |
| `--config <path>` | Override config file location (also via `$AI_CLI_CONFIG`) |

### 2.2 `ai ask [flags] [prompt words...]`

- Positional args are joined with spaces to form the prompt. Unquoted prompts are supported: `ai ask --file img.png what is this an image of` works as typed.
- Use `--` to stop flag parsing when the prompt starts with `-`.
- `--file <path>` (repeatable) attaches a file. Text files are inlined as text content; binary files are rejected unless passed as `--image`.
- `--image <path>` (repeatable) attaches an image.
- `--tools <path>` enables tool-use pass-through (see §6).
- `--conversation <path>` reads a JSONL conversation file and appends the new turn (for agent-driven multi-turn flows).
- Input assembly order (before sending to provider): system prompt, files, images, piped stdin (only if stdin is not a TTY), positional prompt.
- Streams to stdout by default. Exit codes per §6.

### 2.3 `ai chat [flags]`

Opens an interactive REPL. Input uses a `charmbracelet/bubbletea` textarea component (not a full-screen TUI) for input-line editing; responses stream plain to stdout after each submit.

Input features:

- **Bracketed paste** is enabled so pasting multi-line markdown never submits prematurely.
- **Enter** submits. **Shift+Enter** inserts a newline when the terminal supports the Kitty keyboard protocol or CSI u extended modifiers (kitty, WezTerm, Ghostty, iTerm2 with "report modifiers" enabled, recent Windows Terminal, Alacritty with matching config). On terminals that can't disambiguate, the REPL falls back to a `\` line-continuation convention and prints a one-time hint explaining it.
- **Tab** triggers slash-command completion via a dropdown below the input. Completion is limited to `/` commands.

Slash commands:

| Command | Effect |
|---|---|
| `/model <name>` | Change model for the session (verbatim, same rules as `--model`) |
| `/provider <name>` | Change provider for the session |
| `/system <text>` | Set system prompt for the session |
| `/save` | Force a save/flush (normally automatic) |
| `/clear` | Start a new conversation in the same REPL |
| `/exit` or `Ctrl-D` | Exit |
| `/help` | Show commands |

Flags and subcommands:

- `--resume <id>` reopens a saved conversation. `<id>` is the short hash; shortest unambiguous prefix is accepted.
- `--new` starts a fresh conversation even if `--resume` is in scope.
- `ai chat list` — table of recent conversations (ID, title, model, updated)
- `ai chat show <id>` — dumps the conversation (`--format` respected)
- `ai chat rm <id>` / `ai chat rm --older-than 30d` / `ai chat rm --all --yes`
- `ai chat export <id> --format markdown|json`

### 2.4 `ai config ...`

- `ai config get <key>` — dotted path, e.g. `profiles.work.model`
- `ai config set <key> <value>`
- `ai config show` — fully merged view (file + env + flag defaults). Secrets redacted as `••••<last4>` unless `--show-secrets`.
- `ai config path`
- `ai config edit` — opens `$EDITOR` on the config file

### 2.5 `ai profile ...`

Promoted to a top-level command.

- `ai profile list` — names, active marker, one-line summary (provider/model)
- `ai profile show [name]` — resolved view of a profile (defaults to active)
- `ai profile use <name>` — sets `default_profile` in the config file
- `ai profile create <name> [--from <existing>]` — creates a new profile, optionally copying from an existing one; prompts for overrides
- `ai profile rm <name>` — refuses to remove the active default unless `--force`

### 2.6 `ai models [flags]`

Lists models from configured providers.

- `--provider <name>` filters to one provider.
- Default text output is a table: `provider/model  context  input$/M  output$/M`. Price columns left blank for providers that do not expose pricing (e.g., Ollama).
- Honors `--format`.
- Models are fetched via each provider's list endpoint where available. For providers without one (some OpenAI-compatible endpoints), output falls back to the models listed in the profile or provider config block.

### 2.7 `ai version` / `--version`

Prints `ai v<semver> (commit <sha>, go<version>)`.

## 3. Provider Interface & Adapter Strategy

### 3.1 Core types

Defined in `internal/providers/provider.go`.

```go
type Message struct {
    Role       string        // "system" | "user" | "assistant" | "tool"
    Content    []ContentPart
    ToolCalls  []ToolCall    // set on assistant messages that called tools
    ToolCallID string        // set on role="tool" messages
}

type ContentPart struct {
    Type     string // "text" | "image" | "file"
    Text     string
    MIMEType string
    Data     []byte // raw; adapter handles base64/upload per provider
}

type ToolDef struct {
    Name        string
    Description string
    Schema      json.RawMessage // JSON Schema for inputs
}

type ToolCall struct {
    ID        string
    Name      string
    Arguments json.RawMessage
}

type Chunk struct {
    Type     string // "text_delta" | "tool_call_start" | "tool_call_delta" |
                    // "thinking_delta" | "message_stop" | "usage" | "error"
    Text     string
    ToolCall *ToolCall
    Usage    *Usage
    Err      error
}

type Usage struct {
    InputTokens    int
    OutputTokens   int
    CacheReadTokens  int
    CacheWriteTokens int
}

type Request struct {
    Model       string
    Messages    []Message
    System      string
    Tools       []ToolDef
    Temperature *float64
    MaxTokens   *int
    Stream      bool
}

type Response struct {
    Messages   []Message
    Usage      Usage
    StopReason string // "end_turn" | "max_tokens" | "tool_use" | "stop_sequence" | "error"
}

type Provider interface {
    Name() string
    Complete(ctx context.Context, req Request) (Response, error)
    Stream(ctx context.Context, req Request) (<-chan Chunk, error)
    ListModels(ctx context.Context) ([]ModelInfo, error) // may return ErrNotSupported
}

type ModelInfo struct {
    ID             string
    DisplayName    string
    ContextTokens  int
    InputPricePerM  *float64
    OutputPricePerM *float64
}
```

### 3.2 Adapters

One file per SDK. Each is ~150–250 lines and does only type translation + SDK plumbing.

- **`anthropic.go`** — wraps `anthropic-sdk-go`. Handles content blocks, `tool_use` blocks, and extended thinking (surfaced as `Chunk{Type: "thinking_delta"}`; text formatter ignores, JSON/JSONL formatter emits).
- **`openai.go`** — wraps `openai-go`. Constructor takes `BaseURL` and `APIKey`. This single adapter backs three config `type` values: `openai`, `openrouter`, and `openai-compat` (the last of which covers Bifrost, Ollama, LM Studio, vLLM, Groq, Together, and any other OpenAI-compatible endpoint via `base_url`). OpenRouter-specific headers (`HTTP-Referer`, `X-Title`) are attached via the SDK's request-option hook when the adapter is instantiated with `type = "openrouter"`.
- **`gemini.go`** — wraps `google.golang.org/genai`. Maps `ContentPart` to Gemini's `Part` type.

Adapters deliberately do **not** implement retries, SSE parsing, caching, prompt templating, model fallbacks, or automatic tool execution. All of those either live in the SDK or are out of scope.

### 3.3 Streaming

`Stream` returns a `<-chan Chunk` that closes when the request ends (success, error, or context cancel). Callers range over it. On `ctx.Done()`, the adapter cancels the underlying SDK call and closes the channel with a final `Chunk{Type: "error", Err: ctx.Err()}` if the cancellation is observed before natural termination. Each stream owns a single producer goroutine that closes the channel via `defer`.

### 3.4 Registry

Config resolution produces a `map[string]Provider`. Keys are user-facing provider names from the TOML (`anthropic`, `openai`, `openrouter`, `bifrost`, `ollama`, etc.). Built-in constructors handle the four known types; user-defined `openai-compat` endpoints are registered dynamically from `[providers.<name>]` blocks.

Adding a new OpenAI-compatible endpoint requires zero Go code:

```toml
[providers.ollama]
type = "openai-compat"
base_url = "http://localhost:11434/v1"
# api_key optional; env fallback: AI_CLI_OLLAMA_API_KEY
```

## 4. Configuration, Profiles, and Chat Persistence

### 4.1 Config file

Location: `$XDG_CONFIG_HOME/ai-cli/config.toml`, falling back to `~/.config/ai-cli/config.toml` (macOS/Linux) or `%APPDATA%/ai-cli/config.toml` (Windows). Overridable via `--config <path>` or `$AI_CLI_CONFIG`.

Example:

```toml
default_profile = "default"

[providers.anthropic]
type = "anthropic"
# api_key optional; env fallback described below

[providers.openai]
type = "openai"

[providers.openrouter]
type = "openrouter"

[providers.bifrost]
type = "openai-compat"
base_url = "https://bifrost.example.com/v1"

[providers.ollama]
type = "openai-compat"
base_url = "http://localhost:11434/v1"

[profiles.default]
provider = "anthropic"
model    = "claude-sonnet-4-6"

[profiles.work]
provider    = "anthropic"
model       = "claude-opus-4-7"
system      = "You are a precise engineering assistant. Terse answers only."
temperature = 0.2

[profiles.local]
provider = "ollama"
model    = "llama3.1:70b"

[profiles.router]
provider = "bifrost"
model    = "anthropic/claude-opus-4-7"  # verbatim, slash preserved
```

Writes via `ai config set` use `pelletier/go-toml/v2` with comment preservation.

### 4.2 Environment variables

All app-specific env vars are prefixed `AI_CLI_`:

- `AI_CLI_CONFIG` — config file path override
- `AI_CLI_PROFILE` — active profile
- `AI_CLI_PROVIDER`, `AI_CLI_MODEL`, `AI_CLI_FORMAT` — equivalents of the flags
- `AI_CLI_<NAME>_API_KEY` — API key override per provider. `<NAME>` is the uppercased **config block name** (the key in `[providers.<name>]`), not the `type`. Example: `[providers.claude]` with `type = "anthropic"` reads `AI_CLI_CLAUDE_API_KEY`.

For built-in provider *types*, the public convention is honored as a secondary fallback keyed off the `type`. So `[providers.claude]` with `type = "anthropic"` also falls back to `ANTHROPIC_API_KEY` when `AI_CLI_CLAUDE_API_KEY` is unset. For user-defined `openai-compat` endpoints, only the `AI_CLI_<NAME>_API_KEY` form is consulted (no public convention exists).

| Provider | Primary env var | Fallbacks |
|---|---|---|
| Anthropic | `AI_CLI_ANTHROPIC_API_KEY` | `ANTHROPIC_API_KEY` |
| OpenAI | `AI_CLI_OPENAI_API_KEY` | `OPENAI_API_KEY` |
| Gemini | `AI_CLI_GEMINI_API_KEY` | `GEMINI_API_KEY`, then `GOOGLE_API_KEY` |
| OpenRouter | `AI_CLI_OPENROUTER_API_KEY` | `OPENROUTER_API_KEY` |
| openai-compat `<name>` | `AI_CLI_<NAME>_API_KEY` | (none) |

### 4.3 Resolution order

Higher wins:

1. CLI flags
2. `AI_CLI_*` env vars
3. Public provider env vars
4. TOML file values

### 4.4 Model-string resolution

`--model` values are passed **verbatim** to the current provider. No slash parsing, no prefix magic.

- "Current provider" is `--provider` (if set), otherwise the active profile's provider.
- Examples, assuming profile `router` has `provider = "bifrost"`:
  - `ai ask --model anthropic/claude-opus-4-7 hi` → Bifrost, model `anthropic/claude-opus-4-7`
  - `ai ask --profile work --model claude-opus-4-7 hi` → Anthropic, model `claude-opus-4-7`
  - `ai ask --provider anthropic --model claude-opus-4-7 hi` → Anthropic, model `claude-opus-4-7`
  - `ai ask --provider openrouter --model anthropic/claude-3.5-sonnet hi` → OpenRouter, model `anthropic/claude-3.5-sonnet`

### 4.5 `ai config show`

Prints the fully merged view (file + env + flag defaults). API keys are redacted as `••••<last4>` by default. `--show-secrets` un-redacts.

### 4.6 Profiles

See §2.5 for commands. A profile captures `provider`, `model`, optional `system`, and optional sampling parameters (`temperature`, `max_tokens`). Profiles never store API keys — those come from provider blocks and env.

### 4.7 Chat persistence

Storage root: `$XDG_DATA_HOME/ai-cli/chats/` (`~/.local/share/ai-cli/chats/` on macOS/Linux, `%LOCALAPPDATA%/ai-cli/chats/` on Windows).

One JSONL file per conversation, named `<timestamp>-<short-id>.jsonl`:

```jsonl
{"type":"meta","id":"a1b2c3d4","created":"2026-04-23T14:02:10Z","provider":"anthropic","model":"claude-opus-4-7","profile":"work","title":"Debugging Raft leader election"}
{"type":"message","role":"system","content":[{"type":"text","text":"You are..."}]}
{"type":"message","role":"user","content":[{"type":"text","text":"Why does my Raft cluster..."}]}
{"type":"message","role":"assistant","content":[{"type":"text","text":"The most likely cause..."}],"usage":{"input":1240,"output":380}}
```

Rationale for JSONL: append-only (no rewriting per turn), greppable, robust against partial writes, easy to ingest from other tools. The first line is always `type: "meta"`. Readers skip unparseable lines as corruption.

Titles are generated after the first assistant response via a ≤8-word prompt to the current model. Cached; not regenerated on resume.

No central index file. `chat list` globs the directory and reads the first line of each file. At 10k conversations this is still sub-second; an index is added only if measurement shows a need.

`ai chat show` does not redact content (these are the user's files). `ai config show` does.

## 5. Tool Use Pass-Through

### 5.1 Input

`--tools <path>` accepts a JSON file containing an array of tool definitions in the Anthropic-normalized shape:

```json
[
  {
    "name": "get_weather",
    "description": "Get current weather for a city.",
    "input_schema": {
      "type": "object",
      "properties": { "city": { "type": "string" } },
      "required": ["city"]
    }
  }
]
```

Adapters translate to each provider's tool schema (OpenAI's `function` objects, Gemini's `FunctionDeclaration`).

### 5.2 Behavior

When the model requests tool calls, `ai` **surfaces them and stops**. It never executes tools. The caller (an agent, script, or another CLI) decides what to do.

### 5.3 Multi-turn continuation

Callers continue a conversation by passing a JSONL conversation file containing prior messages plus `role: "tool"` replies they generated. The file format is the same JSONL used by `ai chat` persistence (§4.7): a leading `type: "meta"` line followed by `type: "message"` lines.

```
ai ask --tools tools.json --conversation conv.jsonl "continue"
```

`ai` reads `conv.jsonl`, appends the new user turn, sends the request, and appends the resulting assistant message(s) back to the same file.

### 5.4 Output per format

| `--format` | Behavior when model calls tools |
|---|---|
| `text` | Emits any pre-tool assistant text, then a terminal line `<tool-calls: N>`, exits 0. A first-use stderr hint recommends `--format json` when `--tools` is set. |
| `json` | Single object: `{"text", "tool_calls", "stop_reason", "usage", "model", "id"}` |
| `jsonl` | One event per line: `text_delta`, `tool_call` (complete with parsed `arguments`), `usage`, `stop`. Versioned via a `schema` field on the first event. |

## 6. Output Formats, Errors, Exit Codes

### 6.1 Output formats

| Format | Streams? | Use case |
|---|---|---|
| `text` | yes | humans, simple scripts, pipe composition |
| `json` | no | agents wanting the whole response |
| `jsonl` | yes | agents wanting incremental events |

JSONL schema is versioned; the first event on the stream is `{"type":"schema","version":"1"}` so consumers can detect incompatible changes.

### 6.2 Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | API/provider error (model error, rate limit, 5xx after SDK retries) |
| 2 | usage error (bad flag, missing config, unknown profile/provider) |
| 3 | auth error (missing/invalid key) |
| 4 | input error (unreadable file, bad tools JSON, invalid conversation file) |
| 130 | interrupted (Ctrl-C) |

### 6.3 Error surfacing

Errors always go to stderr, human-readable. With `--format json|jsonl`, a final error event also goes to stdout so pipelines get structured signal while humans get a readable message.

## 7. Testing Strategy

- **Unit tests** — every package. Table-driven where natural. High-value targets: config merge, profile resolution, model-string routing, output formatters, argv dispatch, JSONL store append/read.
- **Provider adapter tests** — driven by recorded fixtures under `internal/providers/testdata/`. Fixtures cover plain text, streaming text, tool calls, images, and error shapes. **No live API calls in CI.**
- **Fake OpenAI server** — `test/fake-openai/` runs an in-process HTTP server implementing enough of the OpenAI protocol (chat completions non-stream + SSE) to exercise the `openai-compat` adapter end-to-end without network.
- **CLI golden tests** — `test/cli/` runs the built binary against scripted inputs and compares stdout/stderr/exit against golden files. Covers: flag parsing, argv dispatch, `ai <words>` shortcut, symlink dispatch, plugin fallback.
- **No live-model snapshot tests.** Those are flaky and test the wrong thing.
- **Race detector always on** in test runs: `go test -race ./...`.

## 8. Build & Release

### 8.1 Taskfile

`Taskfile.yml` at repo root, using [go-task](https://taskfile.dev).

Core tasks:

| Task | Effect |
|---|---|
| `task build` | Default non-static build → `./bin/ai` |
| `task build:static` | Static build (`CGO_ENABLED=0`) → `./bin/ai-static` |
| `task build:all` | Both of the above |
| `task install` | Builds, installs to `/usr/local/bin` (or `$AI_CLI_INSTALL_DIR`), creates symlinks `ai-chat`, `ai-ask`, `ai-config`, `ai-models` |
| `task test` | `go test -race ./...` |
| `task lint` | golangci-lint |
| `task fmt` | goimports + gofumpt |
| `task snapshot` | Local cross-compile test (no publish) |
| `task release:ci` | CI-only; cross-compiles all platforms, writes release assets |

Task features used: `sources:` / `generates:` for incremental builds, `deps:` for `fmt → lint → test → build` chaining, `preconditions:` for tool checks, `vars:` for version injection.

Build flags for every target:

```
CGO_ENABLED=<0|1> go build -trimpath \
  -ldflags="-s -w -X <pkg>/internal/version.Version=$(git describe --tags --always)"
```

### 8.2 Static vs non-static

| OS/Arch | Non-static | Static |
|---|---|---|
| linux/amd64 | ✓ | ✓ |
| linux/arm64 | ✓ | ✓ |
| darwin/amd64 | ✓ | — |
| darwin/arm64 | ✓ | — |
| windows/amd64 | ✓ | — |

Static is `CGO_ENABLED=0` — portable, drops into `scratch`/Alpine, no glibc dependency. Non-static uses CGO for native DNS resolver and system cert store. On macOS and Windows, true-static is not meaningful (system libraries are dynamic by design), so only the default variant ships.

Tarball naming:

- `ai_<version>_<os>_<arch>.tar.gz` — default
- `ai_<version>_linux_<arch>_static.tar.gz` — static variants

### 8.3 Release automation — release-please

Commit convention is Conventional Commits (`feat:`, `fix:`, `chore:`, etc.). `gscribe commit --dry-run` is used to draft commit messages per the workspace convention; the project README documents this.

Workflows:

- `.github/workflows/release-please.yml` — runs [release-please](https://github.com/googleapis/release-please) on every push to `main`. It maintains a standing "Release PR" accumulating Conventional Commits and a generated `CHANGELOG.md`. Merging creates a GitHub Release + git tag and bumps `internal/version/version.go`.
- `.github/workflows/release-build.yml` — triggered on `release: published`. Runs `task release:ci` to cross-compile the matrix, compute SHA256 checksums, and upload tarballs + `checksums.txt` as release assets. Also updates a Homebrew tap and Scoop bucket via template-substitution steps.
- `.github/workflows/ci.yml` — test + lint matrix over {linux, macos} × {amd64, arm64} on every push/PR.

release-please config:

- `release-please-config.json` — single-package, Go release type, version source `go`
- `.release-please-manifest.json` — current version tracking

### 8.4 Toolchain

Go toolchain version pinned in `go.mod` (`go 1.22` at minimum). `.claude/mise.toml` adds both `go` and `go-task` so contributors get the right versions via `mise install`.

## 9. Open Questions / Future Work

Explicitly deferred:

- OS keychain integration for API keys
- Full-screen TUI chat mode
- Built-in tool execution (shell, file I/O, HTTP) — would turn this into an agent framework, which is out of scope
- Prompt templates / prompt library
- Cost tracking across sessions
- Multi-provider parallel queries (`--compare`)
- Structured output / JSON schema-enforced responses
- Embeddings (`ai embed`) and non-chat completion (`ai complete`) — candidates for v2

None of these are blocked by the v1 design; they can be added as additional subcommands or provider-interface methods without reshaping the core.
