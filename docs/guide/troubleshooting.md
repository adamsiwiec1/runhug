# Troubleshooting

## OpenAI hangs / times out

1. Check endpoint type: QUEUE workers need `https://api.runpod.ai/v2/{id}/openai/v1`.
2. `{id}.api.runpod.ai` is for **LOAD_BALANCER** only (and use `/v1`, not `/openai/v1`).
3. `/health` 200 with hung OpenAI often means a bad worker image/type mismatch — redeploy with current `runhug deploy` (QUEUE default).

## Deploy created the wrong type

Older tips defaulted to `LOAD_BALANCER` while shipping the queue vLLM worker. Tip on `fix/runpod-v2-endpoint-schema` defaults to **QUEUE**. Pass `--endpoint-type` only when you mean it.

## Search feels empty

```bash
runhug update --packs
runhug update
```

Packs ship as GitHub Release assets, not in git.

## Auth

- `RUNPOD_API_KEY` / `HF_TOKEN` override stored keys.
- `runhug disconnect` / `runhug disconnect hf` forget stored credentials.
