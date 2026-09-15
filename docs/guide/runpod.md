# RunPod deploy

## Connect

```bash
runhug connect
```

Stores the key at `~/.config/runhug/runpod.key` (0600). `RUNPOD_API_KEY` wins when set. Gated Hub weights need `HF_TOKEN`.

## Deploy

```bash
runhug deploy <model> --dry-run
runhug deploy <model>                 # --endpoint-type QUEUE (default)
runhug list
runhug proxy                          # http://127.0.0.1:8080/v1
```

Workers default to **min=0**. One endpoint per model. Sizing uses Hub safetensors metadata + cheapest in-stock pool.

## URLs that work

| Type | OpenAI base |
| --- | --- |
| QUEUE (default) | `https://api.runpod.ai/v2/{id}/openai/v1` |
| LOAD_BALANCER | `https://{id}.api.runpod.ai/v1` |

Do **not** use `{id}.api.runpod.ai/...` for QUEUE — RunPod rejects that for queue workers.

## Chat & agents

```bash
runhug run [model]
runhug start claude              # Anthropic→OpenAI bridge for Claude Code
runhug start claude --no-launch  # print env only
```
