# runhug-cli

**Find the best model. Deploy it in minutes. Run it for pennies.**

CLI to run [Hugging Face](https://huggingface.co) models on
[Runpod](https://docs.runpod.io/serverless/vllm/get-started) easily: find the
best models for a use case via NLP/vector search, deploy in minutes, and
list/manage endpoints — faster than the UI. Pull locally when you want; one
OpenAI-compatible proxy either way.

Commands stay in the terminal: **search**, **recommend**, **init**, **connect**, **deploy**,
**list**, **proxy**.

## Install

```bash
go install github.com/adamsiwiec1/runhug-cli/cmd/runhug-cli@latest
```

From a clone:

```bash
git clone https://github.com/adamsiwiec1/runhug-cli.git
cd runhug-cli
go build -o bin/runhug-cli ./cmd/runhug-cli
```

## First run

```bash
runhug-cli init
```

`init` sets up the **search NLP stack** (not a local chat model): detect Ollama
and offer `ollama pull nomic-embed-text` for embedding rerank, or guide
`connect hf` for Hugging Face Inference embeddings. Optionally install **category index packs** from GitHub Releases
(multi-select) or refresh via `update`. Search works without an embedder
(lexical only); embeddings improve ranking.

```bash
runhug-cli init --yes
runhug-cli connect hf
runhug-cli update
# optional local serve model (not required for search):
runhug-cli init --model <ollama-tag-or-hub-id>
```

## Category index packs

Search uses a **local SQLite** database. Large category packs ship as
**GitHub Release assets** (not in git). A small starter `data/models.db`
remains in-repo for offline smoke tests.

| Pack id | Contents |
| --- | --- |
| `text-generation` | LLMs (`pipeline_tag=text-generation`) |
| `text-to-image` | Image models |
| `video` | `text-to-video` + `image-to-video` |
| `audio` | TTS + ASR |
| `gguf` | `filter=gguf` |

**Approach:** downloaded packs are stored under
`~/.config/runhug-cli/packs/<id>.db` (provenance) and **merged** into
`~/.config/runhug-cli/models.db` with idempotent upserts so search keeps a
single DB path.

```bash
runhug-cli init          # multi-select categories → download + merge
runhug-cli update        # deltas: Hub lastModified > watermark (or delta JSONL)
runhug-cli update --packs  # re-check Releases; full pack replace if needed
```

Pack assets on a release:

- `index-manifest.json` — id, title, pipeline/filter, rows, size_bytes, sha256, db_filename, watermark
- `index-<category>.db` — SQLite (`models` + `metadata`, same schema as today)
- optional `index-<category>-delta.jsonl` — new/changed rows since prior watermark

Override the release repo with `RUNHUG_PACKS_REPO=owner/name` (default
`adamsiwiec1/runhug-cli`). Build packs locally:

```bash
export HF_TOKEN=…          # recommended
export RUNHUG_INDEX_LIMIT=5000   # per-category cap (v1 samples/top-N)
go run ./cmd/build-index-packs --out dist/index --limit 5000
# or: ./scripts/build-index-packs --out dist/index
```

CI uploads packs on `release` published and via `workflow_dispatch`
(`.github/workflows/release-index-packs.yml`). First real upload can be a
manual dispatch onto an existing tag.

## Search

`search` queries the **local SQLite index** (`~/.config/runhug-cli/models.db`,
or the bundled `data/models.db`). It does **not** call the Hugging Face Hub
API. Refresh the index from the Hub with `update` (or `init`). Live Hub
search is opt-in:

```bash
runhug-cli search -q "top penetration testing models" --limit 5
runhug-cli search -q "animated cartoon generation models" --limit 5
runhug-cli search -q "heretic uncensored image models" --limit 5
runhug-cli search qwen --sort likes --limit 5
runhug-cli search instruct --sort downloads --engine vllm
runhug-cli search "instruct coder" --engine gguf --license apache-2.0
runhug-cli search -q cybersec --keyword
runhug-cli search --online -q "offsec hacking model" --sort likes   # live Hub; rate-limited
runhug-cli inspect Qwen/Qwen2.5-7B-Instruct
```

If no local or bundled index exists, search prints a message to run
`runhug-cli update` or `runhug-cli init` instead of hitting the Hub.
`--online` / `--hub` forces a live Hub request (rate-limited; set
`HF_TOKEN` to raise limits and reach gated listings).

`-q` / `--query` is the same as a positional query. If both are set,
`--query` wins.

Search matches **repo id, tags, pipeline tag, and the model card
description** already stored in the index. Relevance is local token overlap;
description and tag hits are boosted, then likes break ties. Alias terms
expand the local query (not extra Hub calls):

- `hacking` / `hack` → also `pentest`, `offsec`, `cybersecurity`, `bug hunter`
- `offsec` → also `pentest`, `red team`, `cyber`
- `cybersec` → also `cybersecurity`

`--sort` is `relevance` (default), `likes`, or `downloads`. Popularity
sorts re-rank the **same local candidate pool** (up to 100 hits), then
`--limit` rows are shown (default 15). They do not re-query the Hub.
`--engine` accepts `vllm`, `gguf`, or any other engine string; `--license`
matches tags (`apache-2.0`, `mit`, `gemma`, `other` for empty or uncommon
licenses). The table may shorten MODEL (ellipsis); `--wrap 28` /
`--word-wrap N` / `-ww N` wraps MODEL across lines at N runes (other
columns stay on the first line). **ACTIONS** offers 🔗 (open Hub) and 📋
(copy via `search --copy N` or REPL `copy N`). **Next:** always uses the
full repo id.

`--sort relevance` can re-rank the local pool with embeddings: local Ollama
`nomic-embed-text` if it is already pulled (cached vectors stay local).
`--semantic` is on by default when an embedder is available; `--keyword` /
`--no-semantic` keep lexical scoring only. No local chat model is required
for search — chat completions are not used for ranking. Default
`pipeline_tag` is **auto** (image/cartoon/diffusion/audio intents map to a
task; otherwise `any`).

### Local index (not a Hub-wide vector DB)

What ships today:

- A **SQLite** local model index (`data/models.db`, or a user copy under
  `~/.config/runhug-cli/models.db`)
- Optional **embedding rerank** (`internal/semantic`) via Ollama
  `nomic-embed-text`

This is **not** a full Hub-wide vector database. Semantic search means local
index + optional embeddings on the candidate pool. The Hub is contacted only
by `update` (refresh) or `search --online` / `--hub`. Refresh the index with:

```bash
runhug-cli update
runhug-cli update --force   # rebuild from Hub
runhug-cli update --cli     # print how to upgrade the CLI itself
```

## Recommend

Ask which model fits a use case. Shortlist comes from the **local index**
(same path as `search`); an optional OpenAI-compatible chat call compares
**only those candidates**. Default advisor is local Ollama
(`http://127.0.0.1:11434/v1`). Each candidate includes a **Suggested GPU**
(Runpod pool + ~$/hr) from live `gpus` catalog when connected, else an
offline estimate.

The SQLite unique key is `models.id` (Hugging Face repo id, e.g. `org/name`).

```bash
runhug-cli recommend "best model for RAG on a 16GB laptop"
runhug-cli recommend -q "…" --candidates 8
runhug-cli recommend --no-llm "coding on a laptop"   # scored shortlist + GPU only
runhug-cli recommend --base-url http://127.0.0.1:11434/v1 --model llama3.2 "…"
runhug-cli recommend --base-url https://api.openai.com/v1 --api-key-env OPENAI_API_KEY --model gpt-4o-mini "…"
runhug-cli recommend gpu Qwen/Qwen2.5-7B-Instruct    # GPU / VRAM for one model
```

Settings: `advisor_base_url`, `advisor_model` (API keys only via env —
never printed).

## Update limit

Hub delta upserts on `update` are capped (default **2000**; `0` = unlimited):

```bash
runhug-cli update --limit 5000
runhug-cli config set update_limit 0
export RUNHUG_UPDATE_LIMIT=1000
```

Precedence: `--limit` > `RUNHUG_UPDATE_LIMIT` > `settings.json` `update_limit` > 2000.
**New** Hub models need `likes ≥ 3` and `downloads ≥ 100`; existing ids always
refresh metadata.

## Connect to Runpod

Runpod's public API is a Bearer key (no OAuth for this CLI). `connect` prints
[https://console.runpod.io/user/credentials?tab=api-key](https://console.runpod.io/user/credentials?tab=api-key)
(it does not open a browser). Paste a key; it is saved under
`~/.config/runhug-cli/` next to the registry (`runpod.key`, mode `0600`) and
never printed. Existing configs from `runpod-vllm-proxy` are migrated on load.
`RUNPOD_API_KEY` still wins when set.

```bash
runhug-cli connect
runhug-cli connect --key "$RUNPOD_API_KEY"
runhug-cli disconnect
```

If you are already connected, `connect` says so without reprinting the secret
and offers to replace the stored key. A rejected key is not saved.

`HF_TOKEN` is for gated Hub models and optional Inference embeddings. Env still
wins when set; otherwise a stored `hf.token` (mode `0600`) from `connect hf` /
`login hf` is used.

```bash
runhug-cli connect hf
runhug-cli login hf
runhug-cli disconnect hf
runhug-cli config
runhug-cli config set no_color true
```

## Deploy, list, proxy

```bash
runhug-cli search qwen
runhug-cli deploy Qwen/Qwen2.5-7B-Instruct
runhug-cli list
runhug-cli proxy
```

One Serverless endpoint per model. Workers default to min=0. `list` shows the
local registry and, when connected, account endpoints (marked **ours** if this
tool created them). `proxy` (alias: `serve`) exposes
`http://127.0.0.1:8080/v1`.

```bash
curl http://127.0.0.1:8080/v1/models
```

## Models already on this machine

```bash
runhug-cli local add                 # list Ollama / GGUF on disk
runhug-cli local add --pick 1        # search the local index for that name
```

`local add --pick 1` on `gemma4:e4b` searches the local index for `gemma4`.
Bare `local add` scans `~/models`, `~/gguf`, `~/.ollama/models`, Hugging Face /
LM Studio caches, `$RVP_CACHE`, and `$RVP_MODELS`.

## Environment

| Variable | Required | Purpose |
| --- | --- | --- |
| `HF_TOKEN` | gated or private Hub models; optional Inference embeddings | Hugging Face token (never printed; wins over stored `hf.token`) |
| `RUNPOD_API_KEY` | if you have not run `connect` | [API key](https://docs.runpod.io/get-started/api-keys) |
| `RUNHUG_CONFIG` | no | Override registry path (key file beside it; `RVP_CONFIG` still accepted) |
| `RVP_MODELS` | no | Extra directories to scan for GGUF |
| `RVP_CACHE` | no | Override GGUF cache |
| `NO_COLOR` | no | Disable ANSI colors |

## Commands

| Command | What it does |
| --- | --- |
| `init` | Runtime + default local model (or `--model` / `--search`) |
| `search` | Local SQLite index search (`--online` / `--hub` for live Hub) |
| `inspect` | Hub card + params + VRAM |
| `connect` / `disconnect` | Runpod API key URL, then save or forget |
| `connect hf` / `login hf` / `disconnect hf` | Hugging Face token (`hf.token`, 0600) |
| `config` / `config get|set` | Paths + `no_color` in `settings.json` |
| `update` | Refresh local SQLite search index (`update --cli` for CLI install tips) |
| `deploy` | Serverless vLLM |
| `list` | Local registry + Runpod deployments when connected |
| `proxy` | OpenAI proxy on localhost (`serve` is an alias) |
| `use` / `url` / `status` / `delete` / `gpus` / `import` | Registry and account |
| `local add` | Disk / Ollama scan → Hub search |

## Docs and contributing

Community files follow
[foss-template](https://github.com/adamsiwiec1/foss-template) (adam-foss).
MIT, Contributor Covenant 2.1, private vulnerability reporting.

- Guide: [docs/guide](docs/guide/index.md)
- [CONTRIBUTING.md](CONTRIBUTING.md) · [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- [SECURITY.md](SECURITY.md) · [SUPPORT.md](SUPPORT.md) · [GOVERNANCE.md](GOVERNANCE.md)
- [CHANGELOG.md](CHANGELOG.md)

```bash
make test
make build
npm ci && npm run docs:build   # VitePress, Node 22, contributors / CI only
```

## Notes

- Search defaults to the local SQLite index. Hub `GET /api/models` is used by `update` and `search --online` / `--hub` only (lexical `search=`; rate-limited). `inspect` uses `GET /api/models/{org}/{name}`; `GET /api/whoami-v2` verifies tokens.
- Config dir: `~/.config/runhug-cli/` (`runpod.key`, `hf.token`, `settings.json`, optional `models.db`).
- Runpod management: [API v2](https://docs.runpod.io/api-reference-v2/overview).
- Worker image is pinned to [worker-vllm v2.27.0](https://github.com/runpod-workers/worker-vllm/releases/tag/v2.27.0).
- Runpod also ships an agent plugin for pods and `runpodctl`:
  [runpod-plugins-official](https://github.com/runpod/runpod-plugins-official).
- Colors follow [NO_COLOR](https://no-color.org/) and stay off when stdout is not a TTY.

## License

[MIT](LICENSE) © 2026 Adam Siwiec. The Code of Conduct text is
[Contributor Covenant 2.1](https://www.contributor-covenant.org/version/2/1/code_of_conduct.html)
(CC-BY-4.0).
