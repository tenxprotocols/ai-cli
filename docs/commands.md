# Commands

Every command, flag, output format, and exit code. Config values and env equivalents live in [configuration.md](configuration.md).

## Global flags

Available on every subcommand.

| Flag | Env | Default | Meaning |
|---|---|---|---|
| `--profile <name>` | `AI_CLI_PROFILE` | config's `default_profile` | Select profile |
| `--provider <name>` | `AI_CLI_PROVIDER` | profile's provider | Override provider for this call |
| `--model <name>` | `AI_CLI_MODEL` | profile's model | Override model — passed verbatim |
| `--format text\|json\|jsonl` | `AI_CLI_FORMAT` | `text` | Output format |
| `--no-stream` | — | off | Wait for the full response before printing |
| `--system <text>` | `AI_CLI_SYSTEM` | profile's system | System prompt, inline |
| `--system-file <path>` | — | — | System prompt from file |
| `--config <path>` | `AI_CLI_CONFIG` | see configuration.md | Config file location |
| `--log-level <level>` | `AI_CLI_LOG_LEVEL` | `warn` | `error`, `warn`, `info`, `debug`, or `trace` |
| `--log-format text\|json` | `AI_CLI_LOG_FORMAT` | `text` | Log record encoding |
| `--log-file <path>` | `AI_CLI_LOG_FILE` | — | Write logs here instead of stderr (created `0600`) |
| `--log-secrets` | — | off | Log API keys and `Authorization` headers unredacted |

## Logging

Logs go to **stderr**, so stdout stays pipeable. Nothing is logged below `warn`
by default. Log flags are read before dispatch and work anywhere on the command
line — `ai --log-level=debug ask hi` and `ai ask --log-level=debug hi` are
equivalent — and they are passed through to `ai-<name>` plugins, which also
inherit `AI_CLI_LOG_LEVEL`.

| Level | What it adds |
|---|---|
| `error` | Failures, with the underlying cause (DNS, TLS, timeouts, HTTP status). The one-line `ai: <error>` message on stderr is unaffected by the level. |
| `warn` | **Default.** Missing API key, response truncated at `max_tokens`. |
| `info` | One line per milestone: config file loaded, profile/provider/model resolved, token usage and wall time, config writes. |
| `debug` | The decision trail: dispatch, every resolution step with the source that won, which env var supplied the key, HTTP method/URL/status/duration. |
| `trace` | The full exchange: request and response headers and bodies, streamed chunk by chunk. |

### Debugging a connection

`--log-level=trace` prints the whole HTTP exchange:

```console
$ ai --log-level=trace ask hi
+0ms DEBUG http: request method=POST url=https://api.anthropic.com/v1/messages
+0ms TRACE http: request headers anthropic-version=2023-06-01 x-api-key=••••1a2b
+0ms TRACE http: request body body="{\"model\":\"claude-opus-5\",...}"
+1ms DEBUG http: response status=200 dur=412ms
+1ms TRACE http: response body chunk="data: {\"type\":\"message_start\",...}"
```

Credentials in headers and in `?key=` query parameters are redacted to
`••••<last4>`, so a trace is safe to paste into an issue. `--log-secrets`
prints them in full, for when the key itself is the suspect — redaction only
ever affects the log, never the outbound request.

Response bodies are logged as they are read rather than buffered, so streaming
still arrives token by token under `trace`. A long trace is easier to read in a
file: `ai --log-level=trace --log-file=/tmp/ai.log ask hi`.

## Dispatch

`ai` resolves what to run in this order:

1. **Known subcommand** — `ai ask ...`, `ai shell ...`
2. **`ai-<name>` on `$PATH`** — `ai deploy prod` execs `ai-deploy prod` if it exists, so third parties can ship subcommands as separate binaries
3. **Fallback to ask** — `ai what is up` becomes `ai ask what is up`

Symlinks work git-style: `ai-shell list open ports` ≡ `ai shell list open ports` (`task install` creates `ai-ask`, `ai-shell`, `ai-config`, `ai-models`).

---

## `ai init`

Interactive setup wizard. Detects API keys in the environment and a running local Ollama, offers a live model list when credentials allow, and writes provider + profile to the config file. Existing config is merged — never overwritten. API keys are never written to disk; the wizard prints the `export` line to use instead.

```bash
ai init
```

## `ai ask [prompt words...]`

