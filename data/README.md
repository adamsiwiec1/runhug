# Bundled Search Index

This directory contains a pre-built SQLite search index of Hugging Face models. The index ships with the package to provide instant search out-of-the-box.

## Usage

The CLI automatically uses this bundled index when:
1. No user-local index exists at `~/.config/runhug-cli/models.db`
2. The bundled index is found (relative to executable or in working directory)

## Updating

Users can create their own fresh index with:

```bash
# Create/update user-local index with latest models
runhug-cli update
```

Once a user-local index exists, it takes precedence over the bundled index.

## Search Hierarchy

1. **User-local index** (`~/.config/runhug-cli/models.db`) — highest priority
2. **Bundled index** (`data/models.db`) — fallback if no user index
3. **No index** — search tells you to run `runhug-cli update` (or `init`). It does **not** call the Hub.
4. **`--online` / `--hub`** — optional live Hub search (rate-limited; set `HF_TOKEN`)

## Contents

- `models.db`: SQLite database with ~800+ diverse text-generation models
- Includes: model ID, tags, likes, downloads, license, library, description
- Size: ~500 KB
- Coverage: Popular models + model families (Llama, Qwen, Mistral, Phi, Gemma, DeepSeek, Yi) + GGUF + Safetensors
- Updated: Periodically with package releases

## Building

To rebuild the bundled index:

```bash
# From repo root
./bin/runhug-cli update --force
cp ~/.config/runhug-cli/models.db data/models.db
git add data/models.db
git commit -m "Update bundled search index"
```

## Benefits

✅ Instant search (no network, no setup required)
✅ Offline capable (works without HF API)
✅ Privacy (no API calls for search)
✅ Small footprint (~100 KB)
✅ Users can still get latest models via `update`
