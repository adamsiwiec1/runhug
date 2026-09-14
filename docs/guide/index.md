# What this is

**Find the best model. Deploy it in minutes. Run it for pennies.**

A Go CLI to run [Hugging Face](https://huggingface.co/docs/hub/en/api) models
on [Runpod](https://docs.runpod.io/serverless/vllm/get-started) easily: find
the best models for a use case via NLP/vector search, deploy in minutes, and
list/manage endpoints — faster than the UI. Pull locally when you want.

```bash
runhug-cli init
runhug-cli search -q qwen --sort likes --engine vllm
runhug-cli inspect Qwen/Qwen2.5-7B-Instruct
runhug-cli connect
runhug-cli deploy Qwen/Qwen2.5-7B-Instruct
runhug-cli list
runhug-cli proxy
```

`init` sets up search NLP: Ollama `nomic-embed-text` and/or `connect hf`
for embeddings, plus an optional local index refresh. No local chat model
is required for search. Optional `--model` / `--search` still installs a
local serve model if you want one.

`search` queries the local SQLite index (`data/models.db` or
`~/.config/runhug-cli/models.db`) and does not call the Hub. Run
`update` to refresh the index from Hugging Face. `--online` / `--hub`
is a live Hub search (rate-limited; set `HF_TOKEN`). Optional embedding
rerank uses local Ollama `nomic-embed-text` (`--keyword` / `--no-semantic`
to skip). Default task is auto/`any` so image and specialty queries are
not forced onto text-generation LLMs.

`connect` prints the Runpod API keys URL
([https://console.runpod.io/user/credentials?tab=api-key](https://console.runpod.io/user/credentials?tab=api-key);
it does not open a browser), then stores the key you paste under
`~/.config/runhug-cli/` (`runpod.key`, mode 0600). Prior
`runpod-vllm-proxy` config dirs are migrated on load. There is no OAuth for
this CLI; `RUNPOD_API_KEY` still works and is preferred when set.

Community files follow
[foss-template](https://github.com/adamsiwiec1/foss-template) from adam-foss.

## Install

```bash
go build -o bin/runhug-cli ./cmd/runhug-cli
```

Or:

```bash
go install github.com/adamsiwiec1/runhug-cli/cmd/runhug-cli@latest
```

## Next

- [Local names](./local.md) — scan this machine, then search the Hub
- [Runpod vLLM](./runpod.md) — connect, deploy, list, proxy
