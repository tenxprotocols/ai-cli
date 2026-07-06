# Configuration

One TOML file plus environment variables. Everything the file can do, flags and env vars can override.

## Zero-config

With no config file (or one that defines no profiles), `ai` synthesizes a setup from the environment — first match wins:

| Source | Provider | Default model |
|---|---|---|
| `ANTHROPIC_API_KEY` | anthropic | `claude-sonnet-5` |
| `OPENAI_API_KEY` | openai | `gpt-5-mini` |
| `GEMINI_API_KEY` | gemini | `gemini-2.5-flash` (free tier: [aistudio.google.com](https://aistudio.google.com)) |
| `OPENROUTER_API_KEY` | openrouter | `openrouter/auto` |
| Local Ollama on `:11434` | ollama | first installed model |

`--model`, `--system`, and their `AI_CLI_*` env forms still apply. `[commands.<name>]` blocks and profiles require a config file — that's the point at which you write one.

## Where the file lives

First match wins:

1. `--config <path>` flag
2. `$AI_CLI_CONFIG`
3. `$XDG_CONFIG_HOME/ai-cli/config.toml`
4. `~/.config/ai-cli/config.toml` (macOS/Linux) or `%AppData%\ai-cli\config.toml` (Windows)

`ai config path` prints the resolved location.

## Precedence

For every setting, higher wins:

```
flag  >  AI_CLI_* env  >  public env (API keys only)  >  [commands.<name>]  >  [profiles.<name>]
```

## TL;DR — the settings you'll actually touch

| What | TOML | Env | Flag |
|---|---|---|---|
| Which profile to use | `default_profile = "work"` | `AI_CLI_PROFILE` | `--profile` |
| Model (verbatim string) | `profiles.<p>.model` | `AI_CLI_MODEL` | `--model` |
| Provider for a call | `profiles.<p>.provider` | `AI_CLI_PROVIDER` | `--provider` |
| API key | `providers.<n>.api_key` (discouraged) | `AI_CLI_<NAME>_API_KEY` or `ANTHROPIC_API_KEY` etc. | — |
| System prompt | `profiles.<p>.system` | `AI_CLI_SYSTEM` | `--system` / `--system-file` |
| Output format | — | `AI_CLI_FORMAT` | `--format` |
| Different model for one subcommand | `[commands.shell] model = "..."` | — | — |

Minimal working config:

```toml
default_profile = "default"

[providers.anthropic]
type = "anthropic"

[profiles.default]
provider = "anthropic"
model    = "claude-sonnet-4-6"
```

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

---

## Complete reference

### Top level

| Key | Type | Required | Meaning |
|---|---|---|---|
| `default_profile` | string | yes (or set `AI_CLI_PROFILE`) | Profile used when `--profile`/`AI_CLI_PROFILE` are unset |

### `[providers.<name>]`

`<name>` is your label for the endpoint — it becomes the `AI_CLI_<NAME>_API_KEY` env var (uppercased) and the prefix in `ai models` output. Define as many as you like.

| Key | Type | Required | Meaning |
|---|---|---|---|
| `type` | string | yes | One of `anthropic`, `openai`, `openrouter`, `gemini`, `openai-compat` |
| `base_url` | string | only for `openai-compat` | Endpoint root. Optional override for built-in types |
| `api_key` | string | no | Discouraged — prefer env. Used only when no env var matches |

Built-in `base_url` defaults:

| `type` | Default `base_url` |
|---|---|
| `anthropic` | `https://api.anthropic.com` |
| `openai` | `https://api.openai.com/v1` |
| `openrouter` | `https://openrouter.ai/api/v1` |
| `gemini` | `https://generativelanguage.googleapis.com/v1beta/openai` (Google's OpenAI-compatible endpoint) |
| `openai-compat` | none — required |

API key resolution, first non-empty wins:

| `type` | 1st | 2nd | 3rd |
|---|---|---|---|
| any | `AI_CLI_<NAME>_API_KEY` | *(per-type below)* | `api_key` in file |
| `anthropic` | ↑ | `ANTHROPIC_API_KEY` | ↑ |
| `openai` | ↑ | `OPENAI_API_KEY` | ↑ |
| `openrouter` | ↑ | `OPENROUTER_API_KEY` | ↑ |
| `gemini` | ↑ | `GEMINI_API_KEY`, then `GOOGLE_API_KEY` | ↑ |
| `openai-compat` | ↑ | *(none — key optional, e.g. Ollama)* | ↑ |

`<NAME>` is the **block name**, not the type: `[providers.claude]` with `type = "anthropic"` reads `AI_CLI_CLAUDE_API_KEY`, then falls back to `ANTHROPIC_API_KEY`.

### `[profiles.<name>]`

A profile bundles where and how to talk. Profiles never hold API keys.

| Key | Type | Required | Env override | Flag | Meaning |
|---|---|---|---|---|---|
| `provider` | string | yes | `AI_CLI_PROVIDER` | `--provider` | A `[providers.<name>]` block name |
| `model` | string | yes | `AI_CLI_MODEL` | `--model` | Passed **verbatim** to the provider — no parsing, slashes preserved |
| `system` | string | no | `AI_CLI_SYSTEM` | `--system`, `--system-file` | System prompt |
| `temperature` | float | no | — | — | Sampling temperature; provider default when unset |
| `max_tokens` | int | no | — | — | Response cap. Anthropic requires one; `4096` is sent when unset |

### `[commands.<name>]`

Same five keys as a profile. Applied on top of the active profile **only when that subcommand runs**. `<name>` is the subcommand: `ask`, `shell`, `models`.

```toml
[commands.shell]
model = "claude-haiku-4-5"    # shell answers come from a fast model

[commands.ask]
provider = "ollama"           # ask runs locally
model    = "llama3.1:70b"
```

Set fields override the profile; unset fields fall through. Flags and env still beat the block. There is no env form for command blocks — they exist to be *static* per-command defaults.

Note: `ai shell` ignores profile/command `system` values (its built-in prompt does the command generation); only an explicit `--system` flag replaces it.

### Environment variables — complete list

| Variable | Equivalent flag | Meaning |
|---|---|---|
| `AI_CLI_CONFIG` | `--config` | Config file path |
| `AI_CLI_PROFILE` | `--profile` | Active profile |
| `AI_CLI_PROVIDER` | `--provider` | Provider override |
| `AI_CLI_MODEL` | `--model` | Model override (verbatim) |
| `AI_CLI_SYSTEM` | `--system` | System prompt |
| `AI_CLI_FORMAT` | `--format` | `text`, `json`, or `jsonl` |
| `AI_CLI_<NAME>_API_KEY` | — | API key for provider block `<name>` |
| `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `OPENROUTER_API_KEY`, `GEMINI_API_KEY`, `GOOGLE_API_KEY` | — | Public-convention key fallbacks |
| `EDITOR` | — | Used by `ai config edit` (default `vi`) |
| `SHELL` | — | Tells `ai shell` which shell dialect to target |

Flag-only (no env): `--no-stream`, `--system-file`.
File-only (no env or flag): `temperature`, `max_tokens`, `[commands.*]` blocks, provider `base_url`/`type`.

### Editing from the CLI

`ai config get|set` understand these dotted paths: `default_profile`, `providers.<name>.type`, `providers.<name>.api_key`, `providers.<name>.base_url`, `profiles.<name>.provider`, `profiles.<name>.model`, `profiles.<name>.system`. Anything else (`temperature`, `max_tokens`, `commands.*`) — use `ai config edit`.

### Full example

```toml
default_profile = "default"

[providers.anthropic]
type = "anthropic"                       # key: AI_CLI_ANTHROPIC_API_KEY or ANTHROPIC_API_KEY

[providers.openrouter]
type = "openrouter"                      # key: AI_CLI_OPENROUTER_API_KEY or OPENROUTER_API_KEY

[providers.bifrost]                      # company proxy — any OpenAI-compatible endpoint
type     = "openai-compat"
base_url = "https://bifrost.example.com/v1"   # key: AI_CLI_BIFROST_API_KEY only

[providers.ollama]
type     = "openai-compat"
base_url = "http://localhost:11434/v1"   # no key needed

[profiles.default]
provider = "anthropic"
model    = "claude-sonnet-4-6"

[profiles.work]
provider    = "bifrost"
model       = "anthropic/claude-sonnet-5"   # verbatim — proxy namespacing preserved
system      = "You are a precise engineering assistant. Terse answers only."
temperature = 0.2
max_tokens  = 2048

[profiles.local]
provider = "ollama"
model    = "llama3.1:70b"

[commands.shell]
model = "claude-haiku-4-5"               # fast model for command generation
```
