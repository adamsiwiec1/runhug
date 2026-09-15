# Install & first deploy

## Install

```bash
git clone https://github.com/adamsiwiec1/runhug-cli.git
cd runhug-cli
go build -o bin/runhug ./cmd/runhug
```

Binary name: `runhug`.

## Wizard (recommended)

```bash
runhug wizard
```

Walks init → connect → recommend → `deploy --dry-run` → optional live deploy. No live endpoint without confirm.

## Manual path

```bash
runhug init --yes
runhug connect                 # RunPod API key
runhug connect hf              # optional HF token
runhug search -q "small instruct"
runhug deploy Qwen/Qwen2.5-0.5B-Instruct --dry-run
runhug deploy Qwen/Qwen2.5-0.5B-Instruct   # QUEUE by default
runhug list
```

## OpenAI base URL (QUEUE)

```
https://api.runpod.ai/v2/{endpoint_id}/openai/v1
```

Load balancer (if you opt in with `--endpoint-type LOAD_BALANCER`):

```
https://{endpoint_id}.api.runpod.ai/v1
```

Next: [Search & recommend](./search.md) · [RunPod deploy](./runpod.md)
