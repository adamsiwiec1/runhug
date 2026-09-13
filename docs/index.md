---
layout: home

hero:
  name: runpod-vllm-proxy
  text: Search, pull, deploy
  tagline: Hugging Face search, a local pull, and Runpod Serverless vLLM — from the terminal.
  actions:
    - theme: brand
      text: Get started
      link: /guide/
    - theme: alt
      text: Runpod
      link: /guide/runpod

features:
  - title: Search
    details: `search` hits the Hugging Face API (relevance by default). Filter by `--engine` / `--license`; likes and downloads re-rank a relevance pool.
  - title: Pull locally
    details: `init` installs a runtime and the default model, or `--model` / `--search` to pick your own.
  - title: Deploy
    details: `connect` prints the API keys URL and saves a key. `deploy` creates one Serverless vLLM endpoint. `list` and `proxy` finish the loop.
---
