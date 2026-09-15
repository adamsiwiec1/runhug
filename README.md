# runhug

**Find the best model. Deploy it in minutes. Run it for pennies.**

Search Hugging Face models, deploy a RunPod serverless vLLM endpoint, and talk to it over an OpenAI-compatible URL — from the terminal.

Full docs: [runhug on GitBook](https://devrecated-solutions.gitbook.io/runhug/)  
Draft CR (until merged): [change request #1](https://app.gitbook.com/o/8ztJRg1pbAuXx8CHt3LF/s/nficyebsa9VUNbdhNhxO/~/changes/Jy96Sn3R3Bo16Qhgry8y/)

## Install

```bash
git clone https://github.com/adamsiwiec1/runhug-cli.git
cd runhug-cli
go build -o bin/runhug ./cmd/runhug
```

Binary: `runhug`. Module path is `github.com/adamsiwiec1/runhug`; the GitHub repo is still `runhug-cli` until renamed.

## First run

```bash
runhug wizard          # guided setup (no live deploy without confirm)
runhug init --yes      # search NLP + optional index packs
runhug connect         # RunPod API key
runhug connect hf      # optional Hub / embeddings token
```

## Everyday commands

```bash
runhug search -q "small instruct llm"
runhug recommend -q "cheap chat on a small GPU"
runhug deploy <model> --dry-run
runhug deploy <model>              # default endpoint type: QUEUE
runhug list
runhug proxy
runhug run [model]                 # chat
runhug start claude                # point Claude Code at the endpoint
```

QUEUE OpenAI base: `https://api.runpod.ai/v2/{id}/openai/v1`  
Load balancer OpenAI base: `https://{id}.api.runpod.ai/v1`

## Docs

| What | Where |
| --- | --- |
| Guides & reference | [GitBook](https://devrecated-solutions.gitbook.io/runhug/) |
| Local VitePress (contributors) | `docs/` — `npm ci && npm run docs:dev` |
| Contributing / security | [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md) |

Deploy stays **RunPod-only** for now (HF is Hub search + embeddings). See the GitBook product note for why.

## License

See [LICENSE](LICENSE).
