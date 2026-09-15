# runhug

<p align="center">
  <img src="docs/public/hero.svg" alt="runhug — Find the best model. Deploy it in minutes. Run it for pennies." width="100%"/>
</p>

**Find the best model. Deploy it in minutes. Run it for pennies.**

Docs: [adamsiwiec1.github.io/runhug](https://adamsiwiec1.github.io/runhug/) (VitePress in `docs/`)

## Install

### Go (`@latest`)

```bash
go install github.com/adamsiwiec1/runhug/cmd/runhug@latest
```

Requires Go on your `PATH`. Drops the binary as **`runhug`** in `$(go env GOPATH)/bin` or `GOBIN`.

### Binary (curl)

Release assets are **bare binaries** (not archives). **Next release** names them `runhug_<version>_<os>_<arch>` (Windows: `.exe`). Older tags may still use the `runhug-cli_` prefix — `install.sh` / `install.ps1` try `runhug_` first, then fall back to `runhug-cli_`.

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/adamsiwiec1/runhug/main/scripts/install.sh | bash
```

Installs to `/usr/local/bin/runhug` when writable, otherwise `~/.local/bin/runhug`.

**Windows (amd64, PowerShell):**

```powershell
irm https://raw.githubusercontent.com/adamsiwiec1/runhug/main/scripts/install.ps1 | iex
```

Exact asset URLs after the next release (swap the tag/version when it ships). Until then, replace `runhug_` with `runhug-cli_` for current assets:

```bash
# macOS Apple Silicon
curl -fsSL -o runhug https://github.com/adamsiwiec1/runhug/releases/download/vX.Y.Z/runhug_X.Y.Z_darwin_arm64
chmod +x runhug && sudo mv runhug /usr/local/bin/runhug

# macOS Intel
curl -fsSL -o runhug https://github.com/adamsiwiec1/runhug/releases/download/vX.Y.Z/runhug_X.Y.Z_darwin_amd64
chmod +x runhug && sudo mv runhug /usr/local/bin/runhug

# Linux amd64
curl -fsSL -o runhug https://github.com/adamsiwiec1/runhug/releases/download/vX.Y.Z/runhug_X.Y.Z_linux_amd64
chmod +x runhug && sudo mv runhug /usr/local/bin/runhug

# Linux arm64
curl -fsSL -o runhug https://github.com/adamsiwiec1/runhug/releases/download/vX.Y.Z/runhug_X.Y.Z_linux_arm64
chmod +x runhug && sudo mv runhug /usr/local/bin/runhug
```

```powershell
# Windows amd64
Invoke-WebRequest -Uri https://github.com/adamsiwiec1/runhug/releases/download/vX.Y.Z/runhug_X.Y.Z_windows_amd64.exe -OutFile runhug.exe
```

Releases: [github.com/adamsiwiec1/runhug/releases](https://github.com/adamsiwiec1/runhug/releases)

### From source (optional)

```bash
git clone https://github.com/adamsiwiec1/runhug.git
cd runhug
go build -o bin/runhug ./cmd/runhug
```

## What it does

1. **Hugging Face search** — find models that fit your use case
2. **RunPod serverless vLLM** — deploy an endpoint in minutes
3. **OpenAI-compatible URL** — chat from any OpenAI client

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
| Site | [GitHub Pages](https://adamsiwiec1.github.io/runhug/) |
| Source | `docs/` — `npm ci && npm run docs:dev` |
| Contributing | [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md) |

Deploy stays **RunPod-only** for now (HF is Hub search + embeddings). See [Why RunPod-only](docs/guide/runpod-only.md).

## License

See [LICENSE](LICENSE).
