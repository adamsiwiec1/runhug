# What this is

A Go CLI that searches [Hugging Face](https://huggingface.co/docs/hub/en/api),
pulls a model locally, and deploys it to
[Runpod Serverless vLLM](https://docs.runpod.io/serverless/vllm/get-started).

```bash
runpod-vllm-proxy init
runpod-vllm-proxy search -q qwen --sort likes --engine vllm
runpod-vllm-proxy inspect Qwen/Qwen2.5-7B-Instruct
runpod-vllm-proxy connect
runpod-vllm-proxy deploy Qwen/Qwen2.5-7B-Instruct
runpod-vllm-proxy list
runpod-vllm-proxy proxy
```

`init` recommends **Qwen/Qwen2.5-1.5B-Instruct** to run locally. Say no and
it lets you search the Hub, or pass `--model` / `--search`.

`connect` prints the Runpod API keys URL
([https://console.runpod.io/user/credentials?tab=api-key](https://console.runpod.io/user/credentials?tab=api-key);
it does not open a browser), then stores the key you paste in the config
directory (mode 0600). There is no OAuth for this CLI; `RUNPOD_API_KEY`
still works and is preferred when set.

Community files follow
[foss-template](https://github.com/adamsiwiec1/foss-template) from adam-foss.

## Install

```bash
go build -o bin/runpod-vllm-proxy ./cmd/runpod-vllm-proxy
```

Or:

```bash
go install github.com/adamsiwiec1/runpod-vllm-proxy/cmd/runpod-vllm-proxy@latest
```

## Next

- [Local names](./local.md) — scan this machine, then search the Hub
- [Runpod vLLM](./runpod.md) — connect, deploy, list, proxy
