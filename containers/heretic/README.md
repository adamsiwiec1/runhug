# runhug heretic

Container image for `runhug heretic make <org/model>`.

It boots a FastAPI dashboard on port **8080** and runs the
[heretic](https://github.com/p-e-w/heretic) (abliteration) optimizer headlessly
against a Hugging Face model. Trial progress is mirrored into `status.json` and
the raw log for the CLI and the dashboard page.

## Usage

```bash
runhug heretic make Qwen/Qwen2.5-7B-Instruct --trials 200
```

The CLI creates a RunPod GPU pod with this image, follows the pod until the
training dashboard is reachable, and streams trial lines (refusals / KL
divergence) into the terminal as they happen. If a Hugging Face token is
configured, the decensored model is uploaded to
`<hf-user>/heretic-<model>-<variant>` when done; the pod keeps running for a
short grace period so the dashboard stays viewable.

## Environment variables

| Variable | Default | Meaning |
| --- | --- | --- |
| `HERETIC_MODEL` | – | Hugging Face model id (required) |
| `HERETIC_ACTION` | `save` | `upload` or `save` |
| `HERETIC_TRIALS` | `200` | optuna trial count |
| `HERETIC_UPLOAD_REPO` | – | target repo (required for `upload`) |
| `HERETIC_UPLOAD_PRIVATE` | off | `1` to create the repo private |
| `HERETIC_UPLOAD_REPRODUCIBILITY` | `none` | `full`, `basic`, or `none` |
| `HERETIC_SAVE_DIR` | `/output/<model>-heretic` | local save path |
| `HERETIC_ARGS` | – | extra flags passed to `heretic` (shlex) |
| `HF_TOKEN` | – | token for gated models / upload |
| `DASHBOARD_PORT` | `8080` | uvicorn port |
| `RUNHUH_TOKEN` | – | optional dashboard auth secret |
| `RUNHUH_KEEP_ALIVE_MIN` | `30` | minutes to stay up after success |
| `RUNHUH_MAX_RUNTIME_MIN` | `720` | hard stop for runaway training |
| `RUNHUH_ERROR_DELAY_S` | `15` | seconds before exiting on error (keep-alive grace) |
| `HERETIC_CHECKPOINT_ACTION` | `continue` | `continue`, `restart`, or `null` |
| `HERETIC_TRIAL_INDEX` | `0` | Pareto-front index of the trial to export |
| `HERETIC_EXPORT_STRATEGY` | `MERGE` | `merge` or `adapter` |

## Tests

Unit tests for `run_pipeline.py` and `dashboard.py` live in `tests/` and run on
a bare `python3` (the dashboard tests need fastapi/uvicorn/jinja2, which are
already in the image):

```bash
python3 -m unittest discover -s tests -v
# or, against the built image (includes the dashboard runtime deps):
docker run --rm -v "$PWD":/src -w /src --entrypoint python runhug-heretic:latest -m unittest discover -s tests -v
```

## Dashboard

Once the pod is `RUNNING`, open the proxy URL printed by the CLI:

- `/` — live chart (refusals vs KL divergence per trial) + log tail
- `/status.json` — raw status & metrics
- `/logs?offset=N` — new log lines; the CLI `follow` loop consumes this
- `/health`

If `RUNHUH_TOKEN` is set, all endpoints except `/health` expect
`?token=<token>` (or an `Authorization: Bearer <token>` header).

## Building locally

```bash
docker build -t ghcr.io/<your-user>/runhug-heretic:latest containers/heretic
```

CI pushes the image on every change under `containers/heretic` (see
`.github/workflows/build-heretic-image.yml`).