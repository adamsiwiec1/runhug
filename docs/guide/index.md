# What runhug is

**Find the best model. Deploy it in minutes. Run it for pennies.**

`runhug` is a Go CLI that:

1. Searches Hugging Face models (local index + optional NLP embeddings)
2. Deploys a RunPod serverless vLLM endpoint (default **QUEUE**)
3. Gives you an OpenAI-compatible base URL to chat or drive agents

```bash
go build -o bin/runhug ./cmd/runhug
runhug wizard
```

Next: [Install & first deploy](./getting-started.md)
