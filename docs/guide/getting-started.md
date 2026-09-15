# Install & first deploy

## Install

### Go (`@latest`)

```bash
go install github.com/adamsiwiec1/runhug/cmd/runhug@latest
```

Binary name: **`runhug`**.

### Binary (curl)

Release assets are bare binaries. **Next release:** `runhug_<version>_<os>_<arch>` (Windows `.exe`). Older tags may still use `runhug-cli_` — install scripts try the new prefix first, then fall back.

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/adamsiwiec1/runhug/main/scripts/install.sh | bash
```

**Windows (amd64, PowerShell):**

```powershell
irm https://raw.githubusercontent.com/adamsiwiec1/runhug/main/scripts/install.ps1 | iex
```

After the next release (until then, use `runhug-cli_` for current assets):

```bash
# macOS arm64
curl -fsSL -o runhug https://github.com/adamsiwiec1/runhug/releases/download/vX.Y.Z/runhug_X.Y.Z_darwin_arm64
chmod +x runhug && sudo mv runhug /usr/local/bin/runhug

# Linux amd64
curl -fsSL -o runhug https://github.com/adamsiwiec1/runhug/releases/download/vX.Y.Z/runhug_X.Y.Z_linux_amd64
chmod +x runhug && sudo mv runhug /usr/local/bin/runhug
```

```powershell
Invoke-WebRequest -Uri https://github.com/adamsiwiec1/runhug/releases/download/vX.Y.Z/runhug_X.Y.Z_windows_amd64.exe -OutFile runhug.exe
```

### From source (optional)

```bash
git clone https://github.com/adamsiwiec1/runhug.git
cd runhug
go build -o bin/runhug ./cmd/runhug
```

## Wizard (recommended)

```bash
runhug wizard
```

Walks init → connect → recommend → `deploy --dry-run` → optional live deploy. No live endpoint without confirm.

## Manual path

```bash
runhug init --yes
runhug connect                 # RunPod API key
runhug connect hf              # optional HF token
runhug search -q "small instruct"
runhug deploy Qwen/Qwen2.5-0.5B-Instruct --dry-run
runhug deploy Qwen/Qwen2.5-0.5B-Instruct   # QUEUE by default
runhug list
```

## OpenAI base URL (QUEUE)

```
https://api.runpod.ai/v2/{endpoint_id}/openai/v1
```

Load balancer (if you opt in with `--endpoint-type LOAD_BALANCER`):

```
https://{endpoint_id}.api.runpod.ai/v1
```

Next: [Search & recommend](./search.md) · [RunPod deploy](./runpod.md)
