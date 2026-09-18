"""Unit tests for the runhug heretic training pod runner (run_pipeline.py)."""

import io
import json
import os
import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import run_pipeline as rp

RELEVANT_ENV = [
    "HERETIC_MODEL",
    "HERETIC_ACTION",
    "HERETIC_TRIALS",
    "HERETIC_UPLOAD_REPO",
    "HERETIC_UPLOAD_PRIVATE",
    "HERETIC_UPLOAD_REPRODUCIBILITY",
    "HERETIC_SAVE_DIR",
    "HERETIC_ARGS",
    "HERETIC_CHECKPOINT_ACTION",
    "HERETIC_TRIAL_INDEX",
    "HERETIC_EXPORT_STRATEGY",
    "RUNHUH_KEEP_ALIVE_MIN",
    "RUNHUH_MAX_RUNTIME_MIN",
    "RUNHUH_ERROR_DELAY_S",
    "DASHBOARD_PORT",
]

BANNER = r"""
\x1b[36m█░█░█▀▀░█▀▄░█▀▀░▀█▀░█░█▀▀\x1b[0m
"""


def _clear_env():
    for key in RELEVANT_ENV:
        os.environ.pop(key, None)


class BaseCase(unittest.TestCase):
    def setUp(self):
        self._saved = {key: os.environ.get(key) for key in RELEVANT_ENV}
        _clear_env()

    def tearDown(self):
        _clear_env()
        for key, value in self._saved.items():
            if value is not None:
                os.environ[key] = value


class TestModelSlug(BaseCase):
    def test_lowercases_and_sanitizes(self):
        self.assertEqual(rp.model_slug("Qwen/Qwen2.5-7B-Instruct"), "qwen-qwen2-5-7b-instruct")
        self.assertEqual(rp.model_slug("  Mistral-7B  "), "mistral-7b")
        self.assertEqual(rp.model_slug("/"), "model")
        self.assertEqual(rp.model_slug("a/b/c"), "a-b-c")

    def test_truncates_to_64(self):
        slug = rp.model_slug("org/" + "x" * 100)
        self.assertLessEqual(len(slug), 64)


class TestStripAnsi(BaseCase):
    def test_removes_escape_sequences(self):
        self.assertEqual(rp.strip_ansi("\x1b[36mhello\x1b[0m"), "hello")
        self.assertEqual(rp.strip_ansi("plain text"), "plain text")
        self.assertEqual(rp.strip_ansi("\x1b[1;31mcolored\x1b[m"), "colored")

    def test_banner_cleans_to_ascii(self):
        stripped = rp.strip_ansi(BANNER)
        self.assertNotIn("\x1b[", stripped)


