# Local & HF connect

## Hugging Face token

```bash
runhug connect hf     # aliases: login hf, hf login
```

Used for gated Hub downloads and optional Inference embeddings. `HF_TOKEN` overrides the stored token.

## Models already on this machine

```bash
runhug local add
runhug local add --pick 3
runhug local setup          # Ollama / llama.cpp / MLX hints
```

## Local serve (optional)

Search does **not** need a local chat model. For local inference:

```bash
runhug init --model <ollama-tag-or-hub-id>
```
