#!/usr/bin/env python3
"""runhug heretic training pod.

Runs the heretic CLI headlessly, appends its output (ANSI-stripped) to
/workspace/runhug/training.log, mirrors live trial progress into
/workspace/runhug/status.json, and serves the dashboard on DASHBOARD_PORT.

Environment variables (all optional unless noted):
  HERETIC_MODEL                Hugging Face model id (required)
  HERETIC_ACTION               "upload" (default when HERETIC_UPLOAD_REPO set) or "save"
  HERETIC_TRIALS               override --n-trials (default 200)
  HERETIC_UPLOAD_REPO          org/repo to push the decensored model to
  HERETIC_UPLOAD_PRIVATE       "1" to create the repo private
  HERETIC_UPLOAD_REPRODUCIBILITY  full | basic | none (default none)
  HERETIC_SAVE_DIR             local save dir (default /output/<model>-heretic)
  HERETIC_ARGS                 extra flags passed through to heretic (shlex)
  HF_TOKEN                     Hugging Face access token (required for upload)
  DASHBOARD_PORT               uvicorn port (default 8080)
  RUNHUH_TOKEN                 optional shared secret for the dashboard
  RUNHUH_KEEP_ALIVE_MIN        minutes to keep the pod alive after training (default 30)
  RUNHUH_MAX_RUNTIME_MIN       hard stop if training runs longer (default 720)
"""

import json
import os
import re
import shlex
import subprocess
import sys
import threading
import time
from pathlib import Path

STATE_DIR = Path("/workspace/runhug")
LOG_PATH = STATE_DIR / "training.log"
STATUS_PATH = STATE_DIR / "status.json"

MAX_LINES = 20000


ANSI_RE = re.compile(r"\x1b\[[0-9;?]*[ -/]*[@-~]")
TRIAL_RE = re.compile(r"^Running trial (\d+) of (\d+)")
PARAM_RE = re.compile(r"^\s*\* ([\w.\-\[\]<>/ ]+) = (.+)$")
REFUSALS_RE = re.compile(r"^\s*\* Refusals:\s*(\d+)\s*/\s*(\d+)")
KL_RE = re.compile(r"^\s*\* KL divergence:\s*(-?[\d.]+)")


def strip_ansi(text):
    return ANSI_RE.sub("", text)


class State:
    """Thread-safe shared state between the heretic subprocess reader and dashboard."""

    def __init__(self):
        self.lock = threading.Lock()
        self.lines = []
        self.total = 0
        self.done = False
        self.model = os.environ.get("HERETIC_MODEL", "")
        self.action = os.environ.get("HERETIC_ACTION", "")
        self.upload_repo_id = os.environ.get("HERETIC_UPLOAD_REPO", "")
        self.save_dir = os.environ.get("HERETIC_SAVE_DIR", "")
        self.started = time.time()
        self.status = "starting"
        self.message = ""
        self.error = ""
        self.trials = []
        self.trials_total = 0
        self.refusals_total = 0
        self.best_refusals = -1
        self.best_kl = float("inf")
        self.current = None

    def addline(self, line):
        with self.lock:
            self.lines.append(line)
            self.total += 1
            if len(self.lines) > MAX_LINES:
                del self.lines[: len(self.lines) - MAX_LINES]

    def snapshot(self):
        with self.lock:
            return list(self.lines), self.total

    def finalize(self):
        cur = self.current
        self.current = None
        if not cur or cur["index"] is None:
            return
        refusals = cur.get("refusals")
        kl = cur.get("kl")
        with self.lock:
            rec = {
                "index": cur["index"],
                "refusals": refusals if refusals is not None else 0,
                "kl": kl if kl is not None else 0.0,
                "params": " ".join(
                    f"{k}={v}" for k, v in sorted(cur["params"].items())
                ) or "",
            }
            self.trials.append(rec)
            self.trials_done = len(self.trials)
            if refusals is not None:
                if self.best_refusals < 0 or refusals < self.best_refusals or (
                    refusals == self.best_refusals and (kl or 0) < self.best_kl
                ):
                    self.best_refusals = refusals
            if kl is not None and kl < self.best_kl:
                self.best_kl = kl

    def parse(self, line):
        m = TRIAL_RE.search(line)
        if m:
            self.finalize()
            self.trials_total = int(m.group(2))
            self.current = {"index": int(m.group(1)), "params": {}, "refusals": None, "kl": None}
            return
        cur = self.current
        if cur is None:
            return
        pm = PARAM_RE.match(line)
        if pm:
            name = pm.group(1).strip()
            value = pm.group(2).strip()
            if len(value) > 160:
                value = value[:160] + "…"
            cur["params"][name] = value
            return
        rm = REFUSALS_RE.match(line)
        if rm:
            cur["refusals"] = int(rm.group(1))
            self.refusals_total = int(rm.group(2))
            return
        km = KL_RE.match(line)
        if km:
            cur["kl"] = float(km.group(1))


