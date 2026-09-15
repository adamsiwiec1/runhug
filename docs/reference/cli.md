# CLI map

```
runhug <command> [flags]
```

## Setup

| Command | What it does |
| --- | --- |
| `wizard` | Interactive guided setup (`guide`, `guided`, `setup`) |
| `init` | Search NLP + optional index packs |
| `connect` | Save RunPod API key |
| `connect hf` | Save Hugging Face token |
| `disconnect [hf]` | Forget stored key(s) |

## Search & index

| Command | What it does |
| --- | --- |
| `search` | Local index; `--online` / `--hub` for live Hub |
| `recommend` | Shortlist + optional advisor |
| `recommend gpu <m>` | GPU / VRAM suggestion for one model |
| `inspect <model>` | Hub card + VRAM estimate |
| `update` | Refresh index / packs |

## RunPod

| Command | What it does |
| --- | --- |
| `deploy <model>` | Serverless vLLM (default QUEUE, min=0) |
| `list` | Registry + account endpoints |
| `proxy` | Local OpenAI proxy `127.0.0.1:8080/v1` |
| `gpus` / `import` / `delete` / `use` / `url` / `status` | Endpoint helpers |

## Chat & agents

| Command | What it does |
| --- | --- |
| `run [model]` | Interactive chat (`-q` one-shot) |
| `start <agent>` | `claude` via Anthropic→OpenAI bridge; `codex` stub |

## Local

| Command | What it does |
| --- | --- |
| `local add` | Register models on this machine |
| `local setup` | Ollama / llama.cpp / MLX |

## Config & env

`config`, `config get|set` — `no_color`, `update_limit`, advisor URL/model.

Important env: `HF_TOKEN`, `RUNPOD_API_KEY`, `RUNHUG_CONFIG`, `RUNHUG_PACKS_REPO`, `RUNHUG_UPDATE_LIMIT`, `NO_COLOR`.
