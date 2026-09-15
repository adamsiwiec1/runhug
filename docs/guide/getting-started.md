# Install & first deploy

## Install

### Go (`@latest`)

```bash
go install github.com/adamsiwiec1/runhug-cli/cmd/runhug-cli@latest
```

**`go install` drops the binary as `runhug-cli`**; curl/`install.sh` installs as `runhug`. Optional: `ln -sf "$(go env GOPATH)/bin/runhug-cli" "$(go env GOPATH)/bin/runhug"`.

### Binary (curl)

Release assets are bare binaries (`runhug-cli_<version>_<os>_<arch>`; Windows `.exe`). Prefer the install script (maps `uname` → asset, installs as `runhug`):

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/adamsiwiec1/runhug-cli/main/scripts/install.sh | bash
```

**Windows (amd64, PowerShell):**

```powershell
irm https://raw.githubusercontent.com/adamsiwiec1/runhug-cli/main/scripts/install.ps1 | iex
```

Exact names for `v0.1.4-beta.3`:

```bash
# macOS arm64
curl -fsSL -o runhug https://github.com/adamsiwiec1/runhug-cli/releases/download/v0.1.4-beta.3/runhug-cli_0.1.4-beta.3_darwin_arm64
chmod +x runhug && sudo mv runhug /usr/local/bin/runhug

# Linux amd64
curl -fsSL -o runhug https://github.com/adamsiwiec1/runhug-cli/releases/download/v0.1.4-beta.3/runhug-cli_0.1.4-beta.3_linux_amd64
chmod +x runhug && sudo mv runhug /usr/local/bin/runhug
```

```powershell
Invoke-WebRequest -Uri https://github.com/adamsiwiec1/runhug-cli/releases/download/v0.1.4-beta.3/runhug-cli_0.1.4-beta.3_windows_amd64.exe -OutFile runhug.exe
```

### From source (optional)

```bash
git clone https://github.com/adamsiwiec1/runhug-cli.git
cd runhug-cli
go build -o bin/runhug-cli ./cmd/runhug-cli
```

## Wizard (recommended)

```bash
runhug-cli wizard
```

Walks init → connect → recommend → `deploy --dry-run` → optional live deploy. No live endpoint without confirm.

## Manual path

```bash
runhug-cli init --yes
runhug-cli connect                 # RunPod API key
runhug-cli connect hf              # optional HF token
runhug-cli search -q "small instruct"
runhug-cli deploy Qwen/Qwen2.5-0.5B-Instruct --dry-run
runhug-cli deploy Qwen/Qwen2.5-0.5B-Instruct   # QUEUE by default
runhug-cli list
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