class TestStateParse(BaseCase):
    def test_starts_trial(self):
        st = rp.State()
        st.parse("Running trial 3 of 20")
        self.assertEqual(st.current["index"], 3)
        self.assertEqual(st.trials_total, 20)
        self.assertEqual(st.current["params"], {})

    def test_parses_params_refusals_kl(self):
        st = rp.State()
        st.parse("Running trial 1 of 20")
        st.parse("  * attn.o_proj.max_weight = 0.93")
        st.parse("  * attn.o_proj.max_weight_position = 17.69")
        st.parse("  * Refusals: 55 / 100")
        st.parse("  * KL divergence: 0.0083")
        self.assertEqual(st.current["params"]["attn.o_proj.max_weight"], "0.93")
        self.assertEqual(st.current["params"]["attn.o_proj.max_weight_position"], "17.69")
        self.assertEqual(st.current["refusals"], 55)
        self.assertEqual(st.refusals_total, 100)
        self.assertEqual(st.current["kl"], 0.0083)

    def test_finalizes_on_next_trial(self):
        st = rp.State()
        st.parse("Running trial 2 of 20")
        st.parse("  * Refusals: 72 / 100")
        st.parse("  * KL divergence: 0.0098")
        st.parse("Running trial 3 of 20")
        self.assertEqual(len(st.trials), 1)
        rec = st.trials[0]
        self.assertEqual(rec["index"], 2)
        self.assertEqual(rec["refusals"], 72)
        self.assertEqual(rec["kl"], 0.0098)
        self.assertEqual(st.trials_done, 1)
        self.assertEqual(st.best_refusals, 72)
        self.assertAlmostEqual(st.best_kl, 0.0098, places=4)

    def test_last_refusal_line_wins(self):
        st = rp.State()
        st.parse("Running trial 1 of 20")
        st.parse("  * Refusals: 30 / 100")
        st.parse("  * Refusals: 10 / 100")
        st.parse("  * KL divergence: 0.005")
        st.parse("Running trial 2 of 20")
        self.assertEqual(st.best_refusals, 10)
        self.assertAlmostEqual(st.best_kl, 0.005, places=4)

    def test_finalize_without_metrics(self):
        st = rp.State()
        st.parse("Running trial 1 of 5")
        st.finalize()
        self.assertEqual(st.trials[0]["refusals"], 0)
        self.assertEqual(st.trials[0]["kl"], 0.0)
        self.assertEqual(st.best_refusals, -1)

    def test_tie_breaks_on_kl(self):
        st = rp.State()
        st.parse("Running trial 1 of 20")
        st.parse("  * Refusals: 50 / 100")
        st.parse("  * KL divergence: 0.02")
        st.parse("Running trial 2 of 20")
        st.parse("  * Refusals: 50 / 100")
        st.parse("  * KL divergence: 0.01")
        st.parse("Running trial 3 of 20")
        self.assertEqual(st.best_refusals, 50)
        self.assertAlmostEqual(st.best_kl, 0.01, places=4)

    def test_long_param_truncated(self):
        st = rp.State()
        st.parse("Running trial 1 of 2")
        st.parse("  * big = " + "x" * 300)
        self.assertEqual(len(st.current["params"]["big"]), 161)  # 160 + ellipsis

    def test_metrics_ignored_before_first_trial(self):
        st = rp.State()
        st.parse("  * max_weight = 1.0")
        self.assertIsNone(st.current)

    def test_addline_trims_to_max(self):
        st = rp.State()
        original = rp.MAX_LINES
        rp.MAX_LINES = 5
        try:
            for i in range(12):
                st.addline(str(i))
            self.assertEqual(st.total, 12)
            self.assertEqual(st.lines, ["7", "8", "9", "10", "11"])
        finally:
            rp.MAX_LINES = original

    def test_snapshot_returns_copy(self):
        st = rp.State()
        st.addline("a")
        lines, total = st.snapshot()
        lines.append("b")
        self.assertEqual(len(st.lines), 1)
        self.assertEqual(total, 1)

    def test_parse_done_logs_and_parses(self):
        st = rp.State()
        rp.parse_done("Running trial 1 of 5", st)
        self.assertEqual(st.total, 1)
        self.assertEqual(st.current["index"], 1)


