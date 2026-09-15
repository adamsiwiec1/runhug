# Category index packs

runhug search is **local-first**. Category packs extend the local SQLite
index beyond the small in-repo starter DB.

## Install

```bash
runhug init
```

When offered, pick categories (numbers, ranges, `all`, or `none`). Packs
download from the latest GitHub Release of `adamsiwiec1/runhug-cli`
(`RUNHUG_PACKS_REPO` overrides), verify SHA-256 from `index-manifest.json`,
store under `~/.config/runhug/packs/`, and **merge** into
`~/.config/runhug/models.db`.

## Update (deltas)

```bash
runhug update          # prefer delta JSONL, else Hub lastModified > watermark
runhug update --packs  # full pack replace from Releases
runhug update --hub    # Hub scrape only (ignore pack selections)
runhug update --force  # rebuild text-generation scrape from scratch
```

Per-category watermarks live in `models.db` metadata (`pack:<id>:watermark`)
and `packs/installed.json`. Updates upsert; they do not delete the DB.

## Build packs

```bash
HF_TOKEN=… RUNHUG_INDEX_LIMIT=5000 go run ./cmd/build-index-packs --out dist/index
```

Categories: `text-generation`, `text-to-image`, `video`, `audio`, `gguf`.
v1 builds are top-N samples when limited; raise the limit for fuller packs.

## CI

`.github/workflows/release-index-packs.yml` builds on release publish and
`workflow_dispatch`, then `gh release upload`s assets. Schedule refresh later.
