# runpod-vllm-proxy

Search [Hugging Face](https://huggingface.co), pull a model locally, and
deploy it to [Runpod Serverless vLLM](https://docs.runpod.io/serverless/vllm/get-started).
The CLI stays in the terminal: **search**, **init**, **connect**, **deploy**,
**list**, **proxy**.

## Install

```bash
go install github.com/adamsiwiec1/runpod-vllm-proxy/cmd/runpod-vllm-proxy@latest
```

From a clone:

```bash
git clone https://github.com/adamsiwiec1/runpod-vllm-proxy.git
cd runpod-vllm-proxy
go build -o bin/runpod-vllm-proxy ./cmd/runpod-vllm-proxy
```

## First run

```bash
runpod-vllm-proxy init
```

`init` installs a local runtime if needed (Ollama by default) and recommends
**Qwen/Qwen2.5-1.5B-Instruct** (`qwen2.5:1.5b`). Enter to accept, or `n` to
search the Hub and pick something else.

```bash
runpod-vllm-proxy init --yes
runpod-vllm-proxy init --model qwen3:8b
runpod-vllm-proxy init --search "coding assistant" --pick 1
```

## Search

```bash
runpod-vllm-proxy search qwen --sort likes
runpod-vllm-proxy search qwen --limit 3
runpod-vllm-proxy search "instruct coder" --filter gguf
runpod-vllm-proxy inspect Qwen/Qwen2.5-7B-Instruct
```

## Connect to Runpod

Runpod's public API is a Bearer key (no OAuth for this CLI). `connect` prints
[https://console.runpod.io/user/credentials?tab=api-key](https://console.runpod.io/user/credentials?tab=api-key)
(it does not open a browser). Paste a key; it is saved next to the registry
(`runpod.key`, mode `0600`) and never printed. `RUNPOD_API_KEY` still wins when
set.

```bash
runpod-vllm-proxy connect
runpod-vllm-proxy connect --key "$RUNPOD_API_KEY"
runpod-vllm-proxy disconnect
```

If you are already connected, `connect` says so without reprinting the secret
and offers to replace the stored key. A rejected key is not saved.

`HF_TOKEN` is only for gated Hub models and is not stored.

## Deploy, list, proxy

```bash
runpod-vllm-proxy search qwen
runpod-vllm-proxy deploy Qwen/Qwen2.5-7B-Instruct
runpod-vllm-proxy list
runpod-vllm-proxy proxy
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
runpod-vllm-proxy local add                 # list Ollama / GGUF on disk
runpod-vllm-proxy local add --pick 1        # search the Hub for that name
```

`local add --pick 1` on `gemma4:e4b` searches Hugging Face for `gemma4`.
Bare `local add` scans `~/models`, `~/gguf`, `~/.ollama/models`, Hugging Face /
LM Studio caches, `$RVP_CACHE`, and `$RVP_MODELS`.

## Environment

| Variable | Required | Purpose |
| --- | --- | --- |
| `HF_TOKEN` | gated or private Hub models | Hugging Face token (never printed, not stored) |
| `RUNPOD_API_KEY` | if you have not run `connect` | [API key](https://docs.runpod.io/get-started/api-keys) |
| `RVP_CONFIG` | no | Override registry path (key file is stored beside it) |
| `RVP_MODELS` | no | Extra directories to scan for GGUF |
| `RVP_CACHE` | no | Override GGUF cache |
| `NO_COLOR` | no | Disable ANSI colors |

## Commands

| Command | What it does |
| --- | --- |
| `init` | Runtime + default local model (or `--model` / `--search`) |
| `search` | Hugging Face search |
| `inspect` | Hub card + params + VRAM |
| `connect` / `disconnect` | Print the API keys URL, then save or forget the key |
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

- Hub: `GET /api/models` and `GET /api/models/{org}/{name}`.
- Runpod management: [API v2](https://docs.runpod.io/api-reference-v2/overview).
- Worker image is pinned to [worker-vllm v2.27.0](https://github.com/runpod-workers/worker-vllm/releases/tag/v2.27.0).
- Runpod also ships an agent plugin for pods and `runpodctl`:
  [runpod-plugins-official](https://github.com/runpod/runpod-plugins-official).
- Colors follow [NO_COLOR](https://no-color.org/) and stay off when stdout is not a TTY.

## License

[MIT](LICENSE) © 2026 Adam Siwiec. The Code of Conduct text is
[Contributor Covenant 2.1](https://www.contributor-covenant.org/version/2/1/code_of_conduct.html)
(CC-BY-4.0).
