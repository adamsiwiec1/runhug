# Changelog

All notable user-facing changes to runhug-cli are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Approximate serverless **cost estimate** block on `wizard` GPU options, `recommend gpu`, and `deploy --dry-run`: $/hr while up, cold-start range from weight size, $/cold vs $/warm request, and daily scenarios (10/100/1000 req, all-warm / 10% cold / all-cold). Labeled as estimates only.
- `wizard` GPU step lists fitting serverless pools (recommended cheapest fit + larger/safer options with VRAM, example GPU, $/hr, stock); user picks by number and that pool is passed to dry-run / live `deploy --gpu`. New `runpod.ListFitting` / `FittingOptions` helpers.

### Fixed
- `wizard` Find-a-model shortlist searches the **raw user query** with `--sort likes` and `--no-semantic` (no `ParseIntent` → `"instruct"` rewrite, no `recommend.Score` known-publisher re-rank). Prints likes/downloads like `search`. `recommend --no-llm` uses the same likes-ordered lexical pool.

## [0.1.2] - 2026-09-14

### Added
- `wizard` (aliases: `guide`, `guided`, `setup`): interactive guided setup from search stack → connect → find model → dry-run deploy; `--yes` / non-TTY prints a checklist and never live-deploys.
- Configurable Hub update limit: `update --limit N`, `RUNHUG_UPDATE_LIMIT`, `config set update_limit` (default 2000; 0=unlimited). New Hub models require likes≥3 and downloads≥100; existing ids always refresh.
- `recommend` command: local shortlist + optional OpenAI-compatible advisor; Suggested GPU per candidate; `recommend gpu <model>`.
- Settings: `advisor_base_url`, `advisor_model`. Documented `models.id` as the unique HF repo id primary key.

## [0.1.1] - 2026-09-14

### Changed
- Index pack builds no longer default-cap at 5000 rows; `--limit 0` / unset `RUNHUG_INDEX_LIMIT` means unlimited.
- Pack builder filters Hub models to `likes >= 3` and `downloads >= 100` (configurable via `--min-likes` / `--min-downloads`), with early-stop when paging by downloads.

### Added
- `ListOpts.MinLikes` / `MinDownloads` with downloads-desc early exit when a whole page is below the download floor.

## [Unreleased]

### Added
- Category SQLite **index packs** on GitHub Releases (`index-manifest.json`, `index-<category>.db`); `init` multi-select install; `update` watermark deltas / `update --packs`; `cmd/build-index-packs` + release workflow.


- Default `search` uses the local/bundled SQLite index only and never calls the Hub when an index exists. Live Hub search is opt-in via `--online` / `--hub` (rate-limited; set `HF_TOKEN`). If no index exists, search tells you to run `update` or `init` instead of hitting the Hub.
- Restored search table **ACTIONS** column (🔗 OSC-8 Hub link, 📋 plain) and footer hint for `copy N` / `search --copy N`.
- Search is NLP/embeddings-only: removed local chat re-rank (`localllm` / `recommend.Rerank` chat path).
- `init` sets up nomic-embed-text / `connect hf` + optional index (no Qwen chat starter).
- Default Hub task is `auto`/`any` with image/audio intent detection; `--keyword` aliases `--no-semantic`.

### Changed
- Rebranded CLI to **runhug-cli** (module `github.com/adamsiwiec1/runhug-cli`, binary `runhug-cli`)
- Config directory is now XDG `~/.config/runhug-cli` (honors `XDG_CONFIG_HOME`); migrates from prior `runpod-vllm-proxy` locations on load
- Prefer `RUNHUG_CONFIG` to override registry path; `RVP_CONFIG` still accepted during transition

### Fixed
- `connect` Authorization header sanitization strips BOM/non-ASCII clipboard junk

## [Unreleased]

### Added

- `search -q` / `--query` is the same as a positional query (`--query`
  wins if both are set). Search matches model card descriptions as well
  as repo id/tags, and expands a small alias map (`hacking` → `pentest`,
  …) as extra Hub `search=` calls before local scoring.
- `search --wrap N` / `--word-wrap N` / `-ww N` wraps MODEL names at N
  runes across multiple lines (clamped 12–80; 0 or omitted = ellipsis
  truncate at 48). Other columns stay on the first line only.
- `search --copy N` copies the MODEL id for row N to the clipboard.
- Semantic rerank of that candidate pool when an embedder is available:
  local Ollama `nomic-embed-text` (or similar), else Hugging Face Inference
  `sentence-transformers/all-MiniLM-L6-v2` if `HF_TOKEN` is set. The Hub
  has no public semantic model-search API. `--semantic` (default when
  an embedder exists) / `--no-semantic`. Chat instruct models are not
  used as embedders.
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

- `search --sort` is `relevance` (default; Hub text search, `sort` omitted),
  `likes`, or `downloads`. Popularity sorts re-rank an expanded 100-hit
  relevance pool (aliases + card descriptions) locally instead of asking
  the Hub to sort by likes or downloads.
- `search` accepts `--license` (`apache-2.0`, `mit`, `gemma`, `other`, …)
  and `--engine` (`vllm`, `gguf`, …).
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
