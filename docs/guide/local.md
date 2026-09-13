# Local names → Hub search

`local add` lists models already on this machine so you can search Hugging
Face for the same family.

```bash
runhug-cli local add
runhug-cli local add --pick 1
runhug-cli search -q gemma4 --sort likes
```

`--pick 1` on `gemma4:e4b` queries the Hub for `gemma4` and prints likes,
downloads, and `https://huggingface.co/<id>`.

Bare `local add` scans `~/models`, `~/gguf`, `~/.ollama/models`, Hugging Face
and LM Studio caches, `$RVP_CACHE`, and `$RVP_MODELS`.

`--register`, `--all`, `--gguf`, and `--url` still write the optional registry
if you need that later. They are not the search path.

After a local pull, `proxy` exposes `http://127.0.0.1:8080/v1` for any
OpenAI-compatible client.

```bash
runhug-cli local add --gguf ~/models/model.Q4_K_M.gguf --name my-local --register
```