class TestBuildCommand(BaseCase):
    def test_requires_model(self):
        with self.assertRaises(SystemExit):
            rp.build_command(rp.State())

    def test_default_save(self):
        os.environ["HERETIC_MODEL"] = "Qwen/Qwen2.5-7B-Instruct"
        st = rp.State()
        cmd = rp.build_command(st)
        self.assertEqual(st.model, "Qwen/Qwen2.5-7B-Instruct")
        self.assertEqual(st.action, "save")
        self.assertEqual(cmd[cmd.index("--n-trials") + 1], "200")
        self.assertEqual(cmd[cmd.index("--model-action") + 1], "save")
        self.assertEqual(cmd[cmd.index("--checkpoint-action") + 1], "continue")
        self.assertEqual(cmd[cmd.index("--trial-index") + 1], "0")
        self.assertEqual(cmd[cmd.index("--export-strategy") + 1], "MERGE")
        self.assertEqual(
            cmd[cmd.index("--save-directory") + 1],
            "/output/qwen-qwen2-5-7b-instruct-heretic",
        )
        self.assertNotIn("--upload-repo-id", cmd)

    def test_trials_and_save_dir_overrides(self):
        os.environ.update(
            {
                "HERETIC_MODEL": "Org/M",
                "HERETIC_TRIALS": "8",
                "HERETIC_SAVE_DIR": "/tmp/custom-out",
            }
        )
        cmd = rp.build_command(rp.State())
        self.assertEqual(cmd[cmd.index("--n-trials") + 1], "8")
        self.assertEqual(cmd[cmd.index("--save-directory") + 1], "/tmp/custom-out")

    def test_headless_flag_overrides(self):
        os.environ.update(
            {
                "HERETIC_MODEL": "Org/M",
                "HERETIC_CHECKPOINT_ACTION": "restart",
                "HERETIC_TRIAL_INDEX": "3",
                "HERETIC_EXPORT_STRATEGY": "ADAPTER",
            }
        )
        cmd = rp.build_command(rp.State())
        self.assertEqual(cmd[cmd.index("--checkpoint-action") + 1], "restart")
        self.assertEqual(cmd[cmd.index("--trial-index") + 1], "3")
        self.assertEqual(cmd[cmd.index("--export-strategy") + 1], "ADAPTER")

    def test_upload_requires_repo(self):
        os.environ.update({"HERETIC_MODEL": "Org/M", "HERETIC_ACTION": "upload"})
        with self.assertRaises(SystemExit):
            rp.build_command(rp.State())

    def test_upload_action_detected_from_repo(self):
        os.environ.update({"HERETIC_MODEL": "Org/M", "HERETIC_UPLOAD_REPO": "adam/test"})
        st = rp.State()
        cmd = rp.build_command(st)
        self.assertEqual(st.action, "upload")
        self.assertEqual(st.upload_repo_id, "adam/test")
        self.assertEqual(cmd[cmd.index("--upload-repo-id") + 1], "adam/test")
        self.assertEqual(cmd[cmd.index("--upload-repo-private") + 1], "false")
        self.assertEqual(
            cmd[cmd.index("--upload-reproducibility-information") + 1], "none"
        )
        self.assertNotIn("--save-directory", cmd)

    def test_upload_private_and_reproducibility(self):
        os.environ.update(
            {
                "HERETIC_MODEL": "Org/M",
                "HERETIC_ACTION": "upload",
                "HERETIC_UPLOAD_REPO": "adam/test",
                "HERETIC_UPLOAD_PRIVATE": "1",
                "HERETIC_UPLOAD_REPRODUCIBILITY": "full",
            }
        )
        cmd = rp.build_command(rp.State())
        self.assertEqual(cmd[cmd.index("--upload-repo-private") + 1], "true")
        self.assertEqual(
            cmd[cmd.index("--upload-reproducibility-information") + 1], "full"
        )

    def test_extra_args_shlex(self):
        os.environ.update(
            {"HERETIC_MODEL": "Org/M", "HERETIC_ARGS": '--max-shard-size 2GB --foo="a b"'}
        )
        cmd = rp.build_command(rp.State())
        self.assertIn("--max-shard-size", cmd)
        self.assertEqual(cmd[cmd.index("--max-shard-size") + 1], "2GB")
        self.assertIn("--foo=a b", cmd)


class TestStatusJson(BaseCase):
    def test_fresh_defaults(self):
        st = rp.State()
        out = rp.status_json(st)
        self.assertEqual(out["status"], "starting")
        self.assertFalse(out["done"])
        self.assertEqual(out["error"], "")
        self.assertEqual(out["best_refusals"], 0)
        self.assertEqual(out["best_kl"], 0.0)
        self.assertEqual(out["refusals_total"], 100)
        self.assertEqual(out["trials_done"], 0)
        self.assertIsInstance(out["elapsed_sec"], int)
        self.assertEqual(out["output_path"], "")

    def test_after_trials(self):
        st = rp.State()
        st.parse("Running trial 1 of 20")
        st.parse("  * Refusals: 55 / 100")
        st.parse("  * KL divergence: 0.0083")
        st.parse("Running trial 2 of 20")
        st.status = "done"
        st.done = True
        out = rp.status_json(st)
        self.assertEqual(out["trials_done"], 1)
        self.assertEqual(out["trials_total"], 20)
        self.assertEqual(out["best_refusals"], 55)
        self.assertAlmostEqual(out["best_kl"], 0.0083, places=6)
        self.assertEqual(out["trials"][0]["index"], 1)
        self.assertTrue(out["done"])
        self.assertEqual(out["trials"][0]["params"], "")