One-shot prompt. Positional words join into the prompt — no quoting needed. Piped stdin is prepended to the prompt.

```bash
ai ask why is the sky blue
ai ask -- "-v flag meaning in grep"          # -- stops flag parsing
git diff | ai ask write a commit message for this
cat error.log | ai ask                       # stdin alone is a valid prompt
ai --format json --no-stream ask capital of france
```

Streams by default with `--format text|jsonl`; `--format json` implies non-streaming. Empty prompt (no words, no stdin) is an error.

## `ai shell [task description...]`

Turns a natural-language description into **one shell command on stdout**. Nothing runs without your explicit choice.

```bash
ai shell find all files larger than 500MB created in the last week
# find . -type f -size +500M -mtime -7
# copy, run, or nothing? [C/r/n]

ai shell show kubernetes contexts | pbcopy
eval "$(ai shell count lines of go code in this repo)"
```

- **Interactive terminal:** after the command prints, choose **c**opy to clipboard (default — just hit Enter), **r**un it via `$SHELL -c` (its exit code becomes `ai`'s), or **n**othing. The prompt goes to stderr, so stdout is always exactly the command.
- **Piped or scripted** (either stdin or stdout not a TTY): the command prints and `ai` exits — no prompt, fully composable.
- Clipboard uses the first of `pbcopy`, `wl-copy`, `xclip`, `xsel`, `clip` found on `$PATH`.
- The built-in prompt targets your OS (`runtime.GOOS`) and `$SHELL`; markdown fences and `$ ` markers are stripped from the reply.
- `--system` replaces the built-in prompt; profile/command `system` values are ignored so a chatty profile can't break output.
- Give it a fast model via `[commands.shell] model = "..."` — see configuration.md.

## `ai models`

Lists models from configured providers as `provider/model-id` lines (display name appended when the provider reports one). Providers that error (missing key, unreachable) are reported on stderr; the rest still print.

```bash
ai models
ai --provider anthropic models
ai --format json models        # [{"provider":"...","id":"...","display_name":"..."}]
ai --format jsonl models       # one object per line
```

## `ai profile <subcommand>`

| Subcommand | Effect |
|---|---|
| `list` | All profiles; `*` marks the default |
| `show [name]` | Fields of a profile (default: active) |
| `use <name>` | Set `default_profile` in the file |
| `create <name> [--from <p>] [--provider <n>] [--model <m>] [--system <s>]` | Create; `--from` copies, other flags override. Needs provider + model |
| `rm <name> [--force]` | Remove; the default profile requires `--force` |

```bash
ai profile create work --provider anthropic --model claude-opus-4-7
ai profile create fast --from work --model claude-haiku-4-5
ai profile use work
```

## `ai config <subcommand>`

| Subcommand | Effect |
|---|---|
| `show [--show-secrets]` | Print the config; API keys redacted as `••••last4` unless `--show-secrets` |
| `path` | Print the resolved config file path |
| `get <key>` | Read one value by dotted path |
| `set <key> <value>` | Write one value by dotted path |
| `edit` | Open the file in `$EDITOR` (default `vi`) |

Supported dotted paths for `get`/`set`: `default_profile`, `providers.<name>.{type,api_key,base_url}`, `profiles.<name>.{provider,model,system}`. Everything else: `ai config edit`.

```bash
ai config set providers.bifrost.type openai-compat
ai config set providers.bifrost.base_url https://bifrost.example.com/v1
ai config get profiles.default.model
```

## `ai version` / `ai completion <shell>`

```bash
ai version              # ai v0.1.0 (darwin/arm64, go1.26.3)
ai completion zsh       # bash|zsh|fish|powershell completions
```

---

## Output formats

| Format | Streams | Shape |
|---|---|---|
| `text` | yes | Plain text; a trailing newline is guaranteed; tool calls summarize as `<tool-calls: N>` |
| `json` | no | One object: `{"text", "tool_calls", "stop_reason", "usage"}` |
| `jsonl` | yes | One event per line; first line is `{"type":"schema","version":"1"}`, then `text_delta`, `usage`, `message_stop` events |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Provider/API error (5xx, rate limit, network) |
| `2` | Usage error (unknown profile/provider, bad format) |
| `3` | Auth error (missing key, 401/403) |
| `4` | Input error (reserved) |
| `130` | Interrupted (Ctrl-C) |

Errors always go to stderr, so stdout stays clean for pipes.
