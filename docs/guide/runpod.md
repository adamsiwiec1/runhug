# Runpod vLLM

Runpod authenticates with an API key. There is no public OAuth/device-code
for third-party CLIs. `connect` prints
[https://console.runpod.io/user/credentials?tab=api-key](https://console.runpod.io/user/credentials?tab=api-key)
(it does not open a browser), then saves the key beside the registry
(`runpod.key`, mode 0600) and never prints it. `RUNPOD_API_KEY` is used if set.
Gated Hub weights also need `HF_TOKEN` (not stored).

```bash
runhug-cli connect
runhug-cli search qwen2.5 instruct
runhug-cli inspect Qwen/Qwen2.5-7B-Instruct
runhug-cli deploy Qwen/Qwen2.5-7B-Instruct
runhug-cli list
runhug-cli proxy
```

`inspect` / `deploy` size VRAM from Hub safetensors metadata and pick the
cheapest in-stock serverless pool. One endpoint per model. Workers default
to min=0.

`list` shows the local registry plus account endpoints when connected.
Deployments this tool created are marked **ours**.

`proxy` (alias `serve`) listens on `http://127.0.0.1:8080/v1`.

```bash
curl http://127.0.0.1:8080/v1/models
```

```python
from openai import OpenAI
client = OpenAI(api_key="unused", base_url="http://127.0.0.1:8080/v1")
client.models.list()
```

Switch with `use` or the request `model` field. `disconnect` deletes the
stored key.