class TestWriteStatus(BaseCase):
    def test_writes_atomically(self):
        with tempfile.TemporaryDirectory() as tmp:
            status_path = Path(tmp) / "status.json"
            old = rp.STATUS_PATH
            rp.STATUS_PATH = status_path
            try:
                os.environ["HERETIC_MODEL"] = "Org/M"
                st = rp.State()
                rp.write_status(st)
                data = json.loads(status_path.read_text())
                self.assertEqual(data["model"], "Org/M")
                self.assertEqual(data["status"], "starting")
                self.assertFalse((status_path.with_suffix(".tmp")).exists())
            finally:
                rp.STATUS_PATH = old


class _FauxProc:
    def __init__(self, stream, returncode=0):
        self.stdout = stream
        self.returncode = returncode


class TestStream(BaseCase):
    def setUp(self):
        super().setUp()
        self._tmp = tempfile.TemporaryDirectory()
        self._old_status = rp.STATUS_PATH
        self._old_log = rp.LOG_PATH
        rp.STATUS_PATH = Path(self._tmp.name) / "status.json"
        rp.LOG_PATH = Path(self._tmp.name) / "training.log"

    def tearDown(self):
        rp.STATUS_PATH = self._old_status
        rp.LOG_PATH = self._old_log
        self._tmp.cleanup()
        super().tearDown()

    def test_tees_strips_ansi_and_logs(self):
        st = rp.State()
        proc = _FauxProc(
            io.StringIO(
                "\x1b[36mRunning trial 1 of 2\x1b[0m\n"
                "* Refusals: 55 / 100\n"
                "* KL divergence: 0.0033\n"
            ),
            returncode=0,
        )
        buf = io.StringIO()
        with redirect_stdout(buf):
            rp.stream(proc, st)

        log = rp.LOG_PATH.read_text().splitlines()
        self.assertEqual(log[0], "Running trial 1 of 2")
        self.assertIn("[runhug] process exited with status 0", log)

        out = buf.getvalue().splitlines()
        self.assertIn("Running trial 1 of 2", out)
        self.assertIn("[runhug] process exited with status 0", out)

        # stream() feeds parse_done, so metrics are captured too.
        self.assertEqual(st.current["refusals"], 55)
        self.assertEqual(st.current["kl"], 0.0033)


