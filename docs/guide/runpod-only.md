# Why deploy stays RunPod-only

Hugging Face is used for **Hub search** and optional **embeddings** — not GPU deploy.

HF Inference Endpoints bill while ready, with a long default idle window and multi-minute cold starts. RunPod serverless bills per second with a short idle window, which matches “run it for pennies,” and offers a wider cheap-GPU catalog.

A second provider would mean dual auth, deploy/list/proxy shapes, docs, and support for a path people can already get with `hf` endpoints if they need Hub-native billing.

If demand shows up later (single HF token, org billing, PrivateLink), an opt-in `runhug deploy --provider hf` is the shape — not dual-by-default.