def parse_done(line, state):
    """Add a line to the log and parse trial metrics from it."""
    state.addline(line)
    state.parse(line)


def status_json(state):
    elapsed = int(time.time() - state.started)
    with state.lock:
        best_refusals = state.best_refusals
        refusals_total = state.refusals_total
        best_kl = state.best_kl
        trials = list(state.trials)
    return {
        "model": state.model,
        "status": state.status,
        "message": state.message,
        "trials_done": len(trials),
        "trials_total": state.trials_total,
        "best_refusals": best_refusals if best_refusals >= 0 else 0,
        "refusals_total": refusals_total or 100,
        "best_kl": round(best_kl, 6) if best_kl != float("inf") else 0.0,
        "trials": trials,
        "upload_repo_id": state.upload_repo_id,
        "output_path": state.save_dir,
        "elapsed_sec": elapsed,
        "error": state.error,
        "done": state.done,
    }


def write_status(state):
    tmp = STATUS_PATH.with_suffix(".tmp")
    tmp.write_text(json.dumps(status_json(state), indent=2))
    os.replace(tmp, STATUS_PATH)


def model_slug(model_id):
    return re.sub(r"[^a-z0-9-]+", "-", model_id.lower()).strip("-")[:64] or "model"


def build_command(state):
    model = os.environ.get("HERETIC_MODEL", "").strip()
    if not model:
        raise SystemExit("HERETIC_MODEL is required")
    state.model = model

    action = (os.environ.get("HERETIC_ACTION") or "").strip().lower()
    if not action:
        action = "upload" if os.environ.get("HERETIC_UPLOAD_REPO") else "save"
    state.action = action

    trials = os.environ.get("HERETIC_TRIALS", "200").strip()
    save_dir = os.environ.get("HERETIC_SAVE_DIR")
    if not save_dir:
        save_dir = f"/output/{model_slug(model)}-heretic"
    state.save_dir = save_dir

    cmd = ["heretic", "--model", model, "--model-action", action, "--n-trials", trials]
    # Bypass the post-training interactive menus on a non-TTY (prompt_toolkit
    # raises EOFError when stdin is not a terminal): auto-pick the best trial
    # and continue any existing study checkpoint (resume interrupted runs).
    cmd += ["--checkpoint-action", os.environ.get("HERETIC_CHECKPOINT_ACTION", "continue")]
    cmd += ["--trial-index", os.environ.get("HERETIC_TRIAL_INDEX", "0")]
    # Export strategy: merge the LoRA into a full model (do not prompt).
    cmd += ["--export-strategy", os.environ.get("HERETIC_EXPORT_STRATEGY", "MERGE")]
    if action == "upload":
        repo = os.environ.get("HERETIC_UPLOAD_REPO", "").strip()
        if not repo:
            raise SystemExit("HERETIC_ACTION=upload requires HERETIC_UPLOAD_REPO")
        state.upload_repo_id = repo
        private = os.environ.get("HERETIC_UPLOAD_PRIVATE") in ("1", "true", "True", "yes")
        cmd += ["--upload-repo-id", repo]
        cmd += ["--upload-repo-private", "true" if private else "false"]
        cmd += [
            "--upload-reproducibility-information",
            os.environ.get("HERETIC_UPLOAD_REPRODUCIBILITY", "none").strip(),
        ]
    else:
        cmd += ["--save-directory", save_dir]

    extra = os.environ.get("HERETIC_ARGS", "").strip()
    if extra:
        cmd += shlex.split(extra)
    return cmd


