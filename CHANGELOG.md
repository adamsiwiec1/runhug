# Changelog

All notable user-facing changes to runpod-vllm-proxy are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Interactive REPL mode: run `runpod-vllm-proxy` with no args (when stdin is a TTY) or `runpod-vllm-proxy interactive` to start a `runpod-vllm-proxy>` prompt with commands: `search`, `copy N`, `inspect`, `deploy`, `connect`, `status`, `list`, `help`, `quit`

### Changed
- `search` expands natural-language queries via local LLM (when running) or heuristics, then ranks Hub hits (not literal name-only match)
- Search now preserves distinctive raw query terms in Hub searches (e.g., "chat heretic cybersecurity" includes those terms, not just generic "instruct")
- Search scoring weights raw query term matches higher (60%) to surface more relevant results for niche queries
- ACTIONS column: 🔗 opens Hub (clickable link), 📋 is plain text (use `copy N` command or `search --copy N` to copy model id)
- Search table adds an ACTIONS column (🔗 Hub link, 📋 compact link)
- `search` takes `-q` / `--query` instead of a positional query
- `search --sort` defaults to `relevance`; `likes` / `downloads` re-rank a 100-hit relevance pool
- `search` gains `--license` and `--engine` (`vllm`, `gguf`) filters


### Fixed
- `connect` Authorization header sanitization strips BOM/non-ASCII clipboard junk
- Go module path aligned to `github.com/adamsiwiec1/runpod-vllm-proxy` (matches the GitHub repo)

## [Unreleased]

### Added

- `connect` / `disconnect` persist a Runpod API key in the user config dir
  (`runpod.key`, mode 0600). `connect` prints
  https://console.runpod.io/user/credentials?tab=api-key (does not open a
  browser), then prompts for a key (hidden, never printed). `--key` skips
  the prompt. Already connected: reports source without reprinting the
  secret and offers to replace. The key is verified with
  `GET /v2/serverless` before save. `config.Load()` reads `RUNPOD_API_KEY`
  first, then the stored key.
- `proxy` — OpenAI proxy on `127.0.0.1:8080/v1`. `serve` remains an alias.
- `list` includes account Serverless endpoints when connected, marked
  **ours** (registry) vs other account endpoints. `deployments` is an alias.
- ANSI colors with a `NO_COLOR` / non-TTY fallback (no extra dependency).

### Changed

- The product is Hub search, local pull (`init`), and Runpod deploy/list/proxy.
- `list` fetches remote endpoints automatically when a key is available
  (`--local` skips that). Next steps after init/deploy point at search,
  inspect, connect, deploy, and proxy.
- Hub tables are aligned columns; likes/downloads and engine tags are colored
  when the terminal allows it.

### Removed

- `chat` (REPL and one-shot).
- `quickstart` (use `search`).

### Docs

- README and VitePress guide lead with init, search, connect, deploy, list,
  proxy.
