---
layout: home

hero:
  name: runhug-cli
  text: Find. Deploy. Run.
  tagline: Find the best Hugging Face model. Deploy it on Runpod in minutes. Run it for pennies — faster than the UI.
  actions:
    - theme: brand
      text: Get started
      link: /guide/
    - theme: alt
      text: Runpod
      link: /guide/runpod

features:
  - title: Search
    details: NLP search over Hub/index (`-q` / `--query`) — repo id, tags, card descriptions, plus embedding rerank (nomic-embed-text or HF Inference). No local chat model required. Default task is auto/any (not forced text-generation).
  - title: Search setup
    details: `init` prepares the embedder + optional index (`nomic-embed-text` / `connect hf` / `update`). Optional `--model` installs a local serve model.
  - title: Deploy
    details: `connect` prints the API keys URL and saves a key. `deploy` creates one Serverless vLLM endpoint. `list` and `proxy` finish the loop.
---