def stream(proc, state):
    with proc.stdout as stream_out:
        for raw in stream_out:
            line = strip_ansi(raw).rstrip("\r\n")
            parse_done(line, state)
            print(line, flush=True)
            with open(LOG_PATH, "a") as fh:
                fh.write(line + "\n")

    with open(LOG_PATH, "a") as fh:
        fh.write(f"[runhug] process exited with status {proc.returncode}\n")
    print(f"[runhug] process exited with status {proc.returncode}", flush=True)
    state.addline(f"[runhug] process exited with status {proc.returncode}")


def start_dashboard(state):
    port = int(os.environ.get("DASHBOARD_PORT", "8080"))

    def run():
        try:
            import uvicorn

            from dashboard import app
        except Exception as exc:  # pragma: no cover - lazy path
            state.message = f"dashboard failed to start: {exc}"
            print(f"[runhug] dashboard error: {exc}", flush=True)
            return
        uvicorn.run(app, host="0.0.0.0", port=port, log_level="warning")

    threading.Thread(target=run, daemon=True).start()


def main():
    os.makedirs(STATE_DIR, exist_ok=True)
    os.makedirs("/output", exist_ok=True)
    state = State()
    state.status = "starting"
    write_status(state)

    start_dashboard(state)
    print("[runhug] dashboard started", flush=True)

    try:
        cmd = build_command(state)
    except SystemExit as exc:
        state.done = True
        state.error = str(exc)
        state.status = "error"
        write_status(state)
        print(f"[runhug] {exc}", flush=True)
        time.sleep(15)
        os._exit(1)

    print("[runhug] " + " ".join(shlex.quote(c) for c in cmd), flush=True)
    state.status = "training"
    write_status(state)

    proc = subprocess.Popen(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1,
    )
    reader = threading.Thread(target=stream, args=(proc, state), daemon=True)
    reader.start()

    max_runtime = int(os.environ.get("RUNHUH_MAX_RUNTIME_MIN", "720"))
    deadline = time.time() + max_runtime * 60

    status_interval = 5.0
    while True:
        if proc.poll() is not None:
            break
        if time.time() > deadline:
            state.error = f"training exceeded RUNHUH_MAX_RUNTIME_MIN={max_runtime}"
            state.status = "error"
            write_status(state)
            proc.terminate()
            try:
                proc.wait(timeout=30)
            except subprocess.TimeoutExpired:
                proc.kill()
            break
        time.sleep(status_interval)
        write_status(state)

    reader.join()
    state.finalize()

    if not state.error and proc.returncode != 0:
        state.error = f"heretic exited with status {proc.returncode}"
    state.done = True
    state.status = "error" if state.error else "done"
    write_status(state)
    print(f"[runhug] {'FAILED: ' + state.error if state.error else 'done'}", flush=True)

    keep_alive = int(os.environ.get("RUNHUH_KEEP_ALIVE_MIN", "30"))
    if not state.error and keep_alive > 0:
        # Stay up after success so the dashboard and any artifacts remain visible.
        for i in range(keep_alive * 12):  # 5s ticks
            time.sleep(5)
        os._exit(0)
    os._exit(1 if state.error else 0)


if __name__ == "__main__":
    main()