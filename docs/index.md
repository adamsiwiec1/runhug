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
    details: `search` (`-q` / `--query`) hits the Hugging Face API and scores repo id, tags, and model card descriptions. Semantic rerank uses local nomic-embed-text or HF Inference when available. Filter by `--engine` / `--license`; likes and downloads re-rank an expanded relevance pool.
  - title: Pull locally
    details: `init` installs a runtime and the default model, or `--model` / `--search` to pick your own.
  - title: Deploy
    details: `connect` prints the API keys URL and saves a key. `deploy` creates one Serverless vLLM endpoint. `list` and `proxy` finish the loop.
---