class TestMainIntegration(BaseCase):
    """run_main()/main() wiring against a stub heretic binary."""

    _ENV_KEYS = (
        "PATH",
        "HERETIC_MODEL",
        "HERETIC_TRIALS",
        "HERETIC_SAVE_DIR",
        "RUNHUH_KEEP_ALIVE_MIN",
        "RUNHUH_MAX_RUNTIME_MIN",
        "RUNHUH_ERROR_DELAY_S",
        "DASHBOARD_PORT",
        "RUNHUH_TOKEN",
        "HERETIC_CHECKPOINT_ACTION",
    )

    def _make_stub(self, tmp, rc, block=False):
        stub_dir = Path(tmp) / "bin"
        stub_dir.mkdir(exist_ok=True)
        stub = stub_dir / "heretic"
        body = "import time\ntime.sleep(3600)\n" if block else f"sys.exit({rc})"
        stub.write_text(
            "#!/usr/bin/env python3\n"
            "import sys\n"
            "print('Running trial 1 of 1')\n"
            "print('  * Refusals: 55 / 100')\n"
            "print('  * KL divergence: 0.0033')\n"
            + body
            + "\n"
        )
        stub.chmod(0o755)
        return stub_dir

    def _prepare_env(self, tmp, rc, *, block=False, keep_alive="0", max_runtime=None, model="Org/Model"):
        orig_env = dict(os.environ)
        self.addCleanup(os.environ.update, orig_env)
        self.addCleanup(os.environ.clear)
        os.environ.clear()
        os.environ.update(
            {
                "PATH": f"{self._make_stub(tmp, rc, block)}:{orig_env.get('PATH', '/usr/bin')}",
                "HERETIC_MODEL": model,
                "HERETIC_TRIALS": "1",
                "HERETIC_SAVE_DIR": str(Path(tmp) / "out"),
                "RUNHUH_KEEP_ALIVE_MIN": keep_alive,
                "RUNHUH_ERROR_DELAY_S": "0",
                "DASHBOARD_PORT": "0",
            }
        )
        if max_runtime is not None:
            os.environ["RUNHUH_MAX_RUNTIME_MIN"] = max_runtime

    def _patch(self, tmp, *, patch_sleep=False):
        state_dir = Path(tmp) / "runhug"
        state_dir.mkdir(parents=True, exist_ok=True)
        saved = (rp.STATE_DIR, rp.STATUS_PATH, rp.LOG_PATH, rp.os.makedirs, rp.time.sleep)
        self.addCleanup(lambda: setattr(rp, "STATE_DIR", saved[0]))
        self.addCleanup(lambda: setattr(rp, "STATUS_PATH", saved[1]))
        self.addCleanup(lambda: setattr(rp, "LOG_PATH", saved[2]))
        self.addCleanup(lambda: setattr(rp.os, "makedirs", saved[3]))
        self.addCleanup(lambda: setattr(rp.time, "sleep", saved[4]))
        rp.STATE_DIR = state_dir
        rp.STATUS_PATH = state_dir / "status.json"
        rp.LOG_PATH = state_dir / "training.log"
        rp.os.makedirs = lambda p, exist_ok=True: None
        if patch_sleep:
            rp.time.sleep = lambda *a, **kw: None
        return state_dir

    def test_success_returns_zero(self):
        with tempfile.TemporaryDirectory() as tmp:
            state_dir = self._patch(tmp)
            self._prepare_env(tmp, rc=0)
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = rp.run_main()
            self.assertEqual(code, 0)
            self.assertIn("[runhug] done", buf.getvalue())
            self.assertIn("Running trial 1 of 1", buf.getvalue())
            data = json.loads((state_dir / "status.json").read_text())
            self.assertEqual(data["status"], "done")
            self.assertTrue(data["done"])
            self.assertEqual(data["error"], "")
            self.assertEqual(data["best_refusals"], 55)

    def test_failure_reports_error_and_exits_nonzero(self):
        with tempfile.TemporaryDirectory() as tmp:
            state_dir = self._patch(tmp)
            self._prepare_env(tmp, rc=1)
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = rp.run_main()
            self.assertEqual(code, 1)
            self.assertIn("FAILED: heretic exited with status 1", buf.getvalue())
            data = json.loads((state_dir / "status.json").read_text())
            self.assertEqual(data["status"], "error")
            self.assertTrue(data["done"])
            self.assertEqual(data["error"], "heretic exited with status 1")

    def test_runtime_deadline_times_out(self):
        with tempfile.TemporaryDirectory() as tmp:
            state_dir = self._patch(tmp, patch_sleep=True)
            self._prepare_env(tmp, rc=0, block=True, max_runtime="0")
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = rp.run_main()
            self.assertEqual(code, 1)
            self.assertIn("exceeded RUNHUH_MAX_RUNTIME_MIN=0", buf.getvalue())
            data = json.loads((state_dir / "status.json").read_text())
            self.assertEqual(data["status"], "error")
            self.assertIn("RUNHUH_MAX_RUNTIME_MIN", data["error"])

    def test_deadline_upgrades_to_kill_when_terminate_times_out(self):
        class FakePopen:
            def __init__(self, cmd, **kwargs):
                self.stdout = io.StringIO("Running trial 1 of 1\n")
                self.returncode = None
                self._killed = False

            def poll(self):
                return None

            def terminate(self):
                pass

            def kill(self):
                self._killed = True
                self.returncode = -9

            def wait(self, timeout=None):
                if not self._killed:
                    raise subprocess.TimeoutExpired("heretic", timeout)
                return -9

        with tempfile.TemporaryDirectory() as tmp:
            state_dir = self._patch(tmp, patch_sleep=True)
            saved_popen = rp.subprocess.Popen
            rp.subprocess.Popen = FakePopen
            self.addCleanup(lambda: setattr(rp.subprocess, "Popen", saved_popen))
            orig_env = dict(os.environ)
            self.addCleanup(os.environ.clear)
            self.addCleanup(os.environ.update, orig_env)
            os.environ.clear()
            os.environ.update(
                {
                    "HERETIC_MODEL": "Org/M",
                    "HERETIC_TRIALS": "1",
                    "HERETIC_SAVE_DIR": str(Path(tmp) / "out"),
                    "RUNHUH_MAX_RUNTIME_MIN": "0",
                    "RUNHUH_KEEP_ALIVE_MIN": "0",
                    "RUNHUH_ERROR_DELAY_S": "0",
                    "DASHBOARD_PORT": "0",
                }
            )
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = rp.run_main()
            self.assertEqual(code, 1)
            self.assertIn("exceeded RUNHUH_MAX_RUNTIME_MIN=0", buf.getvalue())
            data = json.loads((state_dir / "status.json").read_text())
            self.assertEqual(data["status"], "error")

    def test_keep_alive_waits_before_returning_zero(self):
        with tempfile.TemporaryDirectory() as tmp:
            state_dir = self._patch(tmp, patch_sleep=True)
            self._prepare_env(tmp, rc=0, keep_alive="1")
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = rp.run_main()
            self.assertEqual(code, 0)
            data = json.loads((state_dir / "status.json").read_text())
            self.assertEqual(data["status"], "done")
            self.assertTrue(data["done"])

    def test_missing_model_exits_nonzero(self):
        with tempfile.TemporaryDirectory() as tmp:
            state_dir = self._patch(tmp)
            self._prepare_env(tmp, rc=0, model="")
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = rp.run_main()
            self.assertEqual(code, 1)
            self.assertIn("HERETIC_MODEL is required", buf.getvalue())
            data = json.loads((state_dir / "status.json").read_text())
            self.assertEqual(data["status"], "error")
            self.assertTrue(data["done"])

    def test_main_entrypoint_exits_with_code(self):
        with tempfile.TemporaryDirectory() as tmp:
            state_dir = Path(tmp) / "runhug"
            stub = Path(tmp) / "bin" / "heretic"
            stub.parent.mkdir()
            stub.write_text("#!/usr/bin/env python3\nimport sys\nprint('Running trial 1 of 1')\nsys.exit(0)\n")
            stub.chmod(0o755)
            env = dict(os.environ)
            env.update(
                {
                    "PATH": f"{stub.parent}:{env.get('PATH', '/usr/bin')}",
                    "HERETIC_MODEL": "Org/Model",
                    "HERETIC_TRIALS": "1",
                    "HERETIC_SAVE_DIR": str(Path(tmp) / "out"),
                    "RUNHUH_KEEP_ALIVE_MIN": "0",
                    "RUNHUH_ERROR_DELAY_S": "0",
                    "DASHBOARD_PORT": "0",
                }
            )
            env.pop("RUNHUH_TOKEN", None)
            prelude = (
                "from pathlib import Path\nimport run_pipeline as rp\n"
                f"rp.STATE_DIR = Path({str(state_dir)!r})\n"
                "rp.LOG_PATH = rp.STATE_DIR / 'training.log'\n"
                "rp.STATUS_PATH = rp.STATE_DIR / 'status.json'\n"
                "rp.LOG_PATH.parent.mkdir(parents=True, exist_ok=True)\n"
                "rp.os.makedirs = lambda p, exist_ok=True: None\n"
                "rp.main()\n"
            )
            proc = subprocess.run(
                [sys.executable, "-c", prelude],
                capture_output=True,
                text=True,
                cwd=str(Path(__file__).resolve().parents[1]),
                env=env,
                timeout=120,
            )
            self.assertEqual(proc.returncode, 0, proc.stderr[-1500:])
            self.assertIn("[runhug] done", proc.stdout)
            data = json.loads((state_dir / "status.json").read_text())
            self.assertEqual(data["status"], "done")


if __name__ == "__main__":
    unittest.main()