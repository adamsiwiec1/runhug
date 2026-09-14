# runhug-cli

**Find the best model. Deploy it in minutes. Run it for pennies.**

CLI to run [Hugging Face](https://huggingface.co) models on
[Runpod](https://docs.runpod.io/serverless/vllm/get-started) easily: find the
best models for a use case via NLP/vector search, deploy in minutes, and
list/manage endpoints — faster than the UI. Pull locally when you want; one
OpenAI-compatible proxy either way.

Commands stay in the terminal: **search**, **init**, **connect**, **deploy**,
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

`init` installs a local runtime if needed (Ollama by default) and offers the
**default local starter** **Qwen/Qwen2.5-1.5B-Instruct** (`qwen2.5:1.5b`) —
small official Instruct chat model that fits a laptop. Enter to accept, or
`n` to decline and search the Hub / pick something else (`--model`, `--search`).

```bash
runhug-cli init --yes
runhug-cli init --model qwen3:8b
runhug-cli init --search "coding assistant" --pick 1
```

## Search

```bash
runhug-cli search qwen
runhug-cli search -q cybersec
runhug-cli search qwen --sort likes --limit 5
runhug-cli search instruct --sort downloads --engine vllm
runhug-cli search "instruct coder" --engine gguf --license apache-2.0
runhug-cli inspect Qwen/Qwen2.5-7B-Instruct
```

`-q` / `--query` is the same as a positional query. If both are set,
`--query` wins.

Search matches **repo id, tags, pipeline tag, and the model card
description** (Hub `full=true`, plus a bounded card `Get` when the list
omits it — no weight downloads). Relevance is local token overlap;
description and tag hits are boosted, then likes break ties.

The Hub `search=` API mostly matches repo id / author, so a short query
also fires a few extra list calls (max 4): documented aliases, distinctive
tokens, and sometimes `task=any` or a tag `filter=`. Aliases:

- `hacking` / `hack` → also `pentest`, `offsec`, `cybersecurity`, `bug hunter`
- `offsec` → also `pentest`, `red team`, `cyber`
- `cybersec` → also `cybersecurity`

`--sort` is `relevance` (default; Hub `sort` omitted), `likes`, or
`downloads`. Popularity sorts still build that expanded pool (up to 100
after union), then re-rank locally — they do not ask the Hub to sort by
likes. `--limit` is how many rows to show (default 15). `--engine`
accepts `vllm`, `gguf`, or any other engine string; `--license` matches
Hub tags (`apache-2.0`, `mit`, `gemma`, `other` for empty or uncommon
licenses). The table may shorten MODEL; `--word-wrap` / `-ww` prints the
full repo id. **Next:** always uses the full repo id.

The Hub has **no public semantic model-search API**. Website search and
`GET /api/models?search=` are lexical (repo id / author; cards are a
separate full-text index). After that recall, `--sort relevance` can
re-rank the pool with embeddings: local Ollama `nomic-embed-text` if
it is already pulled, otherwise Hugging Face Inference
`sentence-transformers/all-MiniLM-L6-v2` when `HF_TOKEN` is set.
`--semantic` is on when an embedder is available; `--no-semantic` keeps
lexical scoring. Chat models such as Qwen2.5-1.5B-Instruct are not used
as embedders.

### Local index (not a Hub-wide vector DB)

What ships today:

- A **SQLite** local model index (`data/models.db`, or a user copy under
  `~/.config/runhug-cli/models.db`)
- Optional **embedding rerank** (`internal/semantic`) via Ollama
  `nomic-embed-text` or Hugging Face Inference

This is **not** a full Hub-wide vector database. Semantic search means local
index + optional embeddings on the candidate pool. Refresh the index with:

```bash
runhug-cli update
runhug-cli update --force   # rebuild from Hub
runhug-cli update --cli     # print how to upgrade the CLI itself
```

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
runhug-cli local add --pick 1        # search the Hub for that name
```

`local add --pick 1` on `gemma4:e4b` searches Hugging Face for `gemma4`.
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
| `search` | Hugging Face search |
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

- Hub: `GET /api/models` (lexical `search=`; no semantic model-search API), `GET /api/models/{org}/{name}`, and `GET /api/whoami-v2` for token verify.
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
