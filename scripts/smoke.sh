#!/usr/bin/env bash
# Contract checks for a built ai binary: it runs, carries a stamped version,
# lists its commands, keeps logs off stdout, and fails helpfully when nothing
# is configured. Run via `task smoke` (or `task smoke:archive ARCHIVE=...`).
set -euo pipefail

BIN="${1:-./bin/ai}"

fail() {
  echo "smoke: $*" >&2
  exit 1
}

[ -x "$BIN" ] || fail "$BIN is not an executable file"
echo "smoke: testing $BIN"

# --- version is stamped by ldflags, not the "dev" fallback -------------------
version_output="$("$BIN" version)"
echo "  version -> $version_output"
case "$version_output" in
  "ai "*) ;;
  *) fail "expected version output to start with 'ai ', got: $version_output" ;;
esac
version_field="$(printf '%s' "$version_output" | awk '{print $2}')"
[ -n "$version_field" ] || fail "version output carries no version field"
[ "$version_field" != "dev" ] || fail "version is the 'dev' fallback: ldflags did not apply"

# --- help lists every subcommand --------------------------------------------
help_output="$("$BIN" --help)"
for subcommand in ask shell config models profile init version; do
  case "$help_output" in
    *"$subcommand"*) ;;
    *) fail "--help does not mention the '$subcommand' subcommand" ;;
  esac
done
echo "  --help -> lists all subcommands"

# --- config path resolves with no config file present ------------------------
"$BIN" config path >/dev/null || fail "config path failed"
echo "  config path -> ok"

# --- logs go to stderr, never stdout ----------------------------------------
stdout_only="$("$BIN" --log-level=debug version 2>/dev/null)"
case "$stdout_only" in
  *DEBUG*) fail "debug logs leaked into stdout: $stdout_only" ;;
esac
stderr_only="$("$BIN" --log-level=debug version 2>&1 >/dev/null)"
case "$stderr_only" in
  *DEBUG*) ;;
  *) fail "expected debug logs on stderr, got: $stderr_only" ;;
esac
echo "  --log-level=debug -> logs on stderr only"

# --- an unknown log level is rejected ---------------------------------------
if "$BIN" --log-level=bogus version >/dev/null 2>&1; then
  fail "an unknown --log-level should be rejected"
fi
echo "  --log-level=bogus -> rejected"

# --- an unconfigured ask explains itself and exits 1 -------------------------
set +e
unconfigured="$(
  env -u ANTHROPIC_API_KEY -u OPENAI_API_KEY -u OPENROUTER_API_KEY \
      -u GEMINI_API_KEY -u GOOGLE_API_KEY -u AI_CLI_PROFILE \
      AI_CLI_CONFIG=/nonexistent/ai-cli-smoke.toml \
      "$BIN" ask hi 2>&1
)"
unconfigured_code=$?
set -e
[ "$unconfigured_code" -eq 1 ] || fail "unconfigured ask should exit 1, got $unconfigured_code"
case "$unconfigured" in
  *"nothing configured yet"*) ;;
  *) fail "unconfigured ask should explain setup, got: $unconfigured" ;;
esac
echo "  unconfigured ask -> explains setup, exits 1"

echo "smoke: all checks passed"
