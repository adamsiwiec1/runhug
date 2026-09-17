#!/usr/bin/env python3
"""runhug heretic dashboard.

Read-only FastAPI app that serves trial progress from the files written by
run_pipeline.py. Endpoints:

  GET /            HTML dashboard (Chart.js trial chart + live log tail)
  GET /health      {"ok": true}
  GET /status.json current status / metrics
  GET /logs?offset=N   new log lines after N

If RUNHUH_TOKEN is set, every endpoint except /health requires ?token=<token>
(or an Authorization: Bearer <token> header).
"""

import os
from pathlib import Path

from fastapi import FastAPI, Request, status
from fastapi.responses import HTMLResponse, JSONResponse
from fastapi.templating import Jinja2Templates

STATE_DIR = Path("/workspace/runhug")
STATUS_PATH = STATE_DIR / "status.json"
LOG_PATH = STATE_DIR / "training.log"

TOKEN = os.environ.get("RUNHUH_TOKEN", "")
templates = Jinja2Templates(directory=str(Path(__file__).resolve().parent / "templates"))

app = FastAPI(title="runhug heretic dashboard")


def authorized(request: Request) -> bool:
    if not TOKEN:
        return True
    if request.query_params.get("token") == TOKEN:
        return True
    auth = request.headers.get("authorization", "")
    return auth == f"Bearer {TOKEN}"


def deny() -> JSONResponse:
    return JSONResponse({"error": "unauthorized"}, status_code=status.HTTP_401_UNAUTHORIZED)


def read_status() -> dict:
    try:
        return json_loads(STATUS_PATH)
    except Exception:
        return {}


def json_loads(path: Path) -> dict:
    import json

    return json.loads(path.read_text())


@app.get("/health")
def health() -> dict:
    return {"ok": True}


@app.get("/status.json")
def status_json(request: Request) -> JSONResponse:
    if not authorized(request):
        return deny()
    return JSONResponse(read_status())


@app.get("/logs")
def logs(request: Request, offset: int = 0, n: int = 500) -> JSONResponse:
    if not authorized(request):
        return deny()
    lines: list[str] = []
    if LOG_PATH.exists():
        lines = LOG_PATH.read_text(errors="replace").splitlines()
    total = len(lines)
    start = min(max(offset, 0), total)
    chunk = lines[start : start + n]
    return JSONResponse(
        {
            "lines": chunk,
            "offset": start + len(chunk),
            "total": total,
            "done": bool(read_status().get("done")),
        }
    )


@app.get("/", response_class=HTMLResponse)
def index(request: Request):
    if not authorized(request):
        return deny()
    token = request.query_params.get("token", "")
    return templates.TemplateResponse(
        "index.html",
        {"request": request, "token": token, "model": read_status().get("model", "")},
    )