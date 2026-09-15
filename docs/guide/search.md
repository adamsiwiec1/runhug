# Search & recommend

## Search

```bash
runhug search -q "cheap chat llm"
runhug search -q qwen --sort likes --engine vllm
runhug search -q "text to image" --online   # live Hub (needs HF_TOKEN)
```

Default search is **local SQLite** only. Refresh with `runhug update`. Embeddings (Ollama `nomic-embed-text` or HF Inference) improve ranking; `--keyword` skips them.

## Recommend

```bash
runhug recommend -q "cheap chat on a small GPU"
runhug recommend gpu Qwen/Qwen2.5-0.5B-Instruct
runhug inspect Qwen/Qwen2.5-0.5B-Instruct
```

Shortlist + optional LLM advisor + GPU pool hints. Model id is the Hugging Face repo id.
